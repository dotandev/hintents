// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Module cache and WASM custom section stripper for the simulator engine.
//! Strips non-essential custom sections (e.g., `name`, `.debug_*`, `producers`)
//! before module instantiation, reducing memory usage per contract instance (Issue #1977).

#![allow(dead_code)]

use std::collections::HashMap;
use std::sync::RwLock;
use thiserror::Error;
use wasmparser::Parser;

const WASM_MAGIC: &[u8; 4] = b"\0asm";
const WASM_VERSION: &[u8; 4] = &[1, 0, 0, 0];

#[derive(Debug, Error, PartialEq, Eq)]
pub enum WasmStripError {
    #[error("input is smaller than 8-byte WASM header")]
    TooShort,
    #[error("invalid WASM magic bytes")]
    InvalidMagic,
    #[error("unsupported WASM version")]
    UnsupportedVersion,
    #[error("unexpected end of file while parsing section")]
    UnexpectedEof,
    #[error("malformed LEB128 integer")]
    MalformedLeb128,
    #[error("malformed UTF-8 custom section name: {0}")]
    MalformedUtf8(String),
    #[error("stripped WASM failed validation: {0}")]
    ValidationFailed(String),
}

/// Helper to read an unsigned 32-bit LEB128 encoded integer.
fn read_u32_leb128(bytes: &[u8], offset: &mut usize) -> Result<u32, WasmStripError> {
    let mut result: u32 = 0;
    let mut shift = 0;
    loop {
        if *offset >= bytes.len() {
            return Err(WasmStripError::UnexpectedEof);
        }
        let byte = bytes[*offset];
        *offset += 1;
        result |= ((byte & 0x7f) as u32) << shift;
        if (byte & 0x80) == 0 {
            break;
        }
        shift += 7;
        if shift > 35 {
            return Err(WasmStripError::MalformedLeb128);
        }
    }
    Ok(result)
}

/// Check if a custom section is essential for Soroban contract execution.
/// Soroban metadata and specification sections (e.g. `contractmetav0`,
/// `contractenvmetav0`, `contractspecv0`) must be preserved.
#[must_use]
pub fn is_essential_custom_section(name: &str) -> bool {
    name.starts_with("contract")
}

/// Check if a custom section is non-essential and safe to strip before execution.
#[must_use]
pub fn is_non_essential_custom_section(name: &str) -> bool {
    name == "name"
        || name.starts_with(".debug")
        || name == "producers"
        || name == "target_features"
        || !is_essential_custom_section(name)
}

/// Strip non-essential custom sections from a WebAssembly module binary.
///
/// Standard sections (Types, Functions, Tables, Memory, Globals, Exports, Code, Data)
/// and essential Soroban metadata custom sections are preserved verbatim.
/// Bulky debug sections (`.debug_*`), symbol name sections (`name`), and build metadata
/// (`producers`, `target_features`) are removed to optimize simulator memory.
pub fn strip_non_essential_custom_sections(wasm: &[u8]) -> Result<Vec<u8>, WasmStripError> {
    if wasm.len() < 8 {
        return Err(WasmStripError::TooShort);
    }

    if &wasm[0..4] != WASM_MAGIC {
        return Err(WasmStripError::InvalidMagic);
    }

    if &wasm[4..8] != WASM_VERSION {
        return Err(WasmStripError::UnsupportedVersion);
    }

    let mut output = Vec::with_capacity(wasm.len());
    output.extend_from_slice(&wasm[0..8]);

    let mut offset = 8;
    while offset < wasm.len() {
        let section_start = offset;
        let section_id = wasm[offset];
        offset += 1;

        let payload_len = read_u32_leb128(wasm, &mut offset)? as usize;
        let payload_start = offset;

        if payload_start + payload_len > wasm.len() {
            return Err(WasmStripError::UnexpectedEof);
        }

        let section_end = payload_start + payload_len;

        if section_id != 0 {
            // Standard section (not custom) — preserve intact
            output.extend_from_slice(&wasm[section_start..section_end]);
        } else {
            // Custom section — parse section name
            let mut name_offset = payload_start;
            let name_len = read_u32_leb128(wasm, &mut name_offset)? as usize;

            if name_offset + name_len > section_end {
                return Err(WasmStripError::UnexpectedEof);
            }

            let name_bytes = &wasm[name_offset..name_offset + name_len];
            let name = std::str::from_utf8(name_bytes)
                .map_err(|e| WasmStripError::MalformedUtf8(e.to_string()))?;

            if is_essential_custom_section(name) {
                // Essential custom section — preserve
                output.extend_from_slice(&wasm[section_start..section_end]);
            }
            // Non-essential custom sections are omitted
        }

        offset = section_end;
    }

    // Validate the stripped WASM binary with wasmparser to ensure it is structurally valid
    for payload in Parser::new(0).parse_all(&output) {
        if let Err(e) = payload {
            return Err(WasmStripError::ValidationFailed(e.to_string()));
        }
    }

    Ok(output)
}

/// In-memory cache for parsed and stripped WASM contract modules.
/// Uses Blake3 hashing to cache stripped binaries, avoiding duplicate parsing and allocation.
#[derive(Default)]
pub struct ModuleCache {
    cache: RwLock<HashMap<[u8; 32], Vec<u8>>>,
}

impl ModuleCache {
    #[must_use]
    pub fn new() -> Self {
        Self {
            cache: RwLock::new(HashMap::new()),
        }
    }

    /// Retrieve the stripped WASM binary from cache or strip and cache it.
    pub fn get_or_strip(&self, wasm: &[u8]) -> Result<Vec<u8>, WasmStripError> {
        let hash = blake3::hash(wasm);
        let key = *hash.as_bytes();

        if let Ok(guard) = self.cache.read() {
            if let Some(cached) = guard.get(&key) {
                return Ok(cached.clone());
            }
        }

        let stripped = strip_non_essential_custom_sections(wasm)?;

        if let Ok(mut guard) = self.cache.write() {
            guard.insert(key, stripped.clone());
        }

        Ok(stripped)
    }

    /// Clear all cached modules.
    pub fn clear(&self) {
        if let Ok(mut guard) = self.cache.write() {
            guard.clear();
        }
    }

    /// Return the number of cached modules.
    pub fn len(&self) -> usize {
        self.cache.read().map(|g| g.len()).unwrap_or(0)
    }

    /// Check if the cache is empty.
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use wasmparser::Payload;

    // Helper to construct a minimal valid WASM module with a custom section
    fn make_wasm_with_custom_section(section_name: &str, content: &[u8]) -> Vec<u8> {
        let wat = r#"
            (module
                (func (export "test") (result i32)
                    i32.const 42
                )
            )
        "#;
        let mut wasm = wat::parse_str(wat).expect("valid WAT");

        // Append custom section (id = 0)
        let name_bytes = section_name.as_bytes();
        let mut payload = Vec::new();

        // Write name length as LEB128
        let mut n = name_bytes.len();
        while n >= 0x80 {
            payload.push(((n & 0x7f) as u8) | 0x80);
            n >>= 7;
        }
        payload.push(n as u8);
        payload.extend_from_slice(name_bytes);
        payload.extend_from_slice(content);

        // Section header
        wasm.push(0); // Section ID = 0 (custom)
        let mut plen = payload.len();
        while plen >= 0x80 {
            wasm.push(((plen & 0x7f) as u8) | 0x80);
            plen >>= 7;
        }
        wasm.push(plen as u8);
        wasm.extend_from_slice(&payload);

        wasm
    }

    #[test]
    fn test_strip_debug_and_name_sections() {
        let debug_content = vec![0xaa; 500]; // 500 bytes of dummy debug info
        let wasm_with_debug = make_wasm_with_custom_section(".debug_info", &debug_content);
        let wasm_with_name = make_wasm_with_custom_section("name", b"function_names_table");

        let stripped_debug = strip_non_essential_custom_sections(&wasm_with_debug).unwrap();
        assert!(
            stripped_debug.len() < wasm_with_debug.len(),
            "stripped WASM should be smaller than original"
        );
        assert_eq!(
            wasm_with_debug.len() - stripped_debug.len(),
            // 500 content + 11 name + 1 name_len + 1 sec_id + 2 sec_len = 515
            515
        );

        let stripped_name = strip_non_essential_custom_sections(&wasm_with_name).unwrap();
        assert!(stripped_name.len() < wasm_with_name.len());

        // Verify that stripped WASM does not contain .debug_info or name sections
        for payload in Parser::new(0).parse_all(&stripped_debug) {
            if let Ok(Payload::CustomSection(c)) = payload {
                assert_ne!(c.name(), ".debug_info");
            }
        }

        for payload in Parser::new(0).parse_all(&stripped_name) {
            if let Ok(Payload::CustomSection(c)) = payload {
                assert_ne!(c.name(), "name");
            }
        }
    }

    #[test]
    fn test_preserve_soroban_essential_metadata() {
        let metadata_content = b"soroban_contract_env_v0_spec_data";
        let wasm = make_wasm_with_custom_section("contractmetav0", metadata_content);

        let stripped = strip_non_essential_custom_sections(&wasm).unwrap();
        // Since contractmetav0 is essential, it should be preserved
        assert_eq!(stripped.len(), wasm.len());

        let mut found_metadata = false;
        for payload in Parser::new(0).parse_all(&stripped) {
            if let Ok(Payload::CustomSection(c)) = payload {
                if c.name() == "contractmetav0" {
                    found_metadata = true;
                    assert_eq!(c.data(), metadata_content);
                }
            }
        }
        assert!(found_metadata, "contractmetav0 section must be preserved");
    }

    #[test]
    fn test_invalid_wasm_headers() {
        // Too short
        assert_eq!(
            strip_non_essential_custom_sections(b"wasm"),
            Err(WasmStripError::TooShort)
        );

        // Invalid magic
        assert_eq!(
            strip_non_essential_custom_sections(b"not_wasm_binary!"),
            Err(WasmStripError::InvalidMagic)
        );

        // Unsupported version
        let bad_version = [0x00, 0x61, 0x73, 0x6d, 0x02, 0x00, 0x00, 0x00];
        assert_eq!(
            strip_non_essential_custom_sections(&bad_version),
            Err(WasmStripError::UnsupportedVersion)
        );
    }

    #[test]
    fn test_module_cache_hit_and_clear() {
        let cache = ModuleCache::new();
        assert!(cache.is_empty());

        let wasm = make_wasm_with_custom_section("name", b"test_function");
        let stripped1 = cache.get_or_strip(&wasm).unwrap();
        assert_eq!(cache.len(), 1);

        // Second call should return cached entry
        let stripped2 = cache.get_or_strip(&wasm).unwrap();
        assert_eq!(stripped1, stripped2);
        assert_eq!(cache.len(), 1);

        cache.clear();
        assert!(cache.is_empty());
    }
}
