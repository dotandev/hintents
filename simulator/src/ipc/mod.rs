// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

pub mod decompress;
pub mod types;
pub mod validate;

pub use types::IpcError;

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
}
