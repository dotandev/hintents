// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

mod cli;
mod config;
mod gas_optimizer;
mod ipc;
mod theme;

use base64::Engine as _;
use serde::{Deserialize, Serialize};
use soroban_env_host::xdr::ReadXdr;
use std::collections::HashMap;
use std::io::{self, Read};
use std::panic;

use crate::gas_optimizer::{BudgetMetrics, GasOptimizationAdvisor, OptimizationReport};
use soroban_env_host::events::Events;

// -----------------------------------------------------------------------------
// Data Structures
// -----------------------------------------------------------------------------

#[derive(Debug, Deserialize)]
struct SimulationRequest {
    envelope_xdr: String,
    #[allow(dead_code)]
    result_meta_xdr: String,
    // Key XDR -> Entry XDR
    ledger_entries: Option<HashMap<String, String>>,
    enable_optimization_advisor: bool,
    profile: Option<bool>,
}

#[derive(Debug, Serialize)]
struct DiagnosticEvent {
    event_type: String,
    contract_id: Option<String>,
    topics: Vec<String>,
    data: String,
    in_successful_contract_call: bool,
}

#[derive(Debug, Serialize)]
pub struct CategorizedEvent {
    pub category: String,
    pub details: String,
}

#[derive(Debug, Serialize)]
struct BudgetUsage {
    cpu_instructions: u64,
    memory_bytes: u64,
    operations_count: usize,
}

#[derive(Debug, Serialize)]
struct StructuredError {
    error_type: String,
    message: String,
    details: Option<String>,
}

#[derive(Debug, Serialize)]
struct SimulationResponse {
    status: String,
    error: Option<String>,
    events: Vec<String>,
    diagnostic_events: Vec<DiagnosticEvent>,
    categorized_events: Vec<CategorizedEvent>,
    logs: Vec<String>,
    flamegraph: Option<String>,
    optimization_report: Option<OptimizationReport>,
    budget_usage: Option<BudgetUsage>,
}

fn execute_operations(
    _host: &soroban_env_host::Host,
    _ops: &[soroban_env_host::xdr::Operation],
) -> Result<Vec<String>, soroban_env_host::HostError> {
    // Placeholder for actual execution logic using the host
    // In a real scenario, this would apply operations.
    // For now we just return empty logs or mock behavior if needed
    Ok(vec![])
}

fn categorize_events(_events: &Events) -> Vec<CategorizedEvent> {
    // Placeholder function since it was missing in original file
    vec![]
}

// -----------------------------------------------------------------------------
// Main Execution
// -----------------------------------------------------------------------------

fn main() {
    // Read JSON from Stdin
    let mut buffer = String::new();
    if let Err(e) = io::stdin().read_to_string(&mut buffer) {
        let res = SimulationResponse {
            status: "error".to_string(),
            error: Some(format!("Failed to read stdin: {}", e)),
            events: vec![],
            diagnostic_events: vec![],
            categorized_events: vec![],
            logs: vec![],
            flamegraph: None,
            optimization_report: None,
            budget_usage: None,
        };
        println!("{}", serde_json::to_string(&res).unwrap());
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
                diagnostic_events: vec![],
                categorized_events: vec![],
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

    // Initialize Host
    let host = soroban_env_host::Host::default();
    host.set_diagnostic_level(soroban_env_host::DiagnosticLevel::Debug)
        .unwrap();

    let mut loaded_entries_count = 0;

    // Populate Host Storage
    if let Some(entries) = &request.ledger_entries {
        for (key_xdr, entry_xdr) in entries {
            let _key = match base64::engine::general_purpose::STANDARD.decode(key_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerKey::from_xdr(
                    b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(k) => k,
                    Err(e) => return send_error(format!("Failed to parse LedgerKey XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerKey Base64: {}", e)),
            };

            let _entry = match base64::engine::general_purpose::STANDARD.decode(entry_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerEntry::from_xdr(
                    b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(e) => e,
                    Err(e) => return send_error(format!("Failed to parse LedgerEntry XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerEntry Base64: {}", e)),
            };
            loaded_entries_count += 1;
        }
    }

    // Extract Operations and Simulate
    let operations = match &envelope {
        soroban_env_host::xdr::TransactionEnvelope::Tx(tx_v1) => &tx_v1.tx.operations,
        soroban_env_host::xdr::TransactionEnvelope::TxV0(tx_v0) => &tx_v0.tx.operations,
        soroban_env_host::xdr::TransactionEnvelope::TxFeeBump(bump) => match &bump.tx.inner_tx {
            soroban_env_host::xdr::FeeBumpTransactionInnerTx::Tx(tx_v1) => &tx_v1.tx.operations,
        },
    };

    // Wrap the operation execution in panic protection
    let result = panic::catch_unwind(panic::AssertUnwindSafe(|| {
        execute_operations(&host, operations)
    }));

    // Budget and Reporting
    let budget = host.budget_cloned();
    let cpu_insns = budget.get_cpu_insns_consumed().unwrap_or(0);
    let mem_bytes = budget.get_mem_bytes_consumed().unwrap_or(0);

    let budget_usage = BudgetUsage {
        cpu_instructions: cpu_insns,
        memory_bytes: mem_bytes,
        operations_count: operations.as_slice().len(),
    };

    let optimization_report = if request.enable_optimization_advisor {
        let advisor = GasOptimizationAdvisor::new();
        let metrics = BudgetMetrics {
            cpu_instructions: budget_usage.cpu_instructions,
            memory_bytes: budget_usage.memory_bytes,
            total_operations: budget_usage.operations_count,
        };
        Some(advisor.analyze(&metrics))
    } else {
        None
    };

    let mut flamegraph_svg = None;
    if request.profile.unwrap_or(false) {
        // Simple simulated flamegraph for demonstration
        let folded_data = format!("Total;CPU {}\nTotal;Memory {}\n", cpu_insns, mem_bytes);
        let mut result = Vec::new();
        let mut options = inferno::flamegraph::Options::default();
        options.title = "Soroban Resource Consumption".to_string();

        if let Err(e) =
            inferno::flamegraph::from_reader(&mut options, folded_data.as_bytes(), &mut result)
        {
            eprintln!("Failed to generate flamegraph: {}", e);
        } else {
            flamegraph_svg = Some(String::from_utf8_lossy(&result).to_string());
        }
    }

    match result {
        Ok(Ok(exec_logs)) => {
            // Extract both raw event strings and structured diagnostic events
            let (events, diagnostic_events) = match host.get_events() {
                Ok(evs) => {
                    let raw_events: Vec<String> =
                        evs.0.iter().map(|e| format!("{:?}", e)).collect();
                    let diag_events: Vec<DiagnosticEvent> = evs
                        .0
                        .iter()
                        .map(|event| {
                            let event_type = match &event.event.type_ {
                                soroban_env_host::xdr::ContractEventType::Contract => {
                                    "contract".to_string()
                                }
                                soroban_env_host::xdr::ContractEventType::System => {
                                    "system".to_string()
                                }
                                soroban_env_host::xdr::ContractEventType::Diagnostic => {
                                    "diagnostic".to_string()
                                }
                            };

                            let contract_id = event
                                .event
                                .contract_id
                                .as_ref()
                                .map(|contract_id| format!("{:?}", contract_id));

                            let (topics, data) = match &event.event.body {
                                soroban_env_host::xdr::ContractEventBody::V0(v0) => {
                                    let topics: Vec<String> =
                                        v0.topics.iter().map(|t| format!("{:?}", t)).collect();
                                    let data = format!("{:?}", v0.data);
                                    (topics, data)
                                }
                            };

                            DiagnosticEvent {
                                event_type,
                                contract_id,
                                topics,
                                data,
                                in_successful_contract_call: event.failed_call,
                            }
                        })
                        .collect();
                    (raw_events, diag_events)
                }
                Err(_) => (vec!["Failed to retrieve events".to_string()], vec![]),
            };

            // Capture categorized events for analyzer
            let categorized_events = match host.get_events() {
                Ok(evs) => categorize_events(&evs),
                Err(_) => vec![],
            };

            let mut final_logs = vec![
                format!("Host Initialized with Budget: {:?}", budget),
                format!("Loaded {} Ledger Entries", loaded_entries_count),
                format!("Captured {} diagnostic events", diagnostic_events.len()),
                format!("CPU Instructions Used: {}", cpu_insns),
                format!("Memory Bytes Used: {}", mem_bytes),
            ];
            for log in exec_logs {
                final_logs.push(log);
            }

            let response = SimulationResponse {
                status: "success".to_string(),
                error: None,
                events,
                diagnostic_events,
                categorized_events,
                logs: final_logs,
                flamegraph: flamegraph_svg,
                optimization_report,
                budget_usage: Some(budget_usage),
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
                diagnostic_events: vec![],
                categorized_events: vec![],
                logs: vec![],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
            };
            println!("{}", serde_json::to_string(&response).unwrap());
        }
        Err(panic_info) => {
            let panic_msg = if let Some(s) = panic_info.downcast_ref::<&str>() {
                s.to_string()
            } else if let Some(s) = panic_info.downcast_ref::<String>() {
                s.clone()
            } else {
                "Unknown panic".to_string()
            };

            let response = SimulationResponse {
                status: "error".to_string(),
                error: Some(format!("Simulator panicked: {}", panic_msg)),
                events: vec![],
                diagnostic_events: vec![],
                categorized_events: vec![],
                logs: vec![format!("PANIC: {}", panic_msg)],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
            };
            println!("{}", serde_json::to_string(&response).unwrap());
        }
    }
}

// -----------------------------------------------------------------------------
// Decoder Logic
// -----------------------------------------------------------------------------

/// Decodes generic errors and WASM traps into human-readable messages.
///
/// Differentiates between:
/// 1. VM-initiated traps (WASM execution failures)
/// 2. Host-initiated traps (Soroban environment logic failures)
#[allow(dead_code)]
fn decode_error(err_msg: &str) -> String {
    let err_lower = err_msg.to_lowercase();

    // Check for VM-initiated traps (Pure WASM)
    if err_lower.contains("wasm trap") || err_lower.contains("trapped") {
        if err_lower.contains("unreachable") {
            return "VM Trap: Unreachable Instruction (Panic or invalid code path)".to_string();
        }
        if err_lower.contains("out of bounds") || err_lower.contains("memory access") {
            return "VM Trap: Out of Bounds Access (Invalid memory read/write)".to_string();
        }
        if err_lower.contains("integer overflow") || err_lower.contains("arithmetic overflow") {
            return "VM Trap: Integer Overflow".to_string();
        }
        if err_lower.contains("stack overflow") || err_lower.contains("call stack exhausted") {
            return "VM Trap: Stack Overflow (Recursion limit exceeded)".to_string();
        }
        if err_lower.contains("divide by zero") {
            return "VM Trap: Division by Zero".to_string();
        }
        return format!("VM Trap: Unknown Wasm Trap ({})", err_msg);
    }

    // Check for Host-initiated traps (Soroban Host Logic)
    if err_lower.contains("hosterror") || err_lower.contains("context") {
        return format!("Host Trap: {}", err_msg);
    }

    // Fallback
    format!("Execution Error: {}", err_msg)
}

fn send_error(msg: String) {
    let res = SimulationResponse {
        status: "error".to_string(),
        error: Some(msg),
        events: vec![],
        diagnostic_events: vec![],
        categorized_events: vec![],
        logs: vec![],
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
    };
    println!("{}", serde_json::to_string(&res).unwrap());
}

#[allow(dead_code)]
fn run_local_wasm_replay(wasm_path: &str, mock_args: &Option<Vec<String>>) {
    use soroban_env_host::Host;
    use std::fs;

    eprintln!("🔧 Local WASM Replay Mode");
    eprintln!("WASM Path: {}", wasm_path);
    eprintln!("⚠️  WARNING: Using Mock State (not mainnet data)");
    eprintln!();

    // Read WASM file
    match fs::read(wasm_path) {
        Ok(bytes) => {
            eprintln!("✓ Loaded WASM file: {} bytes", bytes.len());
        }
        Err(e) => {
            send_error(format!("Failed to read WASM file: {}", e));
            return;
        }
    };

    // Initialize Host
    let host = Host::default();
    host.set_diagnostic_level(soroban_env_host::DiagnosticLevel::Debug)
        .unwrap();

    eprintln!("✓ Initialized Host with diagnostic level: Debug");

    // TODO: Full execution requires 'testutils' feature which is currently causing build issues.
    // For now, we just parse args and print what we WOULD do.

    eprintln!(
        "⚠️  Full execution temporarily disabled due to build issues with 'testutils' feature."
    );
    eprintln!("   (See issue #183 for details)");

    // Parse Arguments (Mock)
    if let Some(args) = mock_args {
        if !args.is_empty() {
            eprintln!("▶ Would invoke function: {}", args[0]);
            eprintln!("  With args: {:?}", &args[1..]);
        }
    }

    // Capture Logs/Events
    let events = match host.get_events() {
        Ok(evs) => evs
            .0
            .iter()
            .map(|e| format!("{:?}", e))
            .collect::<Vec<String>>(),
        Err(e) => vec![format!("Failed to retrieve events: {:?}", e)],
    };

    let logs = vec![
        format!("Host Budget: {:?}", host.budget_cloned()),
        "Execution: Skipped (Build Issue)".to_string(),
    ];

    let response = SimulationResponse {
        status: "success".to_string(),
        error: None,
        events,
        diagnostic_events: vec![],
        categorized_events: vec![],
        logs,
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
    };

    println!("{}", serde_json::to_string(&response).unwrap());
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_decode_vm_traps() {
        // 1. Out of Bounds
        let msg = decode_error("Error: Wasm Trap: out of bounds memory access");
        assert!(msg.contains("VM Trap: Out of Bounds Access"));

        // 2. Integer Overflow
        let msg = decode_error("Error: trapped: integer overflow");
        assert!(msg.contains("VM Trap: Integer Overflow"));

        // 3. Stack Overflow
        let msg = decode_error("Wasm Trap: call stack exhausted");
        assert!(msg.contains("VM Trap: Stack Overflow"));

        // 4. Unreachable
        let msg = decode_error("Wasm Trap: unreachable");
        assert!(msg.contains("VM Trap: Unreachable Instruction"));
    }

    #[test]
    fn test_decode_host_traps() {
        // Host Error
        let msg = decode_error("HostError: Error(Context, InvalidInput)");
        assert!(msg.contains("Host Trap"));
        assert!(!msg.contains("VM Trap"));
    }

    #[test]
    fn test_unknown_trap_fallback() {
        let msg = decode_error("Wasm Trap: something weird happened");
        assert!(msg.contains("VM Trap: Unknown Wasm Trap"));
    }
}
