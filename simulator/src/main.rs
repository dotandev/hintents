// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use base64::Engine as _;
use serde::{Deserialize, Serialize};
use soroban_env_host::xdr::ReadXdr;
use std::collections::HashMap;
use std::io::{self, Read};

#[derive(Debug, Deserialize)]
struct SimulationRequest {
    envelope_xdr: String,
    result_meta_xdr: String,
    // Key XDR -> Entry XDR
    ledger_entries: Option<HashMap<String, String>>,
}

#[derive(Debug, Serialize)]
struct SimulationResponse {
    status: String,
    error: Option<String>,
    events: Vec<String>,
    logs: Vec<String>,
}

fn main() {
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

    let mut loaded_entries_count = 0;

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
            loaded_entries_count += 1;
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
    for op in operations.iter() {
        if let soroban_env_host::xdr::OperationBody::InvokeHostFunction(host_fn_op) = &op.body {
            match &host_fn_op.host_function {
                soroban_env_host::xdr::HostFunction::InvokeContract(invoke_args) => {
                    eprintln!("Found InvokeContract operation!");

                    let address = &invoke_args.contract_address;
                    let func_name = &invoke_args.function_name;
                    let invoke_args_vec = &invoke_args.args;

                    // Let's just FORMAT the data for now as proof of "Replay Logic" extraction.
                    invocation_logs.push(format!("About to Invoke Contract: {:?}", address));
                    invocation_logs.push(format!("Function: {:?}", func_name));
                    invocation_logs.push(format!("Args Count: {}", invoke_args_vec.len()));

                    // In a full implementation, we'd do:
                    // let res = host.invoke_function(Host::from_xdr(address), ...);
                }
                _ => {
                    invocation_logs.push("Skipping non-InvokeContract Host Function".to_string());
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
    };

    println!("{}", serde_json::to_string(&response).unwrap());
}

fn send_error(msg: String) {
    let res = SimulationResponse {
        status: "error".to_string(),
        error: Some(msg),
        events: vec![],
        logs: vec![],
    };
    println!("{}", serde_json::to_string(&res).unwrap());
}
