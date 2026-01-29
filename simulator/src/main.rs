// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use base64::Engine as _;
use serde::{Deserialize, Serialize};
use serde_json::json;
use soroban_env_host::events::Events;
use soroban_env_host::xdr::ReadXdr;
use std::collections::HashMap;
use std::io::{self, Read};
use std::panic;

#[derive(Debug, Deserialize)]
struct SimulationRequest {
    envelope_xdr: String,
    result_meta_xdr: String,
    ledger_entries: Option<HashMap<String, String>>,
}

#[derive(Debug, Serialize, Clone)]
struct CategorizedEvent {
    event_type: String,
    contract_id: Option<String>,
    topics: Vec<String>,
    data: String,
}

#[derive(Debug, Serialize)]
struct SimulationResponse {
    status: String,
    error: Option<String>,
    events: Vec<String>,
    categorized_events: Vec<CategorizedEvent>,
    logs: Vec<String>,
}

fn categorize_event_for_analyzer(
    event: &soroban_env_host::events::HostEvent,
) -> Result<String, String> {
    use soroban_env_host::xdr::{ContractEventBody, ContractEventType, ScVal};

    let contract_id = match &event.event.contract_id {
        Some(id) => format!("{:?}", id),
        None => "unknown".to_string(),
    };

    let event_type_str = match &event.event.type_ {
        ContractEventType::Contract => "contract",
        ContractEventType::System => "system",
        ContractEventType::Diagnostic => "diagnostic",
    };

    let (topics, _data_val) = match &event.event.body {
        ContractEventBody::V0(v0) => (&v0.topics, &v0.data),
    };

    let event_json = if let Some(first_topic) = topics.get(0) {
        let topic_str = format!("{:?}", first_topic);

        if topic_str.contains("require_auth") {
            let address = if let ScVal::Address(addr) = first_topic {
                format!("{:?}", addr)
            } else {
                "unknown".to_string()
            };

            json!({
                "type": "auth",
                "contract": contract_id,
                "address": address,
                "event_type": event_type_str,
            })
            .to_string()
        } else if topic_str.contains("set")
            || topic_str.contains("write")
            || topic_str.contains("storage")
        {
            json!({
                "type": "storage_write",
                "contract": contract_id,
                "event_type": event_type_str,
            })
            .to_string()
        } else if topic_str.contains("call") || topic_str.contains("invoke") {
            if let ScVal::Symbol(sym) = first_topic {
                json!({
                    "type": "contract_call",
                    "contract": contract_id,
                    "function": sym.to_string(),
                    "event_type": event_type_str,
                })
                .to_string()
            } else {
                json!({
                    "type": "contract_call",
                    "contract": contract_id,
                    "event_type": event_type_str,
                })
                .to_string()
            }
        } else {
            json!({
                "type": "other",
                "contract": contract_id,
                "event_type": event_type_str,
            })
            .to_string()
        }
    } else {
        json!({
            "type": "other",
            "contract": contract_id,
            "event_type": event_type_str,
        })
        .to_string()
    };

    Ok(event_json)
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
                categorized_events: vec![],
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

            // Capture categorized events for analyzer
            let categorized_events = match host.get_events() {
                Ok(evs) => categorize_events(&evs),
                Err(_) => vec![],
            };

            // Success Response
            let response = SimulationResponse {
                status: "success".to_string(),
                error: None,
                events,
                categorized_events,
                logs: invocation_logs,
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
                categorized_events: vec![],
                logs: invocation_logs,
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
                categorized_events: vec![],
                logs: invocation_logs,
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
                }
                _ => {
                    logs.push("Skipping non-InvokeContract Host Function".to_string());
                }
            }
        }
    }

    Ok(logs)
}

fn categorize_events(events: &Events) -> Vec<CategorizedEvent> {
    use soroban_env_host::xdr::{ContractEventBody, ContractEventType, ScVal};

    events
        .0
        .iter()
        .filter_map(|event| {
            // Access body to get topics and data
            let (topics, data_val) = match &event.event.body {
                ContractEventBody::V0(v0) => (&v0.topics, &v0.data),
            };

            if !event.failed_call {
                let event_type = match &event.event.type_ {
                    ContractEventType::Contract => {
                        if let Some(topic) = topics.get(0) {
                            if let ScVal::Symbol(sym) = topic {
                                match sym.to_string().as_str() {
                                    s if s.contains("require_auth") => "require_auth",
                                    s if s.contains("set") || s.contains("write") => {
                                        "storage_write"
                                    }
                                    _ => "contract",
                                }
                            } else {
                                "contract"
                            }
                        } else {
                            "contract"
                        }
                    }
                    ContractEventType::System => "system",
                    ContractEventType::Diagnostic => {
                        if let Some(topic) = topics.get(0) {
                            if let ScVal::Symbol(sym) = topic {
                                match sym.to_string().as_str() {
                                    s if s.contains("fn_call") => "invocation",
                                    s if s.contains("fn_return") => "return",
                                    _ => "diagnostic",
                                }
                            } else {
                                "diagnostic"
                            }
                        } else {
                            "diagnostic"
                        }
                    }
                };

                Some(CategorizedEvent {
                    event_type: event_type.to_string(),
                    contract_id: event
                        .event
                        .contract_id
                        .as_ref()
                        .map(|id| format!("{:?}", id)),
                    topics: topics.iter().map(|t| format!("{:?}", t)).collect(),
                    data: format!("{:?}", data_val),
                })
            } else {
                None
            }
        })
        .collect()
}

fn send_error(msg: String) {
    let res = SimulationResponse {
        status: "error".to_string(),
        error: Some(msg),
        events: vec![],
        categorized_events: vec![],
        logs: vec![],
    };
    println!("{}", serde_json::to_string(&res).unwrap());
}

mod test;
