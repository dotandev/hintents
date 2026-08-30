// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::io::Write;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum IpcError {
    #[error("IPC IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("IPC JSON error: {0}")]
    Json(#[from] serde_json::Error),

    /// Returned when `start_ipc_bridge` cannot bind to the requested address
    /// (e.g. port already in use, permission denied). The underlying
    /// `std::io::Error` is preserved as the error source so callers can
    /// inspect `ErrorKind` (e.g. `AddrInUse`, `PermissionDenied`) and map
    /// it to the appropriate CLI exit code.
    #[error("IPC bridge could not bind: {source}")]
    PortBindingFailed {
        #[source]
        source: std::io::Error,
    },

    #[error("IPC decompress error: {0}")]
    Decompress(String),
}

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
    /// Partial payload chunk within a multi-frame large response.
    /// The consumer concatenates all Chunk frames in seq order to
    /// reconstruct the full JSON payload.
    Chunk,
}

/// A single newline-delimited JSON (NDJSON) frame written to stdout.
#[allow(dead_code)]
#[derive(Debug, Serialize, Deserialize)]
pub struct StreamFrame {
    #[serde(rename = "type")]
    pub frame_type: FrameType,
    pub seq: u32,
    /// Total number of frames in this logical batch (e.g. total chunks).
    /// Only set on chunk frames; omitted for snapshot/final frames.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub total: Option<u32>,
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

/// Streaming response writer that emits large simulation payloads as
/// chunked NDJSON frames without buffering the entire response in memory.
///
/// Each chunk's payload is written as a JSON-escaped string in the `data`
/// field so that every individual chunk frame is valid JSON. The consumer
/// JSON-parses the data field and concatenates the decoded strings in `seq`
/// order to reconstruct the full response payload.
///
/// # Example
///
/// ```ignore
/// let mut streamer = ResponseStreamer::new_stdout(3, 64 * 1024);
/// streamer.feed(br#"{"status":"success","events":"#)?;
/// streamer.feed(br#"[{"id":1},{"id":2}]"#)?;
/// streamer.feed(b"}")?;
/// let total = streamer.finish()?;
/// ```
#[allow(dead_code)]
pub struct ResponseStreamer<W: Write> {
    writer: W,
    seq: u32,
    total_chunks: u32,
    buffer: Vec<u8>,
    chunk_target: usize,
}

#[allow(dead_code)]
impl<W: Write> ResponseStreamer<W> {
    /// Create a new streamer that writes chunked NDJSON frames to `writer`.
    ///
    /// `total_chunks` is the expected number of chunks (known upfront or
    /// estimated). `chunk_target` is the approximate byte size at which a
    /// buffered chunk is flushed as a separate NDJSON frame.
    pub fn new(writer: W, total_chunks: u32, chunk_target: usize) -> Self {
        ResponseStreamer {
            writer,
            seq: 0,
            total_chunks,
            buffer: Vec::with_capacity(chunk_target),
            chunk_target,
        }
    }

    /// Feed raw JSON bytes. When the internal buffer exceeds `chunk_target`,
    /// it is flushed as a chunk frame to the underlying writer.
    pub fn feed(&mut self, bytes: &[u8]) -> Result<(), IpcError> {
        self.buffer.extend_from_slice(bytes);
        if self.buffer.len() >= self.chunk_target {
            self.flush_chunk()?;
        }
        Ok(())
    }

    /// Flush any remaining buffered data as the final chunk and return the
    /// total number of chunks emitted.
    pub fn finish(&mut self) -> Result<u32, IpcError> {
        if !self.buffer.is_empty() {
            self.flush_chunk()?;
        }
        self.writer.flush()?;
        Ok(self.seq)
    }

    fn flush_chunk(&mut self) -> Result<(), IpcError> {
        let data_bytes = core::mem::take(&mut self.buffer);
        let seq = self.seq;
        let total = self.total_chunks;
        write!(
            self.writer,
            r#"{{"type":"chunk","seq":{seq},"total":{total},"data":""#
        )?;
        for &b in &data_bytes {
            match b {
                b'"' => self.writer.write_all(b"\\\"")?,
                b'\\' => self.writer.write_all(b"\\\\")?,
                0x08 => self.writer.write_all(b"\\b")?,
                0x0C => self.writer.write_all(b"\\f")?,
                b'\n' => self.writer.write_all(b"\\n")?,
                b'\r' => self.writer.write_all(b"\\r")?,
                b'\t' => self.writer.write_all(b"\\t")?,
                0x20..=0x7E => self.writer.write_all(&[b])?,
                _ => write!(self.writer, "\\u{:04x}", b)?,
            }
        }
        self.writer.write_all(b"\"}\n")?;
        self.seq += 1;
        Ok(())
    }
}

/// Create a `ResponseStreamer` wrapping `std::io::stdout().lock()`.
#[allow(dead_code)]
pub fn stream_to_stdout(
    total_chunks: u32,
    chunk_target: usize,
) -> ResponseStreamer<std::io::StdoutLock<'static>> {
    ResponseStreamer::new(std::io::stdout().lock(), total_chunks, chunk_target)
}

/// Emit a single chunk frame with raw bytes written as a JSON-escaped
/// string in the `data` field. Prefer `ResponseStreamer` for multi-chunk
/// payloads.
#[allow(dead_code)]
pub fn emit_chunk_raw(seq: u32, total: u32, data: &[u8]) {
    let stdout = std::io::stdout();
    let mut handle = stdout.lock();
    let res = write!(
        handle,
        r#"{{"type":"chunk","seq":{seq},"total":{total},"data":""#
    );
    if res.is_ok() {
        for &b in data {
            let r = match b {
                b'"' => handle.write_all(b"\\\""),
                b'\\' => handle.write_all(b"\\\\"),
                0x08 => handle.write_all(b"\\b"),
                0x0C => handle.write_all(b"\\f"),
                b'\n' => handle.write_all(b"\\n"),
                b'\r' => handle.write_all(b"\\r"),
                b'\t' => handle.write_all(b"\\t"),
                0x20..=0x7E => handle.write_all(&[b]),
                _ => write!(handle, "\\u{:04x}", b),
            };
            if r.is_err() {
                eprintln!("bridge: failed to emit chunk frame (seq={seq})");
                return;
            }
        }
        if writeln!(handle, "\"}}").is_err() {
            eprintln!("bridge: failed to emit chunk frame (seq={seq})");
        }
    } else {
        eprintln!("bridge: failed to emit chunk frame (seq={seq})");
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

#[allow(dead_code)]
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
    #[serde(skip_serializing_if = "Option::is_none")]
    total: Option<u32>,
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
        total: None,
        data,
    }
    .emit();
}

#[allow(dead_code)]
pub fn emit_final_frame(seq: u32, data: serde_json::Value) {
    StreamFrame {
        frame_type: FrameType::Final,
        seq,
        total: None,
        data,
    }
    .emit();
}

/// Emit a single chunk frame with `serde_json::Value` data.
#[allow(dead_code)]
pub fn emit_chunk_frame(seq: u32, total: u32, data: serde_json::Value) {
    StreamFrame {
        frame_type: FrameType::Chunk,
        seq,
        total: Some(total),
        data,
    }
    .emit();
}

#[allow(dead_code)]
pub fn parse_command_frame(input: &str) -> Result<CommandFrame, IpcError> {
    let cmd: CommandFrame = serde_json::from_str(input)?;
    Ok(cmd)
}

#[allow(dead_code)]
pub fn handle_stdin_command(registry: &SnapshotRegistry) -> Result<(), IpcError> {
    use std::io::BufRead;
    let stdin = std::io::stdin();
    let mut line = String::new();
    if stdin.lock().read_line(&mut line)? == 0 {
        return Ok(());
    }
    let cmd = parse_command_frame(line.trim())?;
    match cmd.op {
        CommandOpcode::FetchSnapshot => {
            let snapshots = registry.fetch(cmd.id, cmd.batch_size);
            let response = FetchResponseFrame {
                frame_type: FrameType::FetchResponse,
                seq: cmd.id,
                total: None,
                data: FetchResponseData { snapshots },
            };
            let json_line = serde_json::to_string(&response)?;
            let stdout = std::io::stdout();
            let mut handle = stdout.lock();
            writeln!(handle, "{json_line}")?;
        }
    }
    Ok(())
}
