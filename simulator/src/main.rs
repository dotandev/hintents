// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

mod theme;
mod config;
mod cli;
mod ipc;
mod gas_optimizer;

use base64::Engine as _;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use soroban_env_host::xdr::ReadXdr;
use std::collections::HashMap;
use std::io::{self, Read};
use std::panic;

use gas_optimizer::{BudgetMetrics, GasOptimizationAdvisor, OptimizationReport};

#[derive(Debug, Deserialize)]
struct SimulationRequest {
    #[serde(default)]
    network: Option<String>,
    envelope_xdr: String,
    result_meta_xdr: String,
    // Key XDR -> Entry XDR
    ledger_entries: Option<HashMap<String, String>>,
    timestamp: Option<i64>,
    ledger_sequence: Option<u32>,
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

fn network_passphrase(network: &str) -> Option<&'static str> {
    match network.to_lowercase().as_str() {
        "public" | "mainnet" => Some("Public Global Stellar Network ; September 2015"),
        "testnet" => Some("Test SDF Network ; September 2015"),
        "futurenet" => Some("Test SDF Future Network ; October 2022"),
        _ => None,
    }
}

fn network_id_from_passphrase(passphrase: &str) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(passphrase.as_bytes());
    let digest = hasher.finalize();
    let mut out = [0u8; 32];
    out.copy_from_slice(&digest[..]);
    out
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

    // Set network passphrase / network_id if provided
    if let Some(network) = &request.network {
        if let Some(passphrase) = network_passphrase(network) {
            let network_id = network_id_from_passphrase(passphrase);
            host.with_mut_ledger_info(|ledger_info| {
                ledger_info.network_id = network_id;
            })
            .unwrap();
        }
    }

    // Override Ledger Info if provided
    if request.timestamp.is_some() || request.ledger_sequence.is_some() {
        host.with_mut_ledger_info(|ledger_info| {
            if let Some(ts) = request.timestamp {
                ledger_info.timestamp = ts as u64;
            }
            if let Some(seq) = request.ledger_sequence {
                ledger_info.sequence_number = seq;
            }
        })
        .unwrap();
    }
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_time_travel_deserialization() {
        let json = r#"{"envelope_xdr": "AAAA", "result_meta_xdr": "BBBB", "timestamp": 1738077842, "ledger_sequence": 1234}"#;
        let req: SimulationRequest = serde_json::from_str(json).unwrap();
        assert_eq!(req.timestamp, Some(1738077842));
        assert_eq!(req.ledger_sequence, Some(1234));
    }
}

fn run_local_wasm_replay(wasm_path: &str, mock_args: &Option<Vec<String>>) {
    use std::fs;
    use soroban_env_host::{
        xdr::{ScVal, ScSymbol, ScAddress},
        Host,
    };

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

    // Initialize Host
    let host = Host::default();
    host.set_diagnostic_level(soroban_env_host::DiagnosticLevel::Debug).unwrap();
    
    eprintln!("✓ Initialized Host with diagnostic level: Debug");

    // TODO: Full execution requires 'testutils' feature which is currently causing build issues.
    // For now, we just parse args and print what we WOULD do.
    
    eprintln!("⚠️  Full execution temporarily disabled due to build issues with 'testutils' feature.");
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
        Ok(evs) => evs.0.iter().map(|e| format!("{:?}", e)).collect::<Vec<String>>(),
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
        logs,
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
    };

    println!("{}", serde_json::to_string(&response).unwrap());
}
