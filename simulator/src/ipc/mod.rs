// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

pub mod decompress;
pub mod validate;

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::io::Write;

/// Identifies the kind of streaming frame emitted to stdout.
#[allow(dead_code)]
#[derive(Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum FrameType {
    /// Intermediate ledger snapshot produced during simulation.
    Snapshot,
    /// Terminal frame; payload is the complete SimulationResponse JSON.
    Final,
    /// Response to a FETCH_SNAPSHOT command from the Go bridge.
    FetchResponse,
}

/// A single newline-delimited JSON (NDJSON) frame written to stdout.
#[allow(dead_code)]
#[derive(Debug, Serialize, Deserialize)]
pub struct StreamFrame {
    #[serde(rename = "type")]
    pub frame_type: FrameType,
    pub seq: u32,
    pub data: serde_json::Value,
}

/// Control commands accepted from the Go bridge in SimulationRequest payloads.
#[derive(Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum BridgeControlCommand {
    RollbackAndResume,
}

impl StreamFrame {
    #[allow(dead_code)]
    pub fn emit(&self) {
        match serde_json::to_string(self) {
            Ok(line) => {
                let stdout = std::io::stdout();
                let mut handle = stdout.lock();
                let _ = writeln!(handle, "{line}");
            }
            Err(e) => {
                eprintln!("bridge: failed to serialize StreamFrame: {e}");
            }
        }
    }
}

#[allow(dead_code)]
pub fn emit_snapshot_frame(seq: u32, data: serde_json::Value) {
    StreamFrame { frame_type: FrameType::Snapshot, seq, data }.emit();
}

#[allow(dead_code)]
pub fn emit_final_frame(seq: u32, data: serde_json::Value) {
    StreamFrame { frame_type: FrameType::Final, seq, data }.emit();
}

#[derive(Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum CommandOpcode {
    FetchSnapshot,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CommandFrame {
    pub op: CommandOpcode,
    pub id: u32,
    #[serde(default = "default_batch_size")]
    pub batch_size: u32,
}

fn default_batch_size() -> u32 { 1 }

#[derive(Debug, Serialize, Deserialize)]
pub struct SnapshotEntry {
    pub seq: u32,
    pub data: serde_json::Value,
}

#[derive(Debug, Serialize)]
struct FetchResponseFrame {
    #[serde(rename = "type")]
    frame_type: FrameType,
    seq: u32,
    data: FetchResponseData,
}

#[derive(Debug, Serialize)]
struct FetchResponseData {
    pub snapshots: Vec<SnapshotEntry>,
}

#[derive(Debug, Default)]
pub struct SnapshotRegistry {
    entries: HashMap<u32, serde_json::Value>,
}

impl SnapshotRegistry {
    pub fn new() -> Self { Self::default() }

    pub fn insert(&mut self, seq: u32, data: serde_json::Value) {
        self.entries.insert(seq, data);
    }

    pub fn fetch(&self, id: u32, batch_size: u32) -> Vec<SnapshotEntry> {
        let count = batch_size.clamp(1, 5);
        (id..id.saturating_add(count))
            .filter_map(|seq| {
                self.entries.get(&seq).map(|data| SnapshotEntry { seq, data: data.clone() })
            })
            .collect()
    }
}

pub fn handle_stdin_command(registry: &SnapshotRegistry) {
    use std::io::BufRead;
    let stdin = std::io::stdin();
    let mut line = String::new();
    if stdin.lock().read_line(&mut line).unwrap_or(0) == 0 { return; }
    let cmd: CommandFrame = match serde_json::from_str(line.trim()) {
        Ok(c) => c,
        Err(e) => { eprintln!("ipc: failed to parse command: {e}"); return; }
    };
    match cmd.op {
        CommandOpcode::FetchSnapshot => {
            let snapshots = registry.fetch(cmd.id, cmd.batch_size);
            let response = FetchResponseFrame {
                frame_type: FrameType::FetchResponse,
                seq: cmd.id,
                data: FetchResponseData { snapshots },
            };
            match serde_json::to_string(&response) {
                Ok(json_line) => {
                    let stdout = std::io::stdout();
                    let mut handle = stdout.lock();
                    let _ = writeln!(handle, "{json_line}");
                }
                Err(e) => eprintln!("ipc: failed to serialize fetch response: {e}"),
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_frame_type_serialization() {
        assert_eq!(serde_json::to_string(&FrameType::Snapshot).unwrap(), "\"snapshot\"");
        assert_eq!(serde_json::to_string(&FrameType::Final).unwrap(), "\"final\"");
        assert_eq!(serde_json::to_string(&FrameType::FetchResponse).unwrap(), "\"fetchresponse\"");
    }

    #[test]
    fn test_stream_frame_roundtrip() {
        let frame = StreamFrame { frame_type: FrameType::Snapshot, seq: 3, data: serde_json::json!({"entries": 42}) };
        let json = serde_json::to_string(&frame).unwrap();
        let decoded: StreamFrame = serde_json::from_str(&json).unwrap();
        assert_eq!(decoded.frame_type, FrameType::Snapshot);
        assert_eq!(decoded.seq, 3);
        assert_eq!(decoded.data["entries"], 42);
    }

    #[test]
    fn test_emit_snapshot_frame_does_not_panic() {
        emit_snapshot_frame(0, serde_json::json!({"test": true}));
    }

    #[test]
    fn test_registry_insert_and_fetch_single() {
        let mut reg = SnapshotRegistry::new();
        reg.insert(0, serde_json::json!({"ledger": 0}));
        let result = reg.fetch(0, 1);
        assert_eq!(result.len(), 1);
        assert_eq!(result[0].seq, 0);
    }

    #[test]
    fn test_registry_batch_capped_at_5() {
        let mut reg = SnapshotRegistry::new();
        for i in 0..20u32 { reg.insert(i, serde_json::json!({"ledger": i})); }
        assert_eq!(reg.fetch(0, 10).len(), 5);
    }

    #[test]
    fn test_registry_missing_seqs_skipped() {
        let mut reg = SnapshotRegistry::new();
        reg.insert(0, serde_json::json!({}));
        reg.insert(2, serde_json::json!({}));
        let result = reg.fetch(0, 3);
        assert_eq!(result.len(), 2);
    }

    #[test]
    fn test_command_frame_deserialization() {
        let cmd: CommandFrame = serde_json::from_str(r#"{"op":"FETCH_SNAPSHOT","id":3,"batch_size":5}"#).unwrap();
        assert_eq!(cmd.op, CommandOpcode::FetchSnapshot);
        assert_eq!(cmd.id, 3);
        assert_eq!(cmd.batch_size, 5);
    }

    #[test]
    fn test_command_frame_default_batch_size() {
        let cmd: CommandFrame = serde_json::from_str(r#"{"op":"FETCH_SNAPSHOT","id":7}"#).unwrap();
        assert_eq!(cmd.batch_size, 1);
    }
}