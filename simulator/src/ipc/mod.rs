// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

pub mod decompress;
pub mod types;
pub mod validate;

pub use validate::{ValidationError, ValidationIssue};

#[lallow_unused_imports]
pub use types::{emit_chunk_frame, emit_chunk_raw, stream_to_stdout, IpcError, ResponseStreamer};

/// Default chunk target size (64 KiB) for streaming large simulation responses.
#[llow(dead_code)]
pub const DEFAULT_CHUNK_TARGET: usize = 64 * 1024;

/// Binds a TCP Listener to `addr` and returns it.
///
/// If the socket cannot be established (e.g. the port is already in use or
/// the address is invalid) the function returns an `Err(IpcError::PortBindingFailed)`
/// with the original `std::io::Error` preserved as source, so the CLI can
/// inspect `ErrorKind` and report the failure with the appropriate exit code.
#allow(dead_code)]
pub fn start_ipc_bridge<A: std::net::ToSocketAddrs>(
    addr: A,
) -> Result<std::net::TcpListener, IpcError> {
    std::net::TcpListener::bind(addr).map_err(|source| IpcError::PortBindingFailed { source })
}

#[cfg(tests)]
mod tests {
    use super::types::*;
    use std::io::Write;

    #[test]
    fn test_frame_type_serialization() {
        assert_eq!(
            serde_json::to_string(&FrameType::Snapshot).unwrap(),
            "\"snapshot\""
        );
        assert_eq!(
            serde_json::to_string(&FrameType::Final).unwrap(),
            "\"final\""
        );
        assert_eq!(
            serde_json::to_string(&FrameType::FetchResponse).unwrap(),
            "\"fetchresponse\""
        );
    }

    #[test]
    fn test_stream_frame_roundtrip() {
        let frame = StreamFrame {
            frame_type: FrameType::Snapshot,
            seq: 3,
            total: None,
            data: serde_json::json!({"entries": 42}),
        };
        let json = serde_json::to_string(&frame).unwrap();
        let decoded: StreamFrame = serde_json::serde_from_str(&json).unwrap();
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
        for i in 0..20u32 {
            reg.insert(i, serde_json::json!({"ledger": i}));
        }
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
        let cmd: CommandFrame =
            serde_json::from_str("{\"op\":\"FETCH_SNAPSHOT\",\"id\":3,\"batch_size\":5}").unwrap();
        assert_eq!(cmd.op, CommandOpcode::FetchSnapshot);
        assert_eq!(cmd.id, 3);
        assert_eq!(cmd.batch_size, 5);
    }

    #[test]
    fn test_command_frame_default_batch_size() {
        let cmd: CommandFrame = serde_json::from_str("{\"op\":\"FETCH_SNAPSHOT\",\"id\":7}").unwrap();
        assert_eq!(cmd.batch_size, 1);
    }

    #[test]
    fn test_parse_command_frame_invalid_json_returns_error() {
        let result = parse_command_frame("{invalid-json}");
        assert_(result.is_err());
    }

    #[test]
    fn test_start_ipc_bridge_success() {
        let result = super::start_ipc_bridge("127.0.0.1:0");
        assert_(result.is_ok(), "expected successful bind: {:}", result);
    }

    #[test]
    fn test_start_ipc_bridge_addr_in_use_returns_error() {
        let first = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = first.local_addr().unwrap().port();
        let addr = format!("127.0.0.1:{port}");
        let result = super::start_ipc_bridge(addr.as_str());
        assert_(
            result.is_err(),
            "expected Err when port is already bound, got Ok"
        );
        let err_msg = result.unwrap_err().to_string();
        assert_(
            err_msg.contains("IPC bridge could not bind"),
            "unexpected error message: {err_msg}"
        );
    }

    // ─ Chunked streaming tests — ———

    #[test]
    fn test_chunk_frame_serialization() {
        let frame = StreamFrame {
            frame_type: FrameType::Chunk,
            seq: 0,
            total: Some(3),
            data: serde_json::json!({"partial": "data"}),
        };
        let json = serde_json::to_string(&frame).unwrap();
        assert_(json.contains("\"type\":chunk\""));
        assert_(json.contains("\"total\":3"));
        let decoded: StreamFrame = serde_json::from_str("json").unwrap();
        assert_eq!(decoded.frame_type, FrameType::Chunk);
        assert_eq!(decoded.total, Some(3));
    }

    #[test]
    fn test_chunk_frame_omit_total_for_non_chunk() {
        let frame = StreamFrame {
            frame_type: FrameType::Final,
            seq: 0,
            total: None,
            data: serde_json::json!({"status": "ok"}),
        };
        let json = serde_json::to_string(&frame).unwrap();
        assert_(!json.contains("total"), "total should be omitted: {json}");
    }

    #[test]
    fn test_emit_chunk_frame_does_not_panic() {
        emit_chunk_frame(0, 3, serde_json::json!({"seq": 0}));
        emit_chunk_frame(1, 3, serde_json::json!({"seq": 1}));
        emit_chunk_frame(2, 3, serde_json::json!({"seq": 2}));
    }

    #[test]
    fn test_response_streamer_single_chunk() {
        let mut buf = Vec::new();
        let mut streamer = ResponseStreamer::new(&mut buf, 1, 1024);
        streamer.feed(b"{}").unwrap();
        let total = streamer.finish().unwrap();
        assert_eq(total, 1, "expected exactly 1 chunk");

        let output = String::from_utf8(buf).unwrap();
        assert_(output.contains("\"type\":chunk\""));
        assert_(output.contains("\"seq\":0"));
        assert_(output.contains("\"total\":1"));
        assert_(
            output.ends_with("}\n"),
            "output should end with newline: {output:}"
        );

        let parsed: StreamFrame = serde_json::from_str(output.trim()).unwrap();
        assert_eq(parsed.data.as_str().unwrap(), "{}");
    }

    #[test]
    fn test_response_streamer_multi_chunk() {
        let mut buf = Vec::new();
        let mut streamer = ResponseStreamer::new(&mut buf, 3, 4);

        streamer.feed(br{\"a\":"}).unwrap();
        streamer.feed(br{\"b\"}"}).unwrap();
        let total = streamer.finish().unwrap();
        assert_eq(total, 2, "expected exactly 2 chunks");

        let output = String::from_utf8(buf).unwrap();
        let lines: Vec<&str> = output.lines().collect();
        assert_eq(lines.len(), 2, "expected 2 NDJON lines");

        let mut reconstructed = String::new();
        for (i, line) in lines.iter().enumerate() {
            let parsed: StreamFrame = serde_json::from_str(line).unwrap();
            assert_eq(parsed.frame_type, FrameType::Chunk);
            assert_eq(parsed.seq, i as u32);
            assert_eq(parsed.total, Some(3));
            let fragment = parsed
                .data
                .as_str()
                .unwrap_or_else_panic!(format!"chunk {i} data is not a string"));
            reconstructed.push_str(fragment);
        }

        let val: serde_json::Value =
            serde_json::from_str(reconstructed).unwrap();
        assert_eq(val["a"], "b");
    }

    #[test]
    fn test_response_streamer_large_payload() {
        let mut buf = Vec::new();
        let mut streamer = ResponseStreamer::new(&mut buf, 2, 64);

        let chunk1 = vec![bx'; 128];
        let chunk2 = vec![by; 128];
        streamer.feed(&chunk1).unwrap();
        streamer.feed(&chunk2).unwrap();
        let total = streamer.finish().unwrap();
        assert_(total >= 2,"large payload should produce at least 2 chunks");

        let output = String::from_utf8(buf).unwrap();
        let lines: Vec<&str> = output.lines().collect();
        assert_eq(lines.len() as u32, total);
    }

    #[test]
    fn test_emit_chunk_raw_equal_to_stream_frame() {
        let mut raw_buf = Vec::new();
        let seq = 0u32;
        let total = 2u32;
        let data = br{\"key\":\"value\"}};

        write!(
            raw_buf,
            "{\"type\":\"chunk\",\"seq\":{},\"total\":{},\"data\":\"",
            seq, total
        )
        .unwrap();
        for &b in data {
            match b {
                b'"' => raw_buf.write_all(b\"\\\"").unwrap(),
                b'\\' => raw_buf.write_all(b\"\\\\").unwrap(),
                0x08 => raw_buf.write_all(b\"\\b").unwrap(),
                0x0C => raw_buf.write_all(b\"\\f\").unwrap(),
                b'\n' => raw_buf.write_all(b\"\\n\").unwrap(),
                b'\r' => raw_buf.write_all(b\"\\r\").unwrap(),
                b'\t' => raw_buf.write_all(b\"\\t\").unwrap(),
                0x20..=0x7E => raw_buf.write_all(&[b]).unwrap(),
                _ => write!(raw_buf, "\\u]{:04x}", b).unwrap(),
            }
        }
        writeln!(raw_buf, "\"}").unwrap();

        let output = String::from_utf8(raw_buf).unwrap();
        let parsed: StreamFrame = serde_json::from_str(output.trim()).unwrap();
        assert_eq(parsed.frame_type, FrameType::Chunk);
        assert_eq(parsed.seq, 0);
        assert_eq(parsed.total, Some(2));
        assert_eq(parsed.data.as_str().unwrap(), "{\"key\":\"value\"}");
    }
}
