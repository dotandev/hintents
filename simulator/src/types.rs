// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

use crate::gas_optimizer::OptimizationReport;
use crate::source_mapper::SourceLocation;
use crate::stack_trace::WasmStackTrace;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Deserialize)]
pub struct SimulationRequest {
    pub envelope_xdr: String,
    pub result_meta_xdr: String,
    pub ledger_entries: Option<HashMap<String, String>>,
    #[serde(default)]
    pub control_command: Option<String>,
    #[serde(default)]
    pub rewind_step: Option<u32>,
    #[serde(default)]
    pub fork_params: Option<HashMap<String, String>>,
    #[serde(default)]
    pub harness_reset: bool,
    /// Zstd-compressed, base64-encoded ledger_entries produced by the Go bridge.
    /// When present, takes precedence over the plain `ledger_entries` field.
    #[serde(default)]
    pub ledger_entries_zstd: Option<String>,
    pub contract_wasm: Option<String>,
    pub wasm_path: Option<String>, // Added for local loading
    #[serde(default)]
    pub no_cache: bool,
    pub enable_optimization_advisor: bool,
    pub profile: Option<bool>,
    #[serde(default)]
    pub _timestamp: Option<i64>,
    pub resource_calibration: Option<ResourceCalibration>,
    /// RFC 3339 timestamp supplied by the caller.  Preserved for future use
    /// (e.g. time-locked contract logic); not yet consumed by the simulator.
    #[allow(dead_code)]
    #[serde(default)]
    pub timestamp: String,
    pub mock_base_fee: Option<u32>,
    pub mock_gas_price: Option<u64>,
    pub mock_signature_verification: Option<bool>,
    #[serde(default)]
    pub enable_coverage: bool,
    #[serde(default)]
    pub coverage_lcov_path: Option<String>,
    /// Optional hard memory limit in bytes. If set, the simulator will panic
    /// when memory consumption exceeds this limit, simulating live network constraints.
    pub memory_limit: Option<u64>,
    #[serde(default)]
    pub restore_preamble: Option<serde_json::Value>,
    #[serde(default)]
    pub include_linear_memory: bool,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct ResourceCalibration {
    pub sha256_fixed: u64,
    pub sha256_per_byte: u64,
    pub keccak256_fixed: u64,
    pub keccak256_per_byte: u64,
    pub ed25519_fixed: u64,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct SnapshotMetadata {
    pub id: String,
    pub gas_consumed: u64,
    pub call_stack_depth: u32,
}

#[derive(Debug, Serialize)]
pub struct SimulationResponse {
    pub status: String,
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub lcov_report: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub lcov_report_path: Option<String>,
    pub events: Vec<String>,
    pub diagnostic_events: Vec<DiagnosticEvent>,
    pub categorized_events: Vec<CategorizedEvent>,
    pub logs: Vec<String>,
    pub flamegraph: Option<String>,
    pub optimization_report: Option<OptimizationReport>,
    pub budget_usage: Option<BudgetUsage>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_location: Option<SourceLocation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stack_trace: Option<WasmStackTrace>,
    pub wasm_offset: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub linear_memory_dump: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct DiagnosticEvent {
    pub event_type: String,
    pub contract_id: Option<String>,
    pub topics: Vec<String>,
    pub data: String,
    pub in_successful_contract_call: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub snapshot_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub snapshot_metadata: Option<SnapshotMetadata>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub wasm_instruction: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cpu: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mem: Option<u64>,
}

#[derive(Debug, Serialize)]
pub struct CategorizedEvent {
    pub category: String,
    pub event: DiagnosticEvent,
}

#[derive(Debug, Serialize)]
pub struct BudgetUsage {
    pub cpu_instructions: u64,
    pub memory_bytes: u64,
    pub operations_count: usize,
    pub cpu_limit: u64,
    pub memory_limit: u64,
    pub cpu_usage_percent: f64,
    pub memory_usage_percent: f64,
}

#[derive(Debug, Serialize)]
pub struct StructuredError {
    pub error_type: String,
    pub message: String,
    pub details: Option<String>,
}

/// Captures the ledger state at a specific point in time during simulation.
///
/// Serves as the foundation for the rollback mechanism, allowing the simulator
/// to restore a prior state by replaying from a known-good snapshot.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct StateSnapshot {
    /// Base64-encoded ledger entries (key → entry XDR) at the snapshot point.
    pub ledger_entries: HashMap<String, String>,
    /// Ledger timestamp (seconds since Unix epoch) at the snapshot point.
    pub timestamp: u64,
    /// Index of the WASM instruction at which the snapshot was taken.
    pub instruction_index: u32,
    /// Contract events emitted between the previous snapshot and this one.
    pub events: Vec<String>,
}
