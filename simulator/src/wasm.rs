// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use std::fs;

const WASM_MAGIC: &[u8; 4] = b"\0asm";
pub const MAX_WASM_SIZE: usize = 64 * 1024; // 64 KiB Soroban Limit

#[derive(Debug)]
pub enum WasmLoadError {
    Io(std::io::Error),
    InvalidMagic,
    TooLarge { size: usize, limit: usize },
}

impl std::fmt::Display for WasmLoadError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            WasmLoadError::Io(e) => write!(f, "failed to read WASM file: {}", e),
            WasmLoadError::InvalidMagic => write!(f, "invalid WASM: missing magic bytes (\\0asm)"),
            WasmLoadError::TooLarge { size, limit } => {
                write!(f, "WASM too large: {} bytes (limit {})", size, limit)
            }
        }
    }
}

pub fn load_wasm_from_path(path: &str) -> Result<Vec<u8>, WasmLoadError> {
    let bytes = fs::read(path).map_err(WasmLoadError::Io)?;
    if bytes.len() < 4 || &bytes[..4] != WASM_MAGIC {
        return Err(WasmLoadError::InvalidMagic);
    }
    if bytes.len() > MAX_WASM_SIZE {
        return Err(WasmLoadError::TooLarge {
            size: bytes.len(),
            limit: MAX_WASM_SIZE,
        });
    }
    Ok(bytes)
}
