// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Zstd decompression for ledger snapshot payloads sent over IPC.
//!
//! The Go side compresses `ledger_entries` with Zstd and base64-encodes the
//! result into `ledger_entries_zstd`.  This module decodes and decompresses
//! that field back into the plain `HashMap<String, String>` the simulator
//! expects.

use base64::Engine as _;
use std::collections::HashMap;

/// Decodes a base64-encoded Zstd blob produced by `bridge.CompressRequest`
/// and returns the original `ledger_entries` map.
pub fn decompress_ledger_entries(b64: &str) -> Result<HashMap<String, String>, String> {
    let compressed = base64::engine::general_purpose::STANDARD
        .decode(b64)
        .map_err(|e| format!("ipc/decompress: base64 decode: {e}"))?;

    let raw = zstd::decode_all(compressed.as_slice())
        .map_err(|e| format!("ipc/decompress: zstd decode: {e}"))?;

    serde_json::from_slice(&raw).map_err(|e| format!("ipc/decompress: json parse: {e}"))
}
