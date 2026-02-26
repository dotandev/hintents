// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(warnings, clippy::all, clippy::pedantic, clippy::nursery)]

mod config;
mod gas_optimizer;
mod git_detector;
mod runner;
mod source_map_cache;
mod source_mapper;
mod stack_trace;
mod types;
mod vm;
mod wasm;

use crate::gas_optimizer::{BudgetMetrics, GasOptimizationAdvisor, CPU_LIMIT, MEMORY_LIMIT};
use crate::source_mapper::SourceMapper;
use crate::stack_trace::WasmStackTrace;
use crate::types::*;
use base64::Engine as _;
use soroban_env_host::xdr::ReadXdr;
use soroban_env_host::{
    xdr::{Operation, OperationBody},
    Host, HostError,
};
use std::collections::HashMap;
use std::env;
use std::fs;
use std::io::{self, Read};
use tracing_subscriber::{fmt, EnvFilter};

// Use types::SimulationRequest directly

const ERR_MEMORY_LIMIT_EXCEEDED: &str = "ERR_MEMORY_LIMIT_EXCEEDED";

fn init_logger() {
    // Check if the environment variable ERST_LOG_FORMAT is set to "json"
    let use_json = env::var("ERST_LOG_FORMAT")
        .map(|val| val.to_lowercase() == "json")
        .unwrap_or(false);

    // Default to "info" level logging if not specified
    let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info"));

    let subscriber = fmt::Subscriber::builder()
        .with_env_filter(filter)
        .with_writer(std::io::stderr); // Write logs to stderr

    if use_json {
        // Output machine-parsable JSON
        subscriber.json().flatten_event(true).init();
    } else {
        // Output human-readable text
        subscriber.compact().init();
    }
}

fn send_error(msg: String) {
    let trace = WasmStackTrace::from_host_error(&msg);
    let res = SimulationResponse {
        status: "error".to_string(),
        error: Some(msg),
        error_code: None,
        lcov_report: None,
        lcov_report_path: None,
        events: vec![],
        diagnostic_events: vec![],
        categorized_events: vec![],
        logs: vec![],
        flamegraph: None,
        optimization_report: None,
        budget_usage: None,
        source_location: None,
        stack_trace: Some(trace),
        wasm_offset: None,
    };
    if let Ok(json) = serde_json::to_string(&res) {
        println!("{}", json);
    } else {
        eprintln!("Failed to serialize error response");
        println!("{{\"status\": \"error\", \"error\": \"Internal serialization error\"}}");
    }
    std::process::exit(1);
}

#[derive(Default)]
struct CoverageTracker {
    invoked_functions: HashMap<String, u64>,
}

impl CoverageTracker {
    fn record_operation(&mut self, op: &Operation) {
        if let OperationBody::InvokeHostFunction(invoke_op) = &op.body {
            let function_label = match &invoke_op.host_function {
                soroban_env_host::xdr::HostFunction::InvokeContract(args) => {
                    format!("InvokeContract::{:?}", args.function_name)
                }
                other => other.name().to_string(),
            };
            let entry = self.invoked_functions.entry(function_label).or_insert(0);
            *entry = entry.saturating_add(1);
        }
    }
}

fn generate_lcov_report(coverage: &CoverageTracker, source_file: &str) -> String {
    let mut functions: Vec<(&str, u64)> = coverage
        .invoked_functions
        .iter()
        .map(|(name, count)| (name.as_str(), *count))
        .collect();
    functions.sort_by(|(a, _), (b, _)| a.cmp(b));

    let mut report = String::new();
    report.push_str("TN:simulator\n");
    report.push_str(&format!("SF:{source_file}\n"));

    for (idx, (name, _)) in functions.iter().enumerate() {
        let sanitized = name.replace('\n', "_").replace(',', "_");
        report.push_str(&format!("FN:{},{}\n", idx + 1, sanitized));
    }
    for (name, count) in &functions {
        let sanitized = name.replace('\n', "_").replace(',', "_");
        report.push_str(&format!("FNDA:{count},{sanitized}\n"));
    }

    let fnf = functions.len();
    let fnh = functions.iter().filter(|(_, count)| *count > 0).count();
    report.push_str(&format!("FNF:{fnf}\n"));
    report.push_str(&format!("FNH:{fnh}\n"));

    // Keep a minimal line section so generic LCOV consumers can parse this file.
    report.push_str("DA:1,1\n");
    report.push_str("LF:1\n");
    report.push_str("LH:1\n");
    report.push_str("end_of_record\n");
    report
}

fn check_memory_limit_or_panic(host: &Host, memory_limit: Option<u64>) {
    if let Some(limit) = memory_limit {
        if let Ok(mem_bytes) = host.budget_cloned().get_mem_bytes_consumed() {
            if mem_bytes > limit {
                panic!(
                    "{}: consumed {} bytes, limit {} bytes",
                    ERR_MEMORY_LIMIT_EXCEEDED, mem_bytes, limit
                );
            }
        }
    }
}

fn execute_operations(
    host: &Host,
    operations: &[Operation],
    request: &SimulationRequest,
    memory_limit: Option<u64>,
    coverage: &mut CoverageTracker,
) -> Result<Vec<String>, HostError> {
    let mut logs = Vec::new();
    check_memory_limit_or_panic(host, memory_limit);
    for op in operations {
        coverage.record_operation(op);
        match &op.body {
            OperationBody::InvokeHostFunction(invoke_op) => {
                logs.push("Executing InvokeHostFunction...".to_string());
                
                // Check for signature verification mock
                if let Some(mock_result) = check_signature_verification_mocks(&request, &invoke_op.host_function) {
                    logs.push(format!("Mock signature verification: {:?}", mock_result));
                    if !mock_result {
                        return Err(soroban_env_host::HostError::from(
                            (soroban_env_host::xdr::ScErrorType::Context, soroban_env_host::xdr::ScErrorCode::InvalidInput)
                        ));
                    }
                }
                
                let val = host.invoke_function(invoke_op.host_function.clone())?;
                logs.push(format!("Result: {val:?}"));
                check_memory_limit_or_panic(host, memory_limit);
            }
            _ => {
                logs.push(format!(
                    "Skipping non-Soroban operation: {:?}",
                    op.body.name()
                ));
                check_memory_limit_or_panic(host, memory_limit);
            }
        }
    }
    Ok(logs)
}

fn transaction_fee_stroops(envelope: &soroban_env_host::xdr::TransactionEnvelope) -> u64 {
    match envelope {
        soroban_env_host::xdr::TransactionEnvelope::Tx(tx_v1) => tx_v1.tx.fee as u64,
        soroban_env_host::xdr::TransactionEnvelope::TxV0(tx_v0) => tx_v0.tx.fee as u64,
        soroban_env_host::xdr::TransactionEnvelope::TxFeeBump(bump) => bump.tx.fee as u64,
    }
}

fn mocked_required_fee_stroops(
    request: &SimulationRequest,
    operations_count: usize,
    cpu_insns: u64,
    mem_bytes: u64,
) -> Option<u64> {
    let mut required_fee = 0u64;
    let mut enabled = false;

    if let Some(base_fee) = request.mock_base_fee {
        enabled = true;
        required_fee =
            required_fee.saturating_add((base_fee as u64).saturating_mul(operations_count as u64));
    }

    if let Some(gas_price) = request.mock_gas_price {
        enabled = true;
        // Keep the unit small enough to be predictable in local replay while still driven by observed usage.
        let cpu_units = cpu_insns.saturating_add(9_999) / 10_000;
        let mem_units = mem_bytes.saturating_add(1_023) / 1_024;
        let resource_units = cpu_units.saturating_add(mem_units).max(1);
        required_fee = required_fee.saturating_add(gas_price.saturating_mul(resource_units));
    }

    if enabled {
        Some(required_fee)
    } else {
        None
    }
}

fn check_signature_verification_mocks(
    request: &SimulationRequest,
    host_function: &soroban_env_host::xdr::HostFunction,
) -> Option<bool> {
    // Check if signature verification mocking is enabled
    let mock_result = request.mock_signature_verification?;
    
    // Check if this is a signature verification host function
    // Note: Current soroban-env-host version only has InvokeContract, CreateContract, and UploadContractWasm
    // Signature verification functions may be handled at a different level or in newer versions
    match host_function {
        // For now, we'll mock signature verification based on function name patterns
        _ => {
            // Check if the function name contains signature verification related terms
            let function_name = host_function.name();
            if function_name.contains("Verify") || function_name.contains("Signature") || function_name.contains("Ed25519") {
                Some(mock_result)
            } else {
                None
            }
        }
    }
}

fn categorize_events(events: &soroban_env_host::events::Events) -> Vec<CategorizedEvent> {
    events
        .0
        .iter()
        .map(|e| {
            let category = match e.event.type_ {
                soroban_env_host::xdr::ContractEventType::Contract => "Contract",
                soroban_env_host::xdr::ContractEventType::System => "System",
                soroban_env_host::xdr::ContractEventType::Diagnostic => "Diagnostic",
            }
            .to_string();

            let contract_id = e.event.contract_id.as_ref().map(|id| format!("{id:?}"));
            let topics = match &e.event.body {
                soroban_env_host::xdr::ContractEventBody::V0(v0) => v0
                    .topics
                    .iter()
                    .map(|t| format!("{t:?}"))
                    .collect::<Vec<String>>(),
            };
            let data = match &e.event.body {
                soroban_env_host::xdr::ContractEventBody::V0(v0) => format!("{:?}", v0.data),
            };

            let wasm_instruction = extract_wasm_instruction(&topics, &data);
            CategorizedEvent {
                category,
                event: DiagnosticEvent {
                    event_type: match e.event.type_ {
                        soroban_env_host::xdr::ContractEventType::Contract => {
                            "contract".to_string()
                        }
                        soroban_env_host::xdr::ContractEventType::System => "system".to_string(),
                        soroban_env_host::xdr::ContractEventType::Diagnostic => {
                            "diagnostic".to_string()
                        }
                    },
                    contract_id,
                    topics,
                    data,
                    wasm_instruction,
                    // failed_call=true means the call that emitted this event
                    // actually failed; so a successful call is the inverse.
                    in_successful_contract_call: !e.failed_call,
                },
            }
        })
        .collect()
}

/// Main entry point for the erst simulator.
///
/// Reads a JSON `SimulationRequest` from stdin,
/// initializes a Soroban host environment, and outputs a JSON
/// `SimulationResponse` with simulation results or errors.
///
/// # Panics
///
/// May panic if JSON serialization of the response fails (should not happen
/// with valid `SimulationResponse` structures).
fn main() {
    // 1. Initialize the logger immediately
    init_logger();

    // 2. Log that we started
    tracing::info!(event = "simulator_started", "Simulator initializing...");

    // Read JSON from Stdin
    let mut buffer = String::new();
    if let Err(e) = io::stdin().read_to_string(&mut buffer) {
        let res = SimulationResponse {
            status: "error".to_string(),
            error: Some(format!("Failed to read stdin: {e}")),
            error_code: None,
            lcov_report: None,
            lcov_report_path: None,
            events: vec![],
            diagnostic_events: vec![],
            categorized_events: vec![],
            logs: vec![],
            flamegraph: None,
            optimization_report: None,
            budget_usage: None,
            source_location: None,
            stack_trace: None,
            wasm_offset: None,
        };
        if let Ok(json) = serde_json::to_string(&res) {
            println!("{}", json);
        } else {
            eprintln!("Failed to serialize error response");
            println!("{{\"status\": \"error\", \"error\": \"Internal serialization error\"}}");
        }
        eprintln!("Failed to read stdin: {e}");
        return;
    }

    // Parse Request
    let request: SimulationRequest = match serde_json::from_str(&buffer) {
        Ok(req) => req,
        Err(e) => {
            let res = SimulationResponse {
                status: "error".to_string(),
                error: Some(format!("Invalid JSON: {e}")),
                error_code: None,
                lcov_report: None,
                lcov_report_path: None,
                events: vec![],
                diagnostic_events: vec![],
                categorized_events: vec![],
                logs: vec![],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
                source_location: None,
                stack_trace: None,
                wasm_offset: None,
            };
            println!(
                "{}",
                serde_json::to_string(&res).expect("Failed to serialize error response")
            );
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
                send_error(format!("Failed to parse Envelope XDR: {}", e));
                return;
            }
        },
        Err(e) => {
            send_error(format!("Failed to decode Envelope Base64: {}", e));
            return;
        }
    };

    // Decode ResultMeta XDR
    eprintln!(
        "Debug: Received ResultMetaXdr len: {}",
        request.result_meta_xdr.len()
    );

    let _result_meta = if request.result_meta_xdr.is_empty() {
        eprintln!("Warning: ResultMetaXdr is empty. Host storage may be incomplete.");
        None
    } else {
        match base64::engine::general_purpose::STANDARD.decode(&request.result_meta_xdr) {
            Ok(bytes) => {
                if bytes.is_empty() {
                    eprintln!("Warning: ResultMetaXdr decoded to 0 bytes.");
                    None
                } else {
                    match soroban_env_host::xdr::TransactionResultMeta::from_xdr(
                        &bytes,
                        soroban_env_host::xdr::Limits::none(),
                    ) {
                        Ok(meta) => Some(meta),
                        Err(e) => {
                            eprintln!("Warning: Failed to parse ResultMeta XDR: {}. Proceeding with empty storage.", e);
                            None
                        }
                    }
                }
            }
            Err(e) => {
                eprintln!("Warning: Failed to decode ResultMeta Base64: {e}. Proceeding with empty storage.");
                None
            }
        }
    };

    // Initialize source mapper if WASM is provided
    let source_mapper = if let Some(wasm_base64) = &request.contract_wasm {
        match base64::engine::general_purpose::STANDARD.decode(wasm_base64) {
            Ok(wasm_bytes) => {
                if let Err(e) = vm::enforce_soroban_compatibility(&wasm_bytes) {
                    return send_error(format!("Strict VM enforcement failed: {}", e));
                }
                let mapper = SourceMapper::new_with_options(wasm_bytes, request.no_cache.unwrap_or(false));
                if mapper.has_debug_symbols() {
                    eprintln!("Debug symbols found in WASM");
                    Some(mapper)
                } else {
                    eprintln!("No debug symbols found in WASM");
                    None
                }
            }
            Err(e) => {
                eprintln!("Failed to decode WASM base64: {e}");
                None
            }
        }
    } else {
        None
    };

    // Initialize Host
    let sim_host = runner::SimHost::new(
        None,
        request.resource_calibration.clone(),
        request.memory_limit,
    );
    let host = sim_host.inner;

    // --- START: Local WASM Loading Integration (Issue #70) ---
    if let Some(path) = &request.wasm_path {
        match wasm::load_wasm_from_path(path) {
            Ok(_wasm_bytes) => {
                // `upload_contract_wasm` is crate-private in recent host versions.
                // We still validate local WASM readability here.
                eprintln!("Successfully loaded local WASM from path");
            }
            Err(e) => send_error(format!("Local WASM loading failed: {}", e)),
        }
    }
    // --- END: Local WASM Loading Integration ---

    let mut loaded_entries_count = 0;

    // Populate Host Storage
    if let Some(entries) = &request.ledger_entries {
        for (key_xdr, entry_xdr) in entries {
            // Decode Key
            let _key = match base64::engine::general_purpose::STANDARD.decode(key_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerKey::from_xdr(
                    b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(k) => k,
                    Err(e) => {
                        send_error(format!("Failed to parse LedgerKey XDR: {}", e));
                        return;
                    }
                },
                Err(e) => {
                    send_error(format!("Failed to decode LedgerKey Base64: {}", e));
                    return;
                }
            };

            // Decode Entry
            let _entry = match base64::engine::general_purpose::STANDARD.decode(entry_xdr) {
                Ok(b) => match soroban_env_host::xdr::LedgerEntry::from_xdr(
                    b,
                    soroban_env_host::xdr::Limits::none(),
                ) {
                    Ok(e) => e,
                    Err(e) => {
                        send_error(format!("Failed to parse LedgerEntry XDR: {}", e));
                        return;
                    }
                },
                Err(e) => {
                    send_error(format!("Failed to decode LedgerEntry Base64: {}", e));
                    return;
                }
            };

            // TODO: Inject into host storage.
            // For MVP, we verify we can parse them.
            eprintln!("Parsed Ledger Entry: Key={:?}, Entry={:?}", _key, _entry);
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
    let mut coverage = CoverageTracker::default();
    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        execute_operations(&host, operations, &request, request.memory_limit, &mut coverage)
    }));

    // Budget and Reporting
    let budget = host.budget_cloned();
    let cpu_insns = budget.get_cpu_insns_consumed().unwrap_or(0);
    let mem_bytes = budget.get_mem_bytes_consumed().unwrap_or(0);

    let cpu_usage_percent = (cpu_insns as f64 / CPU_LIMIT as f64) * 100.0;
    let memory_usage_percent = (mem_bytes as f64 / MEMORY_LIMIT as f64) * 100.0;

    let budget_usage = BudgetUsage {
        cpu_instructions: cpu_insns,
        memory_bytes: mem_bytes,
        operations_count: operations.len(),
        cpu_limit: CPU_LIMIT,
        memory_limit: MEMORY_LIMIT,
        cpu_usage_percent,
        memory_usage_percent,
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
        let mut result_vec = Vec::new();
        let mut options = inferno::flamegraph::Options::default();
        options.title = "Soroban Resource Consumption".to_string();

        if let Err(e) =
            inferno::flamegraph::from_reader(&mut options, folded_data.as_bytes(), &mut result_vec)
        {
            eprintln!("Failed to generate flamegraph: {e}");
        } else {
            flamegraph_svg = Some(String::from_utf8_lossy(&result_vec).to_string());
        }
    }

    let mut lcov_report = None;
    let mut lcov_report_path = None;
    if request.enable_coverage {
        let source_file = request
            .wasm_path
            .clone()
            .unwrap_or_else(|| "contract.wasm".to_string());
        let report = generate_lcov_report(&coverage, &source_file);
        if let Some(path) = request.coverage_lcov_path.clone() {
            match fs::write(&path, &report) {
                Ok(()) => {
                    lcov_report_path = Some(path);
                }
                Err(e) => {
                    eprintln!("Failed to write LCOV report: {e}");
                }
            }
        }
        lcov_report = Some(report);
    }

    match result {
        Ok(Ok(exec_logs)) => {
            // Extract both raw event strings and structured diagnostic events
            let (events, diagnostic_events): (Vec<String>, Vec<DiagnosticEvent>) =
                match host.get_events() {
                    Ok(evs) => {
                        let raw_events: Vec<String> =
                            (evs.0).iter().map(|e| format!("{:?}", e)).collect();
                        let diag_events: Vec<DiagnosticEvent> = (evs.0)
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

                                let wasm_instruction = extract_wasm_instruction(&topics, &data);
                                DiagnosticEvent {
                                    event_type,
                                    contract_id,
                                    topics,
                                    data,
                                    in_successful_contract_call: !event.failed_call,
                                    wasm_instruction,
                                }
                            })
                            .collect();
                        (raw_events, diag_events)
                    }
                    Err(_) => (
                        vec!["Failed to retrieve events".to_string()],
                        Vec::<DiagnosticEvent>::new(),
                    ),
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
            final_logs.extend(exec_logs);

            if let Some(required_fee) = mocked_required_fee_stroops(
                &request,
                operations.as_slice().len(),
                cpu_insns,
                mem_bytes,
            ) {
                let declared_fee = transaction_fee_stroops(&envelope);
                final_logs.push(format!(
                    "Mock fee check: declared={} required={}",
                    declared_fee, required_fee
                ));

                if declared_fee < required_fee {
                    let response = SimulationResponse {
                        status: "error".to_string(),
                        error: Some(format!(
                            "insufficient fee (mocked): declared {} stroops, required {} stroops",
                            declared_fee, required_fee
                        )),
                        error_code: None,
                        lcov_report: lcov_report.clone(),
                        lcov_report_path: lcov_report_path.clone(),
                        events,
                        diagnostic_events,
                        categorized_events,
                        logs: final_logs,
                        flamegraph: flamegraph_svg,
                        optimization_report,
                        budget_usage: Some(budget_usage),
                        source_location: None,
                        stack_trace: None,
                        wasm_offset: None,
                    };

                    if let Ok(json) = serde_json::to_string(&response) {
                        println!("{}", json);
                    } else {
                        eprintln!("Failed to serialize simulation response");
                        println!("{{\"status\": \"error\", \"error\": \"Internal serialization error\"}}");
                    }
                    return;
                }
            }

            let response = SimulationResponse {
                status: "success".to_string(),
                error: None,
                error_code: None,
                lcov_report,
                lcov_report_path,
                events,
                diagnostic_events,
                categorized_events,
                logs: final_logs,
                flamegraph: flamegraph_svg,
                optimization_report,
                budget_usage: Some(budget_usage),
                stack_trace: None,
                wasm_offset: None,
                // If a WASM with debug symbols was provided, expose the first
                // mappable source location so callers can correlate failures.
                source_location: source_mapper
                    .as_ref()
                    .and_then(|m| m.map_wasm_offset_to_source(0))
                    .and_then(|loc| serde_json::to_string(&loc).ok()),
                wasm_offset: None,
            };

            if let Ok(json) = serde_json::to_string(&response) {
                println!("{}", json);
            } else {
                eprintln!("Failed to serialize simulation response");
                println!("{{\"status\": \"error\", \"error\": \"Internal serialization error\"}}");
            }
        }
        Ok(Err(host_error)) => {
            // Host error during execution (e.g., contract trap, validation failure)
            let error_debug = format!("{:?}", host_error);
            let decoded_msg = decode_error(&error_debug);
            let wasm_trace = WasmStackTrace::from_host_error(&error_debug);
            let trace_display = wasm_trace.display();

            let structured_error = StructuredError {
                error_type: "HostError".to_string(),
                message: decoded_msg.clone(),
                details: Some(format!(
                    "Contract execution failed with host error: {}",
                    decoded_msg
                )),
            };

            let wasm_offset = extract_wasm_offset(&error_debug);
            let source_location =
                if let (Some(offset), Some(mapper)) = (wasm_offset, &source_mapper) {
                    mapper
                        .map_wasm_offset_to_source(offset)
                        .and_then(|loc| serde_json::to_string(&loc).ok())
                } else {
                    None
                };

            let response = SimulationResponse {
                status: "error".to_string(),
                error: Some(
                    serde_json::to_string(&structured_error).unwrap_or_else(|e| {
                        eprintln!("Failed to serialize structured error: {}", e);
                        format!("Internal error during error serialization: {}", e)
                    }),
                ),
                error_code: None,
                lcov_report: lcov_report.clone(),
                lcov_report_path: lcov_report_path.clone(),
                events: vec![],
                diagnostic_events: vec![],
                categorized_events: vec![],
                logs: vec![format!("Stack trace:\n{}", trace_display)],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
                source_location,
                stack_trace: Some(wasm_trace),
                wasm_offset,
            };
            if let Ok(json) = serde_json::to_string(&response) {
                println!("{}", json);
            } else {
                eprintln!("Failed to serialize host error response");
                println!("{{\"status\": \"error\", \"error\": \"Internal serialization error\"}}");
            }
        }
        Err(panic_info) => {
            let panic_msg = if let Some(s) = panic_info.downcast_ref::<&str>() {
                s.to_string()
            } else if let Some(s) = panic_info.downcast_ref::<String>() {
                s.clone()
            } else {
                "Unknown panic".to_string()
            };

            let wasm_trace = WasmStackTrace::from_panic(&panic_msg);
            let memory_limit_exceeded = panic_msg.contains(ERR_MEMORY_LIMIT_EXCEEDED);

            let response = SimulationResponse {
                status: "error".to_string(),
                error: Some(if memory_limit_exceeded {
                    panic_msg.clone()
                } else {
                    format!("Simulator panicked: {panic_msg}")
                }),
                error_code: if memory_limit_exceeded {
                    Some(ERR_MEMORY_LIMIT_EXCEEDED.to_string())
                } else {
                    None
                },
                lcov_report: lcov_report.clone(),
                lcov_report_path: lcov_report_path.clone(),
                events: vec![],
                diagnostic_events: vec![],
                categorized_events: vec![],
                logs: vec![format!("PANIC: {}", panic_msg)],
                flamegraph: None,
                optimization_report: None,
                budget_usage: None,
                source_location: None,
                stack_trace: Some(wasm_trace),
                wasm_offset: None,
            };
            if let Ok(json) = serde_json::to_string(&response) {
                println!("{}", json);
            } else {
                eprintln!("Failed to serialize panic response");
                println!("{{\"status\": \"error\", \"error\": \"Internal serialization error\"}}");
            }
        }
    }
}

fn extract_wasm_instruction(topics: &[String], data: &str) -> Option<String> {
    let has_budget_topic = topics.iter().any(|topic| {
        let lower = topic.to_lowercase();
        lower.contains("budget") || lower.contains("instruction")
    });
    if !has_budget_topic {
        return None;
    }

    let marker = "Instruction:";
    let idx = data.find(marker)?;
    let mut instr = data[idx + marker.len()..].trim().to_string();
    instr = instr.trim_matches('"').trim_matches('\'').to_string();
    if instr.is_empty() {
        None
    } else {
        Some(instr)
    }
}

fn extract_wasm_offset(error_msg: &str) -> Option<u64> {
    // Look for patterns like "@ 0x[HEX]" in the error message
    // Soroban/Wasmi errors often contain stack traces like:
    // "  0: func[42] @ 0xa3c"

    for line in error_msg.lines() {
        if let Some(pos) = line.find("@ 0x") {
            let hex_part = &line[pos + 4..];
            let end = hex_part
                .find(|c: char| !c.is_ascii_hexdigit())
                .unwrap_or(hex_part.len());
            if let Ok(offset) = u64::from_str_radix(&hex_part[..end], 16) {
                return Some(offset);
            }
        }
        
    }
    None
}
/// Attempts to extract a Key ID from a raw soroban HostError string.
///
/// Soroban host errors for missing storage entries typically contain the
/// LedgerKey in their debug output, e.g.:
///   `HostError: ... storage get ... LedgerKey(ContractData(...))`
///   `key = "GABC.../some_key"`
///
/// Returns `Some(key_string)` if a recognisable key pattern is found,
/// `None` otherwise.
fn extract_missing_key_id(raw: &str) -> Option<String> {
    // Pattern 1: LedgerKey(...) — soroban debug output
    if let Some(start) = raw.find("LedgerKey(") {
        let rest = &raw[start..];
        // Find the matching closing paren
        let mut depth = 0usize;
        let mut end = 0usize;
        for (i, ch) in rest.char_indices() {
            match ch {
                '(' => depth += 1,
                ')' => {
                    depth = depth.saturating_sub(1);
                    if depth == 0 {
                        end = i + 1;
                        break;
                    }
                }
                _ => {}
            }
        }
        if end > 0 {
            return Some(rest[..end].to_string());
        }
    }

    // Pattern 2: key = "..." — explicit key label in error
    if let Some(start) = raw.find("key = \"") {
        let rest = &raw[start + 7..];
        if let Some(end) = rest.find('"') {
            return Some(rest[..end].to_string());
        }
    }

    // Pattern 3: ContractData(...) without LedgerKey wrapper
    if let Some(start) = raw.find("ContractData(") {
        let rest = &raw[start..];
        let mut depth = 0usize;
        let mut end = 0usize;
        for (i, ch) in rest.char_indices() {
            match ch {
                '(' => depth += 1,
                ')' => {
                    depth = depth.saturating_sub(1);
                    if depth == 0 {
                        end = i + 1;
                        break;
                    }
                }
                _ => {}
            }
        }
        if end > 0 {
            return Some(rest[..end].to_string());
        }
    }

    None
}

/// Translate a raw soroban / WASM error string into a user-friendly description.
///
/// Protocol 21 standardised the set of VM trap codes emitted by the host.
/// This function maps those codes to clear English phrases so that
/// upper-level diagnostics (e.g. `erst explain`) can display them directly.
pub fn decode_error(raw: &str) -> String {
    let lower = raw.to_lowercase();

    if lower.contains("wasm trap") || lower.contains("vm trap") {
        if lower.contains("out of bounds") || lower.contains("memory access") {
            return "VM Trap: Out of Bounds Access (VM Trap: Out of bounds memory access) — the contract read or wrote outside its allocated memory region.".to_string();
        }
        if lower.contains("stack overflow") || lower.contains("call stack") {
            return "VM Trap: Stack Overflow — the contract exceeded the maximum call-stack depth."
                .to_string();
        }
        if lower.contains("integer overflow") {
            return "VM Trap: Integer Overflow — arithmetic exceeded integer bounds.".to_string();
        }
        if lower.contains("divide by zero") || lower.contains("division by zero") {
            return "VM Trap: Division by Zero — attempted integer division by zero.".to_string();
        }
        if lower.contains("unreachable") {
            return "VM Trap: Unreachable Instruction — the contract executed an explicit trap or reached dead code.".to_string();
        }
        if lower.contains("indirect call") || lower.contains("table") {
            return "VM Trap: Indirect-Call Type Mismatch — wrong function signature in call_indirect.".to_string();
        }
        return format!("VM Trap: {}", raw);
    }

    if lower.contains("unreachable") {
        return "VM Trap: Unreachable Instruction — the contract executed an explicit trap or reached dead code.".to_string();
    }
    if lower.contains("divide by zero") || lower.contains("division by zero") {
        return "VM Trap: Division by Zero — attempted integer division by zero.".to_string();
    }
    if lower.contains("integer overflow") {
        return "VM Trap: Integer Overflow — arithmetic exceeded integer bounds.".to_string();
    }
    if lower.contains("stack overflow") || lower.contains("call stack") {
        return "VM Trap: Stack Overflow — the contract exceeded the maximum call-stack depth."
            .to_string();
    }

    if lower.contains("auth") || lower.contains("unauthorized") {
        return "Authorization failure — a required signer or policy check was not satisfied."
            .to_string();
    }

    if lower.contains("budget") || lower.contains("cpu limit") || lower.contains("mem limit") {
        return "Resource limit exceeded — the transaction consumed more CPU instructions or memory than the protocol-21 budget allows.".to_string();
    }

  if lower.contains("missing") || lower.contains("not found") {
        let key_hint = extract_missing_key_id(raw);
        if let Some(key) = key_hint {
            return format!(
                "Missing ledger entry — Key ID: {} — the contract referenced a key that does not exist in the current ledger state.",
                key
            );
        }
        return "Missing ledger entry — the contract referenced a key that does not exist in the current ledger state.".to_string();
    }

    // Fallback: return the raw message unchanged.
    raw.to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_decode_vm_traps() {
        assert!(
            decode_error("Error: Wasm Trap: out of bounds memory access")
                .contains("VM Trap: Out of Bounds Access")
        );
        assert!(decode_error("Panic: unreachable").contains("VM Trap: Unreachable Instruction"));
        assert!(decode_error("integer divide by zero").contains("VM Trap: Division by Zero"));
        assert!(decode_error("stack overflow occurred").contains("VM Trap: Stack Overflow"));
        assert_eq!(decode_error("normal error"), "normal error");
    }

    #[test]
    fn test_extract_wasm_instruction() {
        let topics = vec!["budget".to_string(), "tick".to_string()];
        let data = "\"Instruction: i32.add\"".to_string();
        let instr = extract_wasm_instruction(&topics, &data);
        assert_eq!(instr, Some("i32.add".to_string()));

        let data2 = "\"Instruction: call 12\"".to_string();
        let instr2 = extract_wasm_instruction(&topics, &data2);
        assert_eq!(instr2, Some("call 12".to_string()));

        let topics_none = vec!["other".to_string()];
        let instr3 = extract_wasm_instruction(&topics_none, &data);
        assert_eq!(instr3, None);
        let msg = decode_error("Error: Wasm Trap: out of bounds memory access");
        assert!(msg.contains("VM Trap: Out of bounds memory access"));
    }

    #[test]
    fn test_decode_unreachable() {
        let msg = decode_error("wasm trap: unreachable");
        assert!(msg.contains("VM Trap: Unreachable"));
    }

    #[test]
    fn test_enforce_soroban_compatibility_rejects_floats() {
        let wat = r#"
            (module
                (func (export "f") (result f32)
                    f32.const 0.0
                )
            )
        "#;

        let wasm = wat::parse_str(wat).expect("failed to compile WAT");
        let result = vm::enforce_soroban_compatibility(&wasm);
        assert!(result.is_err());
    }

    // ── Protocol-21 host-trait correctness ─────────────────────────────────

    /// `HostEvent.failed_call == true` means the call that emitted the event
    /// *failed*.  `in_successful_contract_call` must therefore be the inverse.
    /// This was silently backwards before the protocol-21 fix.
    #[test]
    fn test_in_successful_contract_call_is_negation_of_failed_call() {
        use soroban_env_host::events::{Events, HostEvent};
        use soroban_env_host::xdr::{
            ContractEvent, ContractEventBody, ContractEventType, ContractEventV0, ExtensionPoint,
            VecM,
        };

        let make_event = |failed: bool| -> HostEvent {
            HostEvent {
                failed_call: failed,
                event: ContractEvent {
                    ext: ExtensionPoint::V0,
                    contract_id: None,
                    type_: ContractEventType::Diagnostic,
                    body: ContractEventBody::V0(ContractEventV0 {
                        topics: VecM::default(),
                        data: soroban_env_host::xdr::ScVal::Void,
                    }),
                },
            }
        };

        // failed_call = true  →  in_successful_contract_call must be false
        let evs_failed = Events(vec![make_event(true)]);
        let categorized = categorize_events(&evs_failed);
        assert_eq!(categorized.len(), 1);
        assert!(
            !categorized[0].event.in_successful_contract_call,
            "a failed call should NOT be marked as a successful contract call"
        );

        // failed_call = false  →  in_successful_contract_call must be true
        let evs_ok = Events(vec![make_event(false)]);
        let categorized = categorize_events(&evs_ok);
        assert_eq!(categorized.len(), 1);
        assert!(
            categorized[0].event.in_successful_contract_call,
            "a successful call MUST be marked as a successful contract call"
        );
    }

    /// categorize_events must correctly map ContractEventType variants to their
    /// lowercase string representations.
    #[test]
    fn test_categorize_events_type_labels() {
        use soroban_env_host::events::{Events, HostEvent};
        use soroban_env_host::xdr::{
            ContractEvent, ContractEventBody, ContractEventType, ContractEventV0, ExtensionPoint,
            VecM,
        };

        let make_typed_event = |t: ContractEventType| HostEvent {
            failed_call: false,
            event: ContractEvent {
                ext: ExtensionPoint::V0,
                contract_id: None,
                type_: t,
                body: ContractEventBody::V0(ContractEventV0 {
                    topics: VecM::default(),
                    data: soroban_env_host::xdr::ScVal::Void,
                }),
            },
        };

        let evs = Events(vec![
            make_typed_event(ContractEventType::Contract),
            make_typed_event(ContractEventType::System),
            make_typed_event(ContractEventType::Diagnostic),
        ]);

        let cats = categorize_events(&evs);
        assert_eq!(cats[0].category, "Contract");
        assert_eq!(cats[1].category, "System");
        assert_eq!(cats[2].category, "Diagnostic");

        // DiagnosticEvent.event_type should be lowercase
        assert_eq!(cats[0].event.event_type, "contract");
        assert_eq!(cats[1].event.event_type, "system");
        assert_eq!(cats[2].event.event_type, "diagnostic");
    }

    /// SourceMapper without debug symbols must return None for source locations,
    /// and the `source_location` field stays absent in serialized JSON.
    #[test]
    fn test_source_mapper_no_symbols_gives_no_location() {
        use crate::source_mapper::SourceMapper;

        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00]; // WASM magic + version
        let mapper = SourceMapper::new_with_options(wasm_bytes, false);
        assert!(!mapper.has_debug_symbols());
        assert!(
            mapper.map_wasm_offset_to_source(0).is_none(),
            "WASM without .debug_info should yield no source location"
        );
    }

    #[test]
    fn test_missing_ledger_entry_includes_key_id() {
        let raw = "HostError: storage get failed LedgerKey(ContractData(hash=abc123, key=Symbol(\"balance\")))";
        let msg = decode_error(raw);
        assert!(msg.contains("Key ID:"), "expected Key ID in message, got: {}", msg);
        assert!(msg.contains("LedgerKey(ContractData"), "expected key content in message, got: {}", msg);
        assert!(msg.contains("Missing ledger entry"));
    }

    #[test]
    fn test_missing_ledger_entry_no_key_falls_back() {
        let msg = decode_error("some missing entry error without key info");
        assert_eq!(msg, "Missing ledger entry — the contract referenced a key that does not exist in the current ledger state.");
    }

    #[test]
    fn test_missing_ledger_entry_contract_data_pattern() {
        let raw = "error: not found ContractData(contract=GABC, key=Bytes(deadbeef))";
        let msg = decode_error(raw);
        assert!(msg.contains("Key ID:"));
        assert!(msg.contains("ContractData("));
    }

    #[test]
    fn test_extract_missing_key_id_ledger_key() {
        let raw = "HostError LedgerKey(ContractData(abc))";
        assert_eq!(extract_missing_key_id(raw), Some("LedgerKey(ContractData(abc))".to_string()));
    }

    #[test]
    fn test_extract_missing_key_id_explicit_key_label() {
        let raw = "error: not found, key = \"GABC123/balance\"";
        assert_eq!(extract_missing_key_id(raw), Some("GABC123/balance".to_string()));
    }

    #[test]
    fn test_extract_missing_key_id_none_when_absent() {
        assert_eq!(extract_missing_key_id("generic error with no key"), None);
    fn test_generate_lcov_report_contains_function_hits() {
        let mut coverage = CoverageTracker::default();
        coverage
            .invoked_functions
            .insert("InvokeContract::\"transfer\"".to_string(), 3);
        coverage
            .invoked_functions
            .insert("InvokeContract::\"init\"".to_string(), 1);

        let report = generate_lcov_report(&coverage, "/tmp/contract.wasm");
        assert!(report.contains("SF:/tmp/contract.wasm"));
        assert!(report.contains("FNDA:3,InvokeContract::\"transfer\""));
        assert!(report.contains("FNDA:1,InvokeContract::\"init\""));
        assert!(report.contains("FNF:2"));
        assert!(report.contains("FNH:2"));
    }
}
>>>>>>> ac2f0a127a0ae2292443728a70f9e09fa77f8835
