// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

pub mod decompress;
pub mod types;
pub mod validate;

#[allow(unused_imports)]
pub use types::{
    emit_chunk_frame, emit_chunk_raw, emit_debug_log, emit_error_log, emit_info_log,
    emit_trace_log, emit_warn_log, stream_to_stdout, IpcError, LogEntry, LogLevel, ResponseStreamer,
    SpanContext,
};

/// Default chunk target size (64 KiB) for streaming large simulation responses.
#[allow(dead_code)]
pub const DEFAULT_CHUNK_TARGET: usize = 64 * 1024;

/// Binds a TCP listener to `addr` and returns it.
///
/// If the socket cannot be established (e.g. the port is already in use or
/// the address is invalid) the function returns an `Err(IpcError::PortBindingFailed)`
/// with the original `std::io::Error` preserved as `source`, so the CLI can
/// inspect `ErrorKind` and report the failure with the appropriate exit code.
#[allow(dead_code)]
pub fn start_ipc_bridge<A: std::net::ToSocketAddrs>(
    addr: A,
) -> Result<std::net::TcpListener, IpcError> {
    std::net::TcpListener::bind(addr).map_err(|source| IpcError::PortBindingFailed { source })
}

#[cfg(test)]
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
            serde_json::from_str(r#"{"op":"FETCH_SNAPSHOT","id":3,"batch_size":5}"#).unwrap();
        assert_eq!(cmd.op, CommandOpcode::FetchSnapshot);
        assert_eq!(cmd.id, 3);
        assert_eq!(cmd.batch_size, 5);
    }

    #[test]
    fn test_command_frame_default_batch_size() {
        let cmd: CommandFrame = serde_json::from_str(r#"{"op":"FETCH_SNAPSHOT","id":7}"#).unwrap();
        assert_eq!(cmd.batch_size, 1);
    }

    #[test]
    fn test_parse_command_frame_invalid_json_returns_error() {
        let result = parse_command_frame("{invalid-json}");
        assert!(result.is_err());
    }

    #[test]
    fn test_start_ipc_bridge_success() {
        // port 0 lets the OS pick a free port — always succeeds
        let result = super::start_ipc_bridge("127.0.0.1:0");
        assert!(result.is_ok(), "expected successful bind: {:?}", result);
    }

    #[test]
    fn test_start_ipc_bridge_addr_in_use_returns_error() {
        // Bind two listeners to the same port to force EADDRINUSE
        let first = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = first.local_addr().unwrap().port();
        let addr = format!("127.0.0.1:{port}");
        let result = super::start_ipc_bridge(addr.as_str());
        assert!(
            result.is_err(),
            "expected Err when port is already bound, got Ok"
        );
        let err_msg = result.unwrap_err().to_string();
        assert!(
            err_msg.contains("IPC bridge could not bind"),
            "unexpected error message: {err_msg}"
        );
    }

    // ── Chunked streaming tests ──────────────────────────────────────

    #[test]
    fn test_chunk_frame_serialization() {
        let frame = StreamFrame {
            frame_type: FrameType::Chunk,
            seq: 0,
            total: Some(3),
            data: serde_json::json!({"partial": "data"}),
        };
        let json = serde_json::to_string(&frame).unwrap();
        assert!(json.contains(r#""type":"chunk""#));
        assert!(json.contains(r#""total":3"#));
        // total field must be present in the serialized output
        let decoded: StreamFrame = serde_json::from_str(&json).unwrap();
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
        // total must not appear in non-chunk frames
        assert!(!json.contains("total"), "total should be omitted: {json}");
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
        assert_eq!(total, 1, "expected exactly 1 chunk");

        let output = String::from_utf8(buf).unwrap();
        assert!(output.contains(r#""type":"chunk""#));
        assert!(output.contains(r#""seq":0"#));
        assert!(output.contains(r#""total":1"#));
        assert!(
            output.ends_with("}\n"),
            "output should end with newline: {output:?}"
        );

        // Data field should be a JSON string containing "{}"
        let parsed: StreamFrame = serde_json::from_str(output.trim()).unwrap();
        assert_eq!(parsed.data.as_str().unwrap(), "{}");
    }

    #[test]
    fn test_response_streamer_multi_chunk() {
        let mut buf = Vec::new();
        // Use a tiny chunk_target so each feed triggers a flush
        let mut streamer = ResponseStreamer::new(&mut buf, 3, 4);

        streamer.feed(br#"{"a":"#).unwrap(); // flushed immediately (5 bytes > 4)
        streamer.feed(br#""b"}"#).unwrap(); // flushed immediately (4 bytes >= 4)
        let total = streamer.finish().unwrap();
        assert_eq!(total, 2, "expected exactly 2 chunks");

        let output = String::from_utf8(buf).unwrap();
        let lines: Vec<&str> = output.lines().collect();
        assert_eq!(lines.len(), 2, "expected 2 NDJSON lines");

        // Each line is valid JSON. The data field is a JSON string.
        let mut reconstructed = String::new();
        for (i, line) in lines.iter().enumerate() {
            let parsed: StreamFrame = serde_json::from_str(line).unwrap();
            assert_eq!(parsed.frame_type, FrameType::Chunk);
            assert_eq!(parsed.seq, i as u32);
            assert_eq!(parsed.total, Some(3));
            // The data is a JSON string containing a raw JSON fragment
            let fragment = parsed
                .data
                .as_str()
                .unwrap_or_else(|| panic!("chunk {i} data is not a string"));
            reconstructed.push_str(fragment);
        }

        // The reconstructed string should be valid JSON
        let val: serde_json::Value =
            serde_json::from_str(&reconstructed).expect("reconstructed payload must be valid JSON");
        assert_eq!(val["a"], "b");
    }

    #[test]
    fn test_response_streamer_large_payload() {
        // Simulate a payload larger than default chunk target
        let mut buf = Vec::new();
        let mut streamer = ResponseStreamer::new(&mut buf, 2, 64);

        // Build a payload that exceeds chunk_target
        let chunk1 = vec![b'x'; 128];
        let chunk2 = vec![b'y'; 128];
        streamer.feed(&chunk1).unwrap();
        streamer.feed(&chunk2).unwrap();
        let total = streamer.finish().unwrap();
        assert!(total >= 2, "large payload should produce at least 2 chunks");

        let output = String::from_utf8(buf).unwrap();
        let lines: Vec<&str> = output.lines().collect();
        assert_eq!(lines.len() as u32, total);
    }

    #[test]
    fn test_emit_chunk_raw_equal_to_stream_frame() {
        // Verify the format produced by emit_chunk_raw matches what
        // a StreamFrame with FrameType::Chunk would produce.
        let mut raw_buf = Vec::new();
        let seq = 0u32;
        let total = 2u32;
        let data = br#"{"key":"value"}"#;

        // Write using same logic as emit_chunk_raw / flush_chunk
        write!(
            raw_buf,
            r#"{{"type":"chunk","seq":{seq},"total":{total},"data":""#
        )
        .unwrap();
        for &b in data {
            match b {
                b'"' => raw_buf.write_all(b"\\\"").unwrap(),
                b'\\' => raw_buf.write_all(b"\\\\").unwrap(),
                0x08 => raw_buf.write_all(b"\\b").unwrap(),
                0x0C => raw_buf.write_all(b"\\f").unwrap(),
                b'\n' => raw_buf.write_all(b"\\n").unwrap(),
                b'\r' => raw_buf.write_all(b"\\r").unwrap(),
                b'\t' => raw_buf.write_all(b"\\t").unwrap(),
                0x20..=0x7E => raw_buf.write_all(&[b]).unwrap(),
                _ => write!(raw_buf, "\\u{:04x}", b).unwrap(),
            }
        }
        writeln!(raw_buf, "\"}}").unwrap();

        let output = String::from_utf8(raw_buf).unwrap();
        let parsed: StreamFrame = serde_json::from_str(output.trim()).unwrap();
        assert_eq!(parsed.frame_type, FrameType::Chunk);
        assert_eq!(parsed.seq, 0);
        assert_eq!(parsed.total, Some(2));
        // Data should be the original raw bytes, decoded from JSON string
        assert_eq!(parsed.data.as_str().unwrap(), r#"{"key":"value"}"#);
    }

    // ── LogEntry tests ───────────────────────────────────────────────

    #[test]
    fn test_log_entry_creation() {
        let entry = LogEntry::new(LogLevel::Debug, "simulator::test", "Test message");
        assert_eq!(entry.level, LogLevel::Debug);
        assert_eq!(entry.target, "simulator::test");
        assert_eq!(entry.message, "Test message");
        assert!(entry.fields.is_none());
        assert!(entry.span.is_none());
        // Timestamp should be in RFC3339 format with nanoseconds
        assert!(entry.timestamp.contains('T'));
        assert!(entry.timestamp.contains('Z'));
    }

    #[test]
    fn test_log_entry_with_fields() {
        let entry = LogEntry::new(LogLevel::Info, "simulator::runner", "Contract invoked")
            .with_fields(serde_json::json!({
                "contract_id": "CAAAAAAA",
                "function": "increment",
                "ledger": 1234
            }));
        assert!(entry.fields.is_some());
        let fields = entry.fields.unwrap();
        assert_eq!(fields["contract_id"], "CAAAAAAA");
        assert_eq!(fields["function"], "increment");
        assert_eq!(fields["ledger"], 1234);
    }

    #[test]
    fn test_log_entry_with_span() {
        let span = SpanContext {
            name: "invoke_contract".to_string(),
            id: 42,
            parent_id: Some(1),
        };
        let entry =
            LogEntry::new(LogLevel::Trace, "simulator::vm", "Entering span").with_span(span);
        assert!(entry.span.is_some());
        let span_ctx = entry.span.unwrap();
        assert_eq!(span_ctx.name, "invoke_contract");
        assert_eq!(span_ctx.id, 42);
        assert_eq!(span_ctx.parent_id, Some(1));
    }

    #[test]
    fn test_log_entry_serialization() {
        let entry = LogEntry::new(LogLevel::Warn, "simulator::storage", "Cache miss")
            .with_fields(serde_json::json!({"key": "counter", "ttl": 3600}));
        let json = serde_json::to_string(&entry).unwrap();
        assert!(json.contains(r#""level":"warn""#));
        assert!(json.contains(r#""target":"simulator::storage""#));
        assert!(json.contains(r#""message":"Cache miss""#));
        assert!(json.contains(r#""key":"counter""#));
        assert!(json.contains(r#""ttl":3600"#));
    }

    #[test]
    fn test_log_entry_deserialization() {
        let json = r#"{
            "timestamp": "2026-08-26T15:30:45.123456789Z",
            "level": "error",
            "target": "simulator::host",
            "message": "Contract execution failed",
            "fields": {"error_code": 1, "reason": "insufficient balance"}
        }"#;
        let entry: LogEntry = serde_json::from_str(json).unwrap();
        assert_eq!(entry.level, LogLevel::Error);
        assert_eq!(entry.target, "simulator::host");
        assert_eq!(entry.message, "Contract execution failed");
        assert_eq!(
            entry.timestamp,
            "2026-08-26T15:30:45.123456789Z"
        );
        let fields = entry.fields.unwrap();
        assert_eq!(fields["error_code"], 1);
        assert_eq!(fields["reason"], "insufficient balance");
    }

    #[test]
    fn test_log_level_serialization() {
        assert_eq!(
            serde_json::to_string(&LogLevel::Trace).unwrap(),
            r#""trace""#
        );
        assert_eq!(
            serde_json::to_string(&LogLevel::Debug).unwrap(),
            r#""debug""#
        );
        assert_eq!(
            serde_json::to_string(&LogLevel::Info).unwrap(),
            r#""info""#
        );
        assert_eq!(
            serde_json::to_string(&LogLevel::Warn).unwrap(),
            r#""warn""#
        );
        assert_eq!(
            serde_json::to_string(&LogLevel::Error).unwrap(),
            r#""error""#
        );
    }

    #[test]
    fn test_log_frame_type_serialization() {
        assert_eq!(
            serde_json::to_string(&FrameType::Log).unwrap(),
            r#""log""#
        );
    }

    #[test]
    fn test_span_context_serialization() {
        let span = SpanContext {
            name: "storage_read".to_string(),
            id: 123,
            parent_id: None,
        };
        let json = serde_json::to_string(&span).unwrap();
        assert!(json.contains(r#""name":"storage_read""#));
        assert!(json.contains(r#""id":123"#));
        // parent_id should be omitted when None
        assert!(!json.contains("parent_id"));
    }

    #[test]
    fn test_span_context_with_parent() {
        let span = SpanContext {
            name: "nested_call".to_string(),
            id: 456,
            parent_id: Some(123),
        };
        let json = serde_json::to_string(&span).unwrap();
        assert!(json.contains(r#""parent_id":123"#));
    }

    #[test]
    fn test_log_entry_emit_does_not_panic() {
        // Test that emit works without panicking (output goes to stdout)
        let entry = LogEntry::new(LogLevel::Debug, "simulator::test", "Test emit");
        entry.emit(); // Should not panic
    }

    #[test]
    fn test_emit_trace_log_convenience_function() {
        emit_trace_log("simulator::test", "Trace message", None);
        // Should not panic
    }

    #[test]
    fn test_emit_debug_log_convenience_function() {
        emit_debug_log(
            "simulator::test",
            "Debug message",
            Some(serde_json::json!({"key": "value"})),
        );
        // Should not panic
    }

    #[test]
    fn test_emit_info_log_convenience_function() {
        emit_info_log("simulator::test", "Info message", None);
        // Should not panic
    }

    #[test]
    fn test_emit_warn_log_convenience_function() {
        emit_warn_log(
            "simulator::test",
            "Warning message",
            Some(serde_json::json!({"severity": "medium"})),
        );
        // Should not panic
    }

    #[test]
    fn test_emit_error_log_convenience_function() {
        emit_error_log(
            "simulator::test",
            "Error occurred",
            Some(serde_json::json!({"code": 500})),
        );
        // Should not panic
    }

    #[test]
    fn test_log_entry_schema_compliance() {
        // Verify the LogEntry serialization produces the expected JSON schema
        let entry = LogEntry::new(LogLevel::Debug, "simulator::runner", "Starting contract")
            .with_fields(serde_json::json!({
                "contract_id": "CTEST",
                "ledger": 5000
            }))
            .with_span(SpanContext {
                name: "invoke_contract".to_string(),
                id: 99,
                parent_id: Some(10),
            });

        let json = serde_json::to_value(&entry).unwrap();

        // Verify all required fields are present
        assert!(json.get("timestamp").is_some());
        assert!(json.get("level").is_some());
        assert!(json.get("target").is_some());
        assert!(json.get("message").is_some());

        // Verify optional fields are present when set
        assert!(json.get("fields").is_some());
        assert!(json.get("span").is_some());

        // Verify span structure
        let span = &json["span"];
        assert!(span.get("name").is_some());
        assert!(span.get("id").is_some());
        assert!(span.get("parent_id").is_some());
    }

    #[test]
    fn test_log_entry_minimal_schema() {
        // Verify minimal LogEntry without optional fields
        let entry = LogEntry::new(LogLevel::Info, "simulator::minimal", "Minimal log");
        let json = serde_json::to_value(&entry).unwrap();

        // Required fields should be present
        assert!(json.get("timestamp").is_some());
        assert!(json.get("level").is_some());
        assert!(json.get("target").is_some());
        assert!(json.get("message").is_some());

        // Optional fields should be absent
        assert!(json.get("fields").is_none());
        assert!(json.get("span").is_none());
    }
}
