// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

pub mod decompress;
pub mod validate;

use serde::{Deserialize, Serialize};
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
}

/// A single newline-delimited JSON (NDJSON) frame written to stdout.
///
/// The Go bridge reads these lines from the simulator subprocess stdout pipe
/// in a background goroutine, enabling the UI to populate frames before the
/// simulation finishes (reducing Time-to-First-Interactive).
#[allow(dead_code)]
#[derive(Debug, Serialize, Deserialize)]
pub struct StreamFrame {
    /// Discriminates snapshot frames from the terminal final frame.
    #[serde(rename = "type")]
    pub frame_type: FrameType,
    /// Monotonically increasing sequence number (0-based) within one run.
    pub seq: u32,
    /// Arbitrary JSON payload.
    ///  - `FrameType::Snapshot`: ledger snapshot data captured mid-simulation.
    ///  - `FrameType::Final`:    the complete `SimulationResponse` object.
    pub data: serde_json::Value,
}

impl StreamFrame {
    /// Serialise this frame to a single JSON line on stdout.
    ///
    /// Uses a locked stdout handle to prevent interleaved output when called
    /// from concurrent contexts. Write errors are logged to stderr and ignored
    /// so that simulation output is not disrupted by a broken pipe — the Go
    /// side will detect the closed reader and surface the error via `Wait()`.
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

/// Emit an intermediate snapshot frame for ledger state captured mid-simulation.
///
/// # Arguments
/// * `seq`  – Monotonically increasing sequence number for this run (start at 0).
/// * `data` – Ledger snapshot data to forward to the Go bridge.
#[allow(dead_code)]
pub fn emit_snapshot_frame(seq: u32, data: serde_json::Value) {
    StreamFrame {
        frame_type: FrameType::Snapshot,
        seq,
        data,
    }
    .emit();
}

/// Emit the terminal frame, signalling to the Go bridge that the simulation
/// has completed and no further frames will follow.
///
/// # Arguments
/// * `seq`  – Sequence number immediately following the last snapshot frame.
/// * `data` – The complete `SimulationResponse` as a `serde_json::Value`.
#[allow(dead_code)]
pub fn emit_final_frame(seq: u32, data: serde_json::Value) {
    StreamFrame {
        frame_type: FrameType::Final,
        seq,
        data,
    }
    .emit();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_frame_type_serialization() {
        let snapshot = serde_json::to_string(&FrameType::Snapshot).unwrap();
        assert_eq!(snapshot, "\"snapshot\"");

        let final_type = serde_json::to_string(&FrameType::Final).unwrap();
        assert_eq!(final_type, "\"final\"");
    }

    #[test]
    fn test_stream_frame_roundtrip() {
        let frame = StreamFrame {
            frame_type: FrameType::Snapshot,
            seq: 3,
            data: serde_json::json!({"entries": 42}),
        };

        let json = serde_json::to_string(&frame).unwrap();
        let decoded: StreamFrame = serde_json::from_str(&json).unwrap();

        assert_eq!(decoded.frame_type, FrameType::Snapshot);
        assert_eq!(decoded.seq, 3);
        assert_eq!(decoded.data["entries"], 42);
    }

    #[test]
    fn test_final_frame_roundtrip() {
        let frame = StreamFrame {
            frame_type: FrameType::Final,
            seq: 5,
            data: serde_json::json!({"status": "success", "events": []}),
        };

        let json = serde_json::to_string(&frame).unwrap();
        assert!(json.contains("\"type\":\"final\""));
        assert!(json.contains("\"seq\":5"));

        let decoded: StreamFrame = serde_json::from_str(&json).unwrap();
        assert_eq!(decoded.frame_type, FrameType::Final);
        assert_eq!(decoded.data["status"], "success");
    }

    #[test]
    fn test_emit_snapshot_frame_does_not_panic() {
        // Only verifies that the helper compiles and runs without panicking.
        // Stdout capture in unit tests is non-trivial; the integration tests
        // validate the actual output format end-to-end.
        emit_snapshot_frame(0, serde_json::json!({"test": true}));
    }
}
