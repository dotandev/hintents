// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use std::fs::File;
use std::io::{BufReader, Read};

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
    let file = File::open(path).map_err(WasmLoadError::Io)?;
    let mut reader = BufReader::new(file);

    // Read only the first 4 bytes to check magic bytes
    // This avoids buffering the entire file into memory upfront
    let mut magic = [0u8; 4];
    reader.read_exact(&mut magic).map_err(WasmLoadError::Io)?;

    if &magic != WASM_MAGIC {
        return Err(WasmLoadError::InvalidMagic);
    }

    // Magic bytes valid — now read the rest of the file
    let mut rest = Vec::new();
    reader.read_to_end(&mut rest).map_err(WasmLoadError::Io)?;

    // Combine magic + rest into full bytes
    let mut bytes = Vec::with_capacity(4 + rest.len());
    bytes.extend_from_slice(&magic);
    bytes.append(&mut rest);

    if bytes.len() > MAX_WASM_SIZE {
        return Err(WasmLoadError::TooLarge {
            size: bytes.len(),
            limit: MAX_WASM_SIZE,
        });
    }

    Ok(bytes)
}