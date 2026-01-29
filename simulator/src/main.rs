// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

mod theme;
mod config;
mod cli;
mod ipc;
mod gas_optimizer;

use base64::Engine as _;
use serde::{Deserialize, Serialize};
use soroban_env_host::xdr::ReadXdr;
use std::collections::HashMap;
use std::io::{self, Read};
use std::panic;

use gas_optimizer::{BudgetMetrics, GasOptimizationAdvisor, OptimizationReport};

#[derive(Debug, Deserialize)]
struct SimulationRequest {
    envelope_xdr: String,
    result_meta_xdr: String,
    // Key XDR -> Entry XDR
    ledger_entries: Option<HashMap<String, String>>,
    // Optional: Path to local WASM file for local replay
    wasm_path: Option<String>,
    // Optional: Mock arguments for local replay (JSON array of strings)
    mock_args: Option<Vec<String>>,
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
    // Read JSON from Stdin
    let mut buffer = String::new();
    if let Err(e) = io::stdin().read_to_string(&mut buffer) {
        let res = SimulationResponse {
            status: "error".to_string(),
            error: Some(format!("Failed to read stdin: {}", e)),
            events: vec![],
            logs: vec![],
            flamegraph: None,
            optimization_report: None,
            budget_usage: None,
        };
        println!("{}", serde_json::to_string(&res).unwrap());
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

    // Check if this is a local WASM replay (no network data)
    if let Some(wasm_path) = &request.wasm_path {
        return run_local_wasm_replay(wasm_path, &request.mock_args);
    }

    // Decode Envelope XDR
    let envelope = match base64::engine::general_purpose::STANDARD.decode(&request.envelope_xdr) {
        Ok(bytes) => match soroban_env_host::xdr::TransactionEnvelope::from_xdr(
            &bytes,
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

    // Populate Host Storage
    let mut loaded_entries_count = 0;
    if let Some(entries) = &request.ledger_entries {
        for (key_xdr, entry_xdr) in entries {
            // Decode Key
            let _key = match base64::engine::general_purpose::STANDARD.decode(key_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerKey::from_xdr(
                    &b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(k) => k,
                    Err(e) => return send_error(format!("Failed to parse LedgerKey XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerKey Base64: {}", e)),
            };

            // Decode Entry
            let _entry = match base64::engine::general_purpose::STANDARD.decode(entry_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerEntry::from_xdr(
                    &b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(e) => e,
                    Err(e) => return send_error(format!("Failed to parse LedgerEntry XDR: {}", e)),
                },
                Err(e) => return send_error(format!("Failed to decode LedgerEntry Base64: {}", e)),
            };

            // In real implementation, we'd inject into host storage here.
            loaded_entries_count += 1;
        }
    }

    // Extract Operations from Envelope
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
        
        if let Err(e) = inferno::flamegraph::from_reader(&mut options, folded_data.as_bytes(), &mut result) {
            eprintln!("Failed to generate flamegraph: {}", e);
        } else {
            flamegraph_svg = Some(String::from_utf8_lossy(&result).to_string());
        }
    }

    match result {
        Ok(exec_logs) => {
            let events = match host.get_events() {
                Ok(evs) => evs.0.iter().map(|e| format!("{:?}", e)).collect(),
                Err(_) => vec!["Failed to retrieve events".to_string()],
            };

            let mut final_logs = vec![
                format!("Host Initialized with Budget: {:?}", budget),
                format!("Loaded {} Ledger Entries", loaded_entries_count),
            ];
            final_logs.extend(exec_logs);

            let response = SimulationResponse {
                status: "success".to_string(),
                error: None,
                events,
                logs: final_logs,
                flamegraph: flamegraph_svg,
                optimization_report,
                budget_usage: Some(budget_usage),
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
                logs: vec![format!("PANIC: {}", panic_msg)],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
            };
            println!("{}", serde_json::to_string(&response).unwrap());
        }
    }
}

fn execute_operations(
    _host: &soroban_env_host::Host,
    operations: &soroban_env_host::xdr::VecM<soroban_env_host::xdr::Operation, 100>,
) -> Vec<String> {
    let mut logs = vec![];
    for (i, op) in operations.as_slice().iter().enumerate() {
        logs.push(format!("Processing operation {}: {:?}", i, op.body));
        // Placeholder for real host invocation
    }
    logs
}

fn send_error(msg: String) {
    let res = SimulationResponse {
        status: "error".to_string(),
        error: Some(msg),
        events: vec![],
        logs: vec![],
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
    };
    println!("{}", serde_json::to_string(&res).unwrap());
}

fn run_local_wasm_replay(wasm_path: &str, mock_args: &Option<Vec<String>>) {
    use std::fs;
    
    eprintln!("🔧 Local WASM Replay Mode");
    eprintln!("WASM Path: {}", wasm_path);
    eprintln!("⚠️  WARNING: Using Mock State (not mainnet data)");
    eprintln!();

    // Read WASM file
    let wasm_bytes = match fs::read(wasm_path) {
        Ok(bytes) => {
            eprintln!("✓ Loaded WASM file: {} bytes", bytes.len());
            bytes
        },
        Err(e) => {
            return send_error(format!("Failed to read WASM file: {}", e));
        }
    };

    // Initialize Host with mock state
    let host = soroban_env_host::Host::default();
    host.set_diagnostic_level(soroban_env_host::DiagnosticLevel::Debug).unwrap();
    
    eprintln!("✓ Initialized Host with diagnostic level: Debug");

    // TODO: In a full implementation, we would:
    // 1. Parse the WASM module to extract contract metadata
    // 2. Deploy the contract to the host
    // 3. Parse mock_args into proper ScVal types
    // 4. Invoke the contract function with the arguments
    // 5. Capture and return the result
    
    // For now, we'll create a basic response showing the setup worked
    let mut logs = vec![
        format!("Host Initialized with Budget: {:?}", host.budget_cloned()),
        format!("WASM file loaded: {} bytes", wasm_bytes.len()),
        "Mock State Provider: Active".to_string(),
    ];

    if let Some(args) = mock_args {
        logs.push(format!("Mock Arguments provided: {} args", args.len()));
        for (i, arg) in args.iter().enumerate() {
            logs.push(format!("  Arg[{}]: {}", i, arg));
        }
    } else {
        logs.push("No mock arguments provided".to_string());
    }

    logs.push("".to_string());
    logs.push("⚠️  Note: Full WASM execution not yet implemented".to_string());
    logs.push("This is a mock response showing the local replay infrastructure is working.".to_string());

    // Capture diagnostic events
    let events = match host.get_events() {
        Ok(evs) => evs.0.iter().map(|e| format!("{:?}", e)).collect::<Vec<String>>(),
        Err(e) => vec![format!("Failed to retrieve events: {:?}", e)],
    };

    let response = SimulationResponse {
        status: "success".to_string(),
        error: None,
        events,
        logs,
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
    };

    println!("{}", serde_json::to_string(&response).unwrap());
}