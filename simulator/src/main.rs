// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0
mod theme;
mod config;
mod cli;
mod ipc;

use base64::Engine as _;
use serde::{Deserialize, Serialize};
use soroban_env_host::xdr::ReadXdr;
use std::collections::HashMap;
use std::io::{self, Read};
use std::panic;
use ipc::validate::validate_request;
use ipc::types::SimulationRequest; // your generated types
use serde_json::from_value;



mod gas_optimizer;
use gas_optimizer::{BudgetMetrics, GasOptimizationAdvisor, OptimizationReport};

#[derive(Debug, Deserialize)]
struct SimulationRequest {
    envelope_xdr: String,
    result_meta_xdr: String,
    // Key XDR -> Entry XDR
    ledger_entries: Option<HashMap<String, String>>,
    profile: Option<bool>,
    #[serde(default)]
    enable_optimization_advisor: bool,
}

#[derive(Debug, Serialize)]
struct SimulationResponse {
    status: String,
    error: Option<String>,
    events: Vec<String>,
    logs: Vec<String>,
    flamegraph: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    optimization_report: Option<OptimizationReport>,
    #[serde(skip_serializing_if = "Option::is_none")]
    budget_usage: Option<BudgetUsage>,
}

#[derive(Debug, Serialize)]
struct BudgetUsage {
    cpu_instructions: u64,
    memory_bytes: u64,
    operations_count: usize,
}

#[derive(Debug, Serialize, Deserialize)]
struct StructuredError {
    error_type: String,
    message: String,
    details: Option<String>,
}

fn main() {
     cli::trace_viewer::render_trace();
    // Read JSON from Stdin
    let mut buffer = String::new();
    if let Err(e) = io::stdin().read_to_string(&mut buffer) {
        eprintln!("Failed to read stdin: {}", e);
        return;
    }

    // Parse Request
    let request: SimulationRequest = match serde_json::from_str(&buffer) {
        Ok(req) => req,
        Err(e) => {
            let res = SimulationResponse {
                status: "error".to_string(),
                error: Some(format!("Invalid JSON: {}", e)),
                events: vec![],
                logs: vec![],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
            };
            println!("{}", serde_json::to_string(&res).unwrap());
            return;
        }
    };

    // Decode Envelope XDR
    let envelope = match base64::engine::general_purpose::STANDARD.decode(&request.envelope_xdr) {
        Ok(bytes) => match soroban_env_host::xdr::TransactionEnvelope::from_xdr(
            bytes,
            soroban_env_host::xdr::Limits::none(),
        ) {
            Ok(env) => env,
            Err(e) => {
                return send_error(format!("Failed to parse Envelope XDR: {}", e));
            }
        },
        Err(e) => {
            return send_error(format!("Failed to decode Envelope Base64: {}", e));
        }
    };

    // Decode ResultMeta XDR
    let _result_meta = if request.result_meta_xdr.is_empty() {
        eprintln!("Warning: ResultMetaXdr is empty. Host storage will be empty.");
        None
    } else {
        match base64::engine::general_purpose::STANDARD.decode(&request.result_meta_xdr) {
            Ok(bytes) => match soroban_env_host::xdr::TransactionResultMeta::from_xdr(
                bytes,
                soroban_env_host::xdr::Limits::none(),
            ) {
                Ok(meta) => Some(meta),
                Err(e) => {
                    return send_error(format!("Failed to parse ResultMeta XDR: {}", e));
                }
            },
            Err(e) => {
                eprintln!("Warning: Failed to decode ResultMeta Base64: {}. Proceeding with empty storage.", e);
                None
            }
        }
    };

    // Initialize Host
    let host = soroban_env_host::Host::default();
    host.set_diagnostic_level(soroban_env_host::DiagnosticLevel::Debug)
        .unwrap();

    // Populate Host Storage
    if let Some(entries) = &request.ledger_entries {
        for (key_xdr, entry_xdr) in entries {
            // Decode Key
            let key = match base64::engine::general_purpose::STANDARD.decode(key_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerKey::from_xdr(
                    b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(k) => k,
                    Err(e) => return send_error(format!("Failed to parse LedgerKey XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerKey Base64: {}", e)),
            };

            // Decode Entry
            let entry = match base64::engine::general_purpose::STANDARD.decode(entry_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerEntry::from_xdr(
                    b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(e) => e,
                    Err(e) => return send_error(format!("Failed to parse LedgerEntry XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerEntry Base64: {}", e)),
            };

            // TODO: Inject into host storage.
            // For MVP, we verify we can parse them.
            eprintln!("Parsed Ledger Entry: Key={:?}, Entry={:?}", key, entry);
        }
    }

    let mut invocation_logs = vec![];

    // Extract Operations from Envelope
    let operations = match &envelope {
        soroban_env_host::xdr::TransactionEnvelope::Tx(tx_v1) => &tx_v1.tx.operations,
        soroban_env_host::xdr::TransactionEnvelope::TxV0(tx_v0) => &tx_v0.tx.operations,
        soroban_env_host::xdr::TransactionEnvelope::TxFeeBump(bump) => match &bump.tx.inner_tx {
            soroban_env_host::xdr::FeeBumpTransactionInnerTx::Tx(tx_v1) => &tx_v1.tx.operations,
        },
    };

    // Iterate and find InvokeHostFunction
    // Wrap the contract invocation in panic protection
    let invocation_result = panic::catch_unwind(panic::AssertUnwindSafe(|| {
        execute_operations(&host, operations)
    }));

    // Simulate budget usage (in production, this would come from host.budget())
    let simulated_budget = BudgetUsage {
        cpu_instructions: 45_000_000, // Example: 45M CPU instructions
        memory_bytes: 18_000_000,     // Example: 18M bytes
        operations_count: operations.len(),
    };

    // Generate optimization report if requested
    let optimization_report = if request.enable_optimization_advisor {
        let advisor = GasOptimizationAdvisor::new();
        let metrics = BudgetMetrics {
            cpu_instructions: simulated_budget.cpu_instructions,
            memory_bytes: simulated_budget.memory_bytes,
            total_operations: simulated_budget.operations_count,
        };
        Some(advisor.analyze(&metrics))
    } else {
        None
    };

    match invocation_result {
        Ok(Ok(execution_logs)) => {
            // Successful execution
            invocation_logs.extend(execution_logs);

            // Capture Diagnostic Events
            let events = match host.get_events() {
                Ok(evs) => evs
                    .0
                    .iter()
                    .map(|e| format!("{:?}", e))
                    .collect::<Vec<String>>(),
                Err(e) => vec![format!("Failed to retrieve events: {:?}", e)],
            };

            // Success Response
            let response = SimulationResponse {
                status: "success".to_string(),
                error: None,
                events,
                logs: invocation_logs,
                optimization_report,
                budget_usage: Some(simulated_budget),
            };

            println!("{}", serde_json::to_string(&response).unwrap());
        }
        Ok(Err(host_error)) => {
            // Host error during execution (e.g., contract trap, validation failure)
            let structured_error = StructuredError {
                error_type: "HostError".to_string(),
                message: format!("{:?}", host_error),
                details: Some(format!(
                    "Contract execution failed with host error: {:?}",
                    host_error
                )),
            };

            let response = SimulationResponse {
                status: "error".to_string(),
                error: Some(serde_json::to_string(&structured_error).unwrap()),
                events: vec![],
                logs: invocation_logs,
                optimization_report,
                budget_usage: Some(simulated_budget),
            };

            println!("{}", serde_json::to_string(&response).unwrap());
        }
        Err(panic_info) => {
            // Panic occurred during execution
            let panic_message = if let Some(s) = panic_info.downcast_ref::<&str>() {
                s.to_string()
            } else if let Some(s) = panic_info.downcast_ref::<String>() {
                s.clone()
            } else {
                "Unknown panic occurred".to_string()
            };

            let structured_error = StructuredError {
                error_type: "Panic".to_string(),
                message: panic_message.clone(),
                details: Some(format!(
                    "Contract execution panicked. This typically indicates a critical error in the contract or host. Panic message: {}",
                    panic_message
                )),
            };

            invocation_logs.push(format!("PANIC: {}", panic_message));

            let response = SimulationResponse {
                status: "error".to_string(),
                error: Some(serde_json::to_string(&structured_error).unwrap()),
                events: vec![],
                logs: invocation_logs,
                optimization_report: None,
                budget_usage: Some(simulated_budget),
            };

            println!("{}", serde_json::to_string(&response).unwrap());
        }
    }
}

/// Execute operations and handle host errors
fn execute_operations(
    _host: &soroban_env_host::Host,
    operations: &soroban_env_host::xdr::VecM<soroban_env_host::xdr::Operation, 100>,
) -> Result<Vec<String>, soroban_env_host::HostError> {
    let mut logs = vec![];

    for op in operations.iter() {
        if let soroban_env_host::xdr::OperationBody::InvokeHostFunction(host_fn_op) = &op.body {
            match &host_fn_op.host_function {
                soroban_env_host::xdr::HostFunction::InvokeContract(invoke_args) => {
                    logs.push("Found InvokeContract operation!".to_string());

                    let address = &invoke_args.contract_address;
                    let func_name = &invoke_args.function_name;
                    let invoke_args_vec = &invoke_args.args;

                    logs.push(format!("About to Invoke Contract: {:?}", address));
                    logs.push(format!("Function: {:?}", func_name));
                    logs.push(format!("Args Count: {}", invoke_args_vec.len()));

                    // In a full implementation, we'd do:
                    // let res = host.invoke_function(...)?;
                    // For now, this is a placeholder for actual contract invocation

                    // Example of how to handle HostError propagation:
                    // match host.invoke_function(...) {
                    //     Ok(result) => {
                    //         logs.push(format!("Invocation successful: {:?}", result));
                    //     }
                    //     Err(e) => {
                    //         // Propagate HostError up to be caught by the outer handler
                    //         return Err(e);
                    //     }
                    // }
                }
                _ => {
                    logs.push("Skipping non-InvokeContract Host Function".to_string());
                }
            }
        }
    }

    // Capture Diagnostic Events
    // Note: In soroban-env-host > v20, 'get_events' returns inputs to internal event system.
    // We want the literal events if possible, or formatted via 'events'.
    // The previous mocked response just had "Parsed Envelope".
    // Now we extract real events.

    // We need to clone them out or iterate. 'host.get_events()' returns a reflected vector.
    // Detailed event retrieval typically requires iterating host storage or using the events buffer.
    // For MVP, we will try `host.events().0` if accessible or just `host.get_events()`.
    // Actually `host.get_events()` returns `Result<Vec<HostEvent>, ...>`.

    let events = match host.get_events() {
        Ok(evs) => evs
            .0
            .iter()
            .map(|e| format!("{:?}", e))
            .collect::<Vec<String>>(),
        Err(e) => vec![format!("Failed to retrieve events: {:?}", e)],
    };

    let mut flamegraph_svg = None;

    if request.profile.unwrap_or(false) {
        let budget = host.budget_cloned();
        
        // Capture detailed cost metrics
        // In a real implementation, we would hook into the host's budget tracker
        // to record costs as they happen. For this demonstration, we'll
        // simulate the hierarchical data based on the final budget state.
        
        let cpu_insns = budget.get_cpu_insns_consumed().unwrap_or(0);
        let mem_bytes = budget.get_mem_bytes_consumed().unwrap_or(0);

        let mut folded_data = String::new();
        
        // Simulate hierarchical data: Total > [Contract] > [CostType]
        // We'll use the invocation logs to "guess" which contracts were called
        for log in &invocation_logs {
            if log.contains("About to Invoke Contract:") {
                let contract_name = log.split(": ").last().unwrap_or("UnknownContract");
                
                // Distribute a portion of the costs to this contract
                // (This is a simplified model for the demonstration)
                let contract_cpu = cpu_insns / (invocation_logs.len() as u64).max(1);
                let contract_mem = mem_bytes / (invocation_logs.len() as u64).max(1);
                
                folded_data.push_str(&format!("Total;{};CPU {}\n", contract_name, contract_cpu));
                folded_data.push_str(&format!("Total;{};Memory {}\n", contract_name, contract_mem));
            }
        }

        // If no contracts were found, just show totals
        if folded_data.is_empty() {
            folded_data.push_str(&format!("Total;CPU {}\n", cpu_insns));
            folded_data.push_str(&format!("Total;Memory {}\n", mem_bytes));
        }
        
        let mut result = Vec::new();
        let mut options = inferno::flamegraph::Options::default();
        options.title = "Soroban Resource Consumption".to_string();
        options.count_name = "units".to_string();

        if let Err(e) = inferno::flamegraph::from_reader(&mut options, folded_data.as_bytes(), &mut result) {
            eprintln!("Failed to generate flamegraph: {}", e);
        } else {
            flamegraph_svg = Some(String::from_utf8_lossy(&result).to_string());
        }
    }

    // Mock Success Response
    let response = SimulationResponse {
        status: "success".to_string(),
        error: None,
        events,
        logs: {
            let mut logs = vec![
                format!("Host Initialized with Budget: {:?}", host.budget_cloned()),
                format!("Loaded {} Ledger Entries", loaded_entries_count),
            ];
            logs.extend(invocation_logs);
            logs
        },
        flamegraph: flamegraph_svg,
    };

    println!("{}", serde_json::to_string(&response).unwrap());
    Ok(logs)
}

fn send_error(msg: String) {
    let res = SimulationResponse {
        status: "error".to_string(),
        error: Some(msg),
        events: vec![],
        logs: vec![],
        flamegraph: None,
    };
    println!("{}", serde_json::to_string(&res).unwrap());
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_simulation_request_deserialization() {
        let json = r#"{"envelope_xdr": "AAAA", "result_meta_xdr": "BBBB", "profile": true}"#;
        let req: SimulationRequest = serde_json::from_str(json).unwrap();
        assert_eq!(req.profile, Some(true));
    }

    #[test]
    fn test_simulation_response_serialization() {
        let res = SimulationResponse {
            status: "success".to_string(),
            error: None,
            events: vec![],
            logs: vec![],
            flamegraph: Some("<svg></svg>".to_string()),
        };
        let json = serde_json::to_string(&res).unwrap();
        assert!(json.contains("\"flamegraph\":\"<svg></svg>\""));
    }
}
        optimization_report: None,
        budget_usage: None,
    };
    println!("{}", serde_json::to_string(&res).unwrap());
}
fn handle_input(json_input: &str) -> Result<SimulationRequest, String> {
    // Step 1: validate JSON against schema
    let validated_value = validate_request(json_input)?;

    // Step 2: deserialize into Rust type
    let req: SimulationRequest = from_value(validated_value).map_err(|e| e.to_string())?;

    // Step 3: optional version check
    if req.version != "1.0" {
        return Err(format!("Unsupported IPC schema version: {}", req.version));
    }

    Ok(req)
}

mod test;
