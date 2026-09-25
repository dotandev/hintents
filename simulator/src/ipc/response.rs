// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use crate::types::SimulationResponse;
use serde::Serialize;
use std::collections::HashMap;
use std::io::{self, Write};
use thiserror::Error;

/// Maximum size in bytes of a single ledger-entry-sized blob embedded in a
/// serialized response payload (e.g. base64-encoded XDR, linear memory dumps,
/// pprof profiles). Mirrors the 10 MiB snapshot cap used by the Go bridge
/// (`internal/bridge/parser.go`'s `MaxSnapshotSize`) and the simulator's own
/// `snapshot::MAX_SNAPSHOT_SIZE`.
pub const MAX_LEDGER_ENTRY_SIZE: usize = 10 * 1024 * 1024;

/// Maximum total size in bytes of a serialized simulation response that is
/// safe to push over the IPC channel. Larger payloads indicate a pathological
/// response; they are rejected before serialization to avoid blocking stdout
/// with a multi-megabyte NDJSON frame.
pub const MAX_RESPONSE_PAYLOAD_SIZE: usize = 10 * 1024 * 1024;

/// Error raised when a simulation response violates a configured size bound.
#[derive(Error, Debug, PartialEq, Eq)]
pub enum ResponseBoundsError {
    #[error("ledger entry '{field}' exceeds size bound: {size} bytes (limit {limit} bytes)")]
    LedgerEntryTooLarge {
        field: &'static str,
        size: usize,
        limit: usize,
    },

    #[error("response payload exceeds size bound: {size} bytes (limit {limit} bytes)")]
    PayloadTooLarge { size: usize, limit: usize },

    #[error("response serialization failed: {0}")]
    Serialization(String),
}

/// A `Write` sink that only counts the bytes written without storing them.
/// Used to measure the exact serialized payload size without buffering the
/// whole JSON document in memory.
struct SizeSink {
    written: usize,
}

impl Write for SizeSink {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        self.written += buf.len();
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

/// Returns the exact size in bytes of `value` once serialized to JSON without
/// materializing the payload in memory.
#[allow(dead_code)]
pub fn serialized_size<T: Serialize>(value: &T) -> Result<usize, ResponseBoundsError> {
    let mut sink = SizeSink { written: 0 };
    serde_json::to_writer(&mut sink, value)
        .map_err(|e| ResponseBoundsError::Serialization(e.to_string()))?;
    Ok(sink.written)
}

/// Strictly validates that a single ledger entry (base64-encoded XDR) stays
/// within the configured size bound before it is serialized into a response.
#[allow(dead_code)]
pub fn validate_ledger_entry_size(entry: &str) -> Result<(), ResponseBoundsError> {
    let size = entry.len();
    if size > MAX_LEDGER_ENTRY_SIZE {
        return Err(ResponseBoundsError::LedgerEntryTooLarge {
            field: "ledger_entries",
            size,
            limit: MAX_LEDGER_ENTRY_SIZE,
        });
    }
    Ok(())
}

/// Strictly validates every ledger entry in a request payload so an oversized
/// entry cannot inflate the resulting response beyond IPC-safe bounds.
#[allow(dead_code)]
pub fn validate_ledger_entries(
    entries: &HashMap<String, String>,
) -> Result<(), ResponseBoundsError> {
    for entry in entries.values() {
        validate_ledger_entry_size(entry)?;
    }
    Ok(())
}

/// Yields the ledger-entry-sized string fields carried by a simulation
/// response together with a stable field name used for error reporting.
fn response_blobs(response: &SimulationResponse) -> Vec<(&'static str, &str)> {
    let mut blobs = Vec::new();
    if let Some(value) = &response.error {
        blobs.push(("error", value.as_str()));
    }
    if let Some(value) = &response.lcov_report {
        blobs.push(("lcov_report", value.as_str()));
    }
    if let Some(value) = &response.flamegraph {
        blobs.push(("flamegraph", value.as_str()));
    }
    if let Some(value) = &response.linear_memory_dump {
        blobs.push(("linear_memory_dump", value.as_str()));
    }
    if let Some(value) = &response.pprof_profile {
        blobs.push(("pprof_profile", value.as_str()));
    }
    for log in &response.logs {
        blobs.push(("logs", log.as_str()));
    }
    for event in &response.events {
        blobs.push(("events", event.as_str()));
    }
    blobs
}

/// Strictly validates the bounds of a simulation response before it is
/// serialized. Every ledger-entry-sized field must stay within
/// [`MAX_LEDGER_ENTRY_SIZE`] and the fully serialized payload must stay within
/// [`MAX_RESPONSE_PAYLOAD_SIZE`].
#[allow(dead_code)]
pub fn validate_response_bounds(response: &SimulationResponse) -> Result<(), ResponseBoundsError> {
    for (field, value) in response_blobs(response) {
        let size = value.len();
        if size > MAX_LEDGER_ENTRY_SIZE {
            return Err(ResponseBoundsError::LedgerEntryTooLarge {
                field,
                size,
                limit: MAX_LEDGER_ENTRY_SIZE,
            });
        }
    }

    let size = serialized_size(response)?;
    if size > MAX_RESPONSE_PAYLOAD_SIZE {
        return Err(ResponseBoundsError::PayloadTooLarge {
            size,
            limit: MAX_RESPONSE_PAYLOAD_SIZE,
        });
    }
    Ok(())
}

/// Validates the response bounds and, when in range, serializes the response
/// to JSON. Returns an error when a bound is violated so callers can emit a
/// safe fallback instead of pushing an oversized payload over IPC.
#[allow(dead_code)]
pub fn serialize_response(response: &SimulationResponse) -> Result<String, ResponseBoundsError> {
    validate_response_bounds(response)?;
    serde_json::to_string(response).map_err(|e| ResponseBoundsError::Serialization(e.to_string()))
}

/// Validates and emits a simulation response to stdout as a single NDJSON
/// line. When the response violates a size bound, a compact error payload is
/// emitted instead so the IPC consumer always receives a parseable frame.
#[allow(dead_code)]
pub fn emit_response(response: &SimulationResponse) {
    match serialize_response(response) {
        Ok(json) => {
            let stdout = io::stdout();
            let mut handle = stdout.lock();
            if writeln!(handle, "{json}").is_err() {
                eprintln!("bridge: failed to emit response");
            }
        }
        Err(e) => {
            eprintln!("bridge: response rejected by bounds validation: {e}");
            let fallback = serde_json::json!({
                "status": "error",
                "error": format!("Simulation response rejected: {e}"),
            });
            if let Ok(json) = serde_json::to_string(&fallback) {
                println!("{json}");
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::SimulationResponse;

    fn sample_response() -> SimulationResponse {
        SimulationResponse {
            status: "success".to_string(),
            error: None,
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
            linear_memory_dump: None,
            asset_anomalies: vec![],
            pprof_profile: None,
        }
    }

    #[test]
    fn test_serialized_size_matches_to_string() {
        let response = sample_response();
        let measured = serialized_size(&response).unwrap();
        let actual = serde_json::to_string(&response).unwrap().len();
        assert_eq!(measured, actual);
    }

    #[test]
    fn test_validate_ledger_entry_size_within_bound() {
        let entry = "a".repeat(1024);
        assert!(validate_ledger_entry_size(&entry).is_ok());
    }

    #[test]
    fn test_validate_ledger_entry_size_over_bound() {
        let entry = "a".repeat(MAX_LEDGER_ENTRY_SIZE + 1);
        let err = validate_ledger_entry_size(&entry).unwrap_err();
        assert!(matches!(
            err,
            ResponseBoundsError::LedgerEntryTooLarge { size, limit, .. }
                if size == MAX_LEDGER_ENTRY_SIZE + 1 && limit == MAX_LEDGER_ENTRY_SIZE
        ));
    }

    #[test]
    fn test_validate_ledger_entries_rejects_oversized() {
        let mut entries = HashMap::new();
        entries.insert("ok".to_string(), "small".to_string());
        entries.insert("big".to_string(), "b".repeat(MAX_LEDGER_ENTRY_SIZE + 1));
        assert!(validate_ledger_entries(&entries).is_err());
    }

    #[test]
    fn test_validate_response_bounds_small_payload_ok() {
        let response = sample_response();
        assert!(validate_response_bounds(&response).is_ok());
    }

    #[test]
    fn test_validate_response_bounds_rejects_oversized_blob() {
        let mut response = sample_response();
        response.linear_memory_dump = Some("x".repeat(MAX_LEDGER_ENTRY_SIZE + 1));
        let err = validate_response_bounds(&response).unwrap_err();
        assert!(matches!(
            err,
            ResponseBoundsError::LedgerEntryTooLarge {
                field: "linear_memory_dump",
                ..
            }
        ));
    }

    #[test]
    fn test_validate_response_bounds_rejects_oversized_payload() {
        let mut response = sample_response();
        // Each entry stays within the per-field bound, but the aggregate
        // serialized payload exceeds MAX_RESPONSE_PAYLOAD_SIZE.
        response.logs = vec![
            "l".repeat(MAX_LEDGER_ENTRY_SIZE),
            "m".repeat(MAX_LEDGER_ENTRY_SIZE),
        ];
        let err = validate_response_bounds(&response).unwrap_err();
        assert!(matches!(err, ResponseBoundsError::PayloadTooLarge { .. }));
    }

    #[test]
    fn test_serialize_response_ok() {
        let response = sample_response();
        let json = serialize_response(&response).unwrap();
        let parsed: serde_json::Value = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed["status"], "success");
    }

    #[test]
    fn test_serialize_response_rejects_oversized() {
        let mut response = sample_response();
        response.flamegraph = Some("f".repeat(MAX_LEDGER_ENTRY_SIZE + 1));
        assert!(serialize_response(&response).is_err());
    }
}
