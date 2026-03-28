// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Snapshot export utilities.
//!
//! Supports writing a [`StateSnapshot`] to a file in either binary (bincode)
//! or human-readable JSON format.  The JSON output is pretty-printed and
//! structured to match the Soroban RPC schema so it can be used for manual
//! auditing or fed into external tooling.
//!
//! # Example
//! ```ignore
//! use crate::export::{export_snapshot, ExportFormat};
//! use crate::types::StateSnapshot;
//!
//! let snap = StateSnapshot::default();
//! export_snapshot(&snap, "/tmp/frame_3.json", ExportFormat::Json)?;
//! export_snapshot(&snap, "/tmp/frame_3.bin",  ExportFormat::Binary)?;
//! ```

use crate::types::StateSnapshot;
use std::fs;
use std::io;
use std::path::Path;
use thiserror::Error;

/// Selects the serialization format used by [`export_snapshot`].
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ExportFormat {
    /// Human-readable, pretty-printed JSON.
    /// Matches the Soroban RPC schema for manual auditing.
    #[default]
    Json,
    /// Compact binary encoding via `bincode`.
    /// Used internally for fast snapshot storage and rollback.
    Binary,
}

impl ExportFormat {
    /// Parses a CLI flag value (`"json"` or `"binary"`) into an [`ExportFormat`].
    ///
    /// Case-insensitive.  Returns an error for unrecognised values.
    pub fn from_flag(flag: &str) -> Result<Self, ExportError> {
        match flag.to_lowercase().as_str() {
            "json" => Ok(ExportFormat::Json),
            "binary" | "bin" => Ok(ExportFormat::Binary),
            other => Err(ExportError::UnknownFormat(other.to_string())),
        }
    }

    /// Returns the conventional file extension for this format.
    pub fn extension(&self) -> &'static str {
        match self {
            ExportFormat::Json => "json",
            ExportFormat::Binary => "bin",
        }
    }
}

/// Errors that can occur during snapshot export.
#[derive(Debug, Error)]
pub enum ExportError {
    #[error("JSON serialization failed: {0}")]
    Json(#[from] serde_json::Error),

    #[error("Binary serialization failed: {0}")]
    Binary(String),

    #[error("Failed to write snapshot file: {0}")]
    Io(#[from] io::Error),

    #[error("Unknown export format '{0}': expected 'json' or 'binary'")]
    UnknownFormat(String),
}

/// Serialises `snapshot` and writes it to `path` using the given `format`.
///
/// * **JSON** — pretty-printed, UTF-8.  The output structure mirrors the
///   Soroban RPC snapshot schema so external auditing tools can consume it
///   without additional transformation.
/// * **Binary** — compact `bincode` encoding used for fast internal storage.
///
/// Parent directories are created automatically if they do not exist.
///
/// # Errors
/// Returns [`ExportError`] if serialization or the file write fails.
pub fn export_snapshot(
    snapshot: &StateSnapshot,
    path: impl AsRef<Path>,
    format: ExportFormat,
) -> Result<(), ExportError> {
    let path = path.as_ref();

    // Create parent directories if needed.
    if let Some(parent) = path.parent() {
        if !parent.as_os_str().is_empty() {
            fs::create_dir_all(parent)?;
        }
    }

    match format {
        ExportFormat::Json => {
            let json = to_json_pretty(snapshot)?;
            fs::write(path, json)?;
        }
        ExportFormat::Binary => {
            let bytes = to_binary(snapshot)?;
            fs::write(path, bytes)?;
        }
    }

    Ok(())
}

/// Serialises `snapshot` to a pretty-printed JSON string.
///
/// The output is structured to match the Soroban RPC snapshot schema:
/// ```json
/// {
///   "ledger_entries": { "<key_xdr>": "<entry_xdr>", ... },
///   "timestamp": 1712345678,
///   "instruction_index": 42,
///   "events": ["..."]
/// }
/// ```
pub fn to_json_pretty(snapshot: &StateSnapshot) -> Result<String, ExportError> {
    Ok(serde_json::to_string_pretty(snapshot)?)
}

/// Serialises `snapshot` to compact binary bytes using `bincode`.
pub fn to_binary(snapshot: &StateSnapshot) -> Result<Vec<u8>, ExportError> {
    bincode::serialize(snapshot)
        .map_err(|e| ExportError::Binary(e.to_string()))
}

/// Deserialises a [`StateSnapshot`] from the raw bytes of an exported file.
///
/// Automatically detects the format: if the bytes are valid UTF-8 starting
/// with `{`, JSON is assumed; otherwise binary (`bincode`) is tried.
///
/// # Errors
/// Returns [`ExportError`] if neither format succeeds.
pub fn load_snapshot(bytes: &[u8]) -> Result<StateSnapshot, ExportError> {
    // Heuristic: JSON objects always start with '{' (after optional BOM/space).
    if bytes.first().copied() == Some(b'{') {
        let snapshot = serde_json::from_slice(bytes)?;
        return Ok(snapshot);
    }
    bincode::deserialize(bytes)
        .map_err(|e| ExportError::Binary(e.to_string()))
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::StateSnapshot;
    use std::collections::HashMap;
    use tempfile::tempdir;

    fn sample_snapshot() -> StateSnapshot {
        let mut entries = HashMap::new();
        entries.insert("key_xdr_1".to_string(), "entry_xdr_1".to_string());
        entries.insert("key_xdr_2".to_string(), "entry_xdr_2".to_string());

        StateSnapshot {
            ledger_entries: entries,
            timestamp: 1_712_345_678,
            instruction_index: 42,
            events: vec!["event_a".to_string(), "event_b".to_string()],
        }
    }

    #[test]
    fn test_json_pretty_output_is_readable() {
        let snap = sample_snapshot();
        let json = to_json_pretty(&snap).expect("JSON serialization failed");

        // Must be pretty-printed (contains newlines and indentation).
        assert!(json.contains('\n'));
        assert!(json.contains("  "));

        // Must contain expected fields.
        assert!(json.contains("ledger_entries"));
        assert!(json.contains("timestamp"));
        assert!(json.contains("instruction_index"));
        assert!(json.contains("events"));
    }

    #[test]
    fn test_json_roundtrip() {
        let snap = sample_snapshot();
        let json = to_json_pretty(&snap).unwrap();
        let restored: StateSnapshot = serde_json::from_str(&json).unwrap();

        assert_eq!(restored.timestamp, snap.timestamp);
        assert_eq!(restored.instruction_index, snap.instruction_index);
        assert_eq!(restored.events, snap.events);
        assert_eq!(restored.ledger_entries, snap.ledger_entries);
    }

    #[test]
    fn test_binary_roundtrip() {
        let snap = sample_snapshot();
        let bytes = to_binary(&snap).unwrap();
        let restored: StateSnapshot = bincode::deserialize(&bytes).unwrap();

        assert_eq!(restored.timestamp, snap.timestamp);
        assert_eq!(restored.ledger_entries, snap.ledger_entries);
    }

    #[test]
    fn test_export_json_writes_file() {
        let dir = tempdir().unwrap();
        let path = dir.path().join("snap.json");

        export_snapshot(&sample_snapshot(), &path, ExportFormat::Json).unwrap();

        let contents = fs::read_to_string(&path).unwrap();
        assert!(contents.contains("ledger_entries"));
        assert!(contents.contains('\n')); // pretty-printed
    }

    #[test]
    fn test_export_binary_writes_file() {
        let dir = tempdir().unwrap();
        let path = dir.path().join("snap.bin");

        export_snapshot(&sample_snapshot(), &path, ExportFormat::Binary).unwrap();

        let bytes = fs::read(&path).unwrap();
        assert!(!bytes.is_empty());
    }

    #[test]
    fn test_load_snapshot_detects_json() {
        let snap = sample_snapshot();
        let json = to_json_pretty(&snap).unwrap();
        let loaded = load_snapshot(json.as_bytes()).unwrap();
        assert_eq!(loaded.timestamp, snap.timestamp);
    }

    #[test]
    fn test_load_snapshot_detects_binary() {
        let snap = sample_snapshot();
        let bytes = to_binary(&snap).unwrap();
        let loaded = load_snapshot(&bytes).unwrap();
        assert_eq!(loaded.timestamp, snap.timestamp);
    }

    #[test]
    fn test_export_format_from_flag() {
        assert_eq!(ExportFormat::from_flag("json").unwrap(), ExportFormat::Json);
        assert_eq!(ExportFormat::from_flag("JSON").unwrap(), ExportFormat::Json);
        assert_eq!(ExportFormat::from_flag("binary").unwrap(), ExportFormat::Binary);
        assert_eq!(ExportFormat::from_flag("bin").unwrap(), ExportFormat::Binary);
        assert!(ExportFormat::from_flag("csv").is_err());
    }

    #[test]
    fn test_export_format_extension() {
        assert_eq!(ExportFormat::Json.extension(), "json");
        assert_eq!(ExportFormat::Binary.extension(), "bin");
    }

    #[test]
    fn test_export_creates_parent_dirs() {
        let dir = tempdir().unwrap();
        let path = dir.path().join("nested/deep/snap.json");

        export_snapshot(&sample_snapshot(), &path, ExportFormat::Json).unwrap();
        assert!(path.exists());
    }
}