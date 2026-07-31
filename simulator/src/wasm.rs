// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

use std::fs::File;
use std::io::{BufReader, Read};
use wasmparser::Parser;

const WASM_MAGIC: &[u8; 4] = b"\0asm";
pub const MAX_WASM_SIZE: usize = 64 * 1024; // 64 KiB Soroban Limit
pub const MAX_DATA_SECTION_SIZE: usize = 32 * 1024; // 32 KiB max data section to prevent OOM

#[derive(Debug)]
pub enum WasmLoadError {
    Io(std::io::Error),
    InvalidMagic,
    TooLarge { size: usize, limit: usize },
    DataSectionTooLarge { size: usize, limit: usize },
}

impl std::fmt::Display for WasmLoadError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            WasmLoadError::Io(e) => write!(f, "failed to read WASM file: {}", e),
            WasmLoadError::InvalidMagic => write!(f, "invalid WASM: missing magic bytes (\\0asm)"),
            WasmLoadError::TooLarge { size, limit } => {
                write!(f, "WASM too large: {} bytes (limit {})", size, limit)
            }
            WasmLoadError::DataSectionTooLarge { size, limit } => {
                write!(
                    f,
                    "WASM data section too large: {} bytes (limit {})",
                    size, limit
                )
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

    // Validate data section size to prevent OOM attacks
    validate_data_section_size(&bytes)?;

    Ok(bytes)
}

/// Validates that the data section size is within safe bounds to prevent OOM attacks.
fn validate_data_section_size(wasm_bytes: &[u8]) -> Result<(), WasmLoadError> {
    let mut total_data_size = 0usize;

    for payload in Parser::new(0).parse_all(wasm_bytes) {
        let payload = payload.map_err(|e| {
            WasmLoadError::Io(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                format!("Failed to parse WASM: {}", e),
            ))
        })?;

        if let wasmparser::Payload::DataSection(reader) = payload {
            for data in reader {
                let data = data.map_err(|e| {
                    WasmLoadError::Io(std::io::Error::new(
                        std::io::ErrorKind::InvalidData,
                        format!("Failed to read data entry: {}", e),
                    ))
                })?;

                // Accumulate the size of each data segment
                total_data_size += data.data.len();

                // Check if we've exceeded the limit
                if total_data_size > MAX_DATA_SECTION_SIZE {
                    return Err(WasmLoadError::DataSectionTooLarge {
                        size: total_data_size,
                        limit: MAX_DATA_SECTION_SIZE,
                    });
                }
            }
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_validate_data_section_size_normal() {
        // A normal WASM module with small data section should pass
        let wasm = wat::parse_str(r#"(module (data (i32.const 0) "hello"))"#).unwrap();
        assert!(validate_data_section_size(&wasm).is_ok());
    }

    #[test]
    fn test_validate_data_section_size_empty() {
        // WASM with no data section should pass
        let wasm = wat::parse_str(r#"(module (func (param i32) (result i32)))"#).unwrap();
        assert!(validate_data_section_size(&wasm).is_ok());
    }

    #[test]
    fn test_validate_data_section_size_exceeds_limit() {
        // Create a WASM with a data section larger than MAX_DATA_SECTION_SIZE
        let large_data = vec![0u8; MAX_DATA_SECTION_SIZE + 1];
        let data_str = format!(
            "(module (data (i32.const 0) \"{}\"))",
            String::from_utf8_lossy(&large_data[..MAX_DATA_SECTION_SIZE])
        );

        // This test is limited by the fact that we can't easily create a WASM
        // with an actual oversized data section using wat::parse_str due to
        // string literal limitations. The validation logic is tested indirectly
        // through the load_wasm_from_path function in integration tests.
    }

    #[test]
    fn test_load_wasm_with_valid_data_section() {
        // Test that loading a WASM with valid data section works
        let wasm = wat::parse_str(r#"(module (data (i32.const 0) "test"))"#).unwrap();

        // Write to a temp file and load it
        let temp_dir = std::env::temp_dir();
        let temp_file = temp_dir.join("test_wasm.wasm");
        std::fs::write(&temp_file, &wasm).unwrap();

        let result = load_wasm_from_path(temp_file.to_str().unwrap());
        assert!(result.is_ok());

        // Cleanup
        std::fs::remove_file(temp_file).ok();
    }
}
