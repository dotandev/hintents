// Copyright 2025 Hintents Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use anyhow::{anyhow, Result};
use sha2::{Digest, Sha256};
use wasmparser::{Parser, Payload};

/// Custom section names that are not required for execution and can be
/// stripped before instantiation to reduce per-instance memory usage.
const STRIPPABLE_CUSTOM_SECTIONS: &[&str] = &[
    "name",
    ".debug_info",
    ".debug_abbrev",
    ".debug_line",
    ".debug_str",
    ".debug_ranges",
    ".debug_loc",
    ".debug_aranges",
    ".debug_pubnames",
    ".debug_pubtypes",
];

/// Returns a copy of the WASM binary with non-essential custom sections removed.
///
/// Every top-level section is re-emitted byte-for-byte except custom sections
/// whose names appear in `STRIPPABLE_CUSTOM_SECTIONS`. The output is a valid
/// WASM module that can be handed directly to the execution engine.
pub fn strip_custom_sections(wasm: &[u8]) -> Result<Vec<u8>> {
    // Validate header: magic + version (8 bytes).
    if wasm.len() < 8 || &wasm[0..4] != b"\0asm" {
        return Err(anyhow!("invalid WASM magic header"));
    }

    let mut out = Vec::with_capacity(wasm.len());
    out.extend_from_slice(&wasm[0..8]);

    let mut last_end: usize = 8;

    for payload in Parser::new(0).parse_all(wasm) {
        match payload? {
            Payload::Version { .. } => {}
            Payload::CustomSection(reader) => {
                // `range()` covers the section payload; the section end is the
                // exclusive end of that range.
                let full_end = reader.range().end;
                let name = reader.name();
                if !STRIPPABLE_CUSTOM_SECTIONS.contains(&name) {
                    out.extend_from_slice(&wasm[last_end..full_end]);
                }
                last_end = full_end;
            }
            Payload::End(offset) => {
                out.extend_from_slice(&wasm[last_end..offset]);
                last_end = offset;
            }
            _ => {}
        }
    }

    if last_end < wasm.len() {
        out.extend_from_slice(&wasm[last_end..]);
    }
    Ok(out)
}

/// Cache of prepared (stripped) WASM modules keyed by content hash.
#[derive(Default)]
pub struct ModuleCache {
    entries: Mutex<HashMap<[u8; 32], Arc<Vec<u8>>>>,
}

impl ModuleCache {
    pub fn new() -> Self {
        Self::default()
    }

    /// Returns the stripped module bytes for `wasm`, computing and caching them
    /// on first use.
    pub fn get_or_prepare(&self, wasm: &[u8]) -> Result<Arc<Vec<u8>>> {
        let key: [u8; 32] = Sha256::digest(wasm).into();
        let mut entries = self
            .entries
            .lock()
            .map_err(|_| anyhow!("module cache poisoned"))?;
        if let Some(existing) = entries.get(&key) {
            return Ok(Arc::clone(existing));
        }
        let stripped = Arc::new(strip_custom_sections(wasm)?);
        entries.insert(key, Arc::clone(&stripped));
        Ok(stripped)
    }

    pub fn len(&self) -> usize {
        self.entries.lock().map(|e| e.len()).unwrap_or(0)
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn module_with_custom(name: &str) -> Vec<u8> {
        let mut m = b"\0asm\x01\0\0\0".to_vec();
        // custom section: id 0, payload = [name_len, name bytes, data 0xAA]
        let mut payload = vec![name.len() as u8];
        payload.extend_from_slice(name.as_bytes());
        payload.push(0xAA);
        m.push(0);
        m.push(payload.len() as u8);
        m.extend_from_slice(&payload);
        m
    }

    #[test]
    fn strips_debug_sections() {
        let wasm = module_with_custom(".debug_info");
        let stripped = strip_custom_sections(&wasm).unwrap();
        assert_eq!(stripped, b"\0asm\x01\0\0\0".to_vec());
    }

    #[test]
    fn keeps_essential_custom_sections() {
        let wasm = module_with_custom("producers");
        let stripped = strip_custom_sections(&wasm).unwrap();
        assert_eq!(stripped, wasm);
    }

    #[test]
    fn rejects_bad_magic() {
        assert!(strip_custom_sections(b"nope1234").is_err());
    }

    #[test]
    fn cache_reuses_entries() {
        let cache = ModuleCache::new();
        let wasm = module_with_custom("name");
        let a = cache.get_or_prepare(&wasm).unwrap();
        let b = cache.get_or_prepare(&wasm).unwrap();
        assert!(Arc::ptr_eq(&a, &b));
        assert_eq!(cache.len(), 1);
    }
}
