// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

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
#[allow(dead_code)]
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

#[derive(Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
#[allow(dead_code)]
pub enum CommandOpcode {
    FetchSnapshot,
}

#[derive(Debug, Serialize, Deserialize)]
#[allow(dead_code)]
pub struct CommandFrame {
    pub op: CommandOpcode,
    pub id: u32,
    #[serde(default = "default_batch_size")]
    pub batch_size: u32,
}

fn default_batch_size() -> u32 {
    1
}

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
#[allow(dead_code)]
pub struct SnapshotRegistry {
    entries: HashMap<u32, serde_json::Value>,
}

impl SnapshotRegistry {
    #[allow(dead_code)]
    pub fn new() -> Self {
        Self::default()
    }

    #[allow(dead_code)]
    pub fn insert(&mut self, seq: u32, data: serde_json::Value) {
        self.entries.insert(seq, data);
    }

    #[allow(dead_code)]
    pub fn fetch(&self, id: u32, batch_size: u32) -> Vec<SnapshotEntry> {
        let count = batch_size.clamp(1, 5);
        (id..id.saturating_add(count))
            .filter_map(|seq| {
                self.entries.get(&seq).map(|data| SnapshotEntry {
                    seq,
                    data: data.clone(),
                })
            })
            .collect()
    }
}

#[allow(dead_code)]
pub fn emit_snapshot_frame(seq: u32, data: serde_json::Value) {
    StreamFrame {
        frame_type: FrameType::Snapshot,
        seq,
        data,
    }
    .emit();
}

#[allow(dead_code)]
pub fn emit_final_frame(seq: u32, data: serde_json::Value) {
    StreamFrame {
        frame_type: FrameType::Final,
        seq,
        data,
    }
    .emit();
}

#[allow(dead_code)]
pub fn handle_stdin_command(registry: &SnapshotRegistry) {
    use std::io::BufRead;
    let stdin = std::io::stdin();
    let mut line = String::new();
    if stdin.lock().read_line(&mut line).unwrap_or(0) == 0 {
        return;
    }
    let cmd: CommandFrame = match serde_json::from_str(line.trim()) {
        Ok(c) => c,
        Err(e) => {
            eprintln!("ipc: failed to parse command: {e}");
            return;
        }
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
