// Copyright 2024 Hintents Contributors
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

use object::Object;
use serde::Serialize;

pub struct SourceMapper {
    has_symbols: bool,
}

#[derive(Debug, Clone, Serialize)]
pub struct SourceLocation {
    pub file: String,
    pub line: u32,
    pub column: Option<u32>,
}

impl SourceMapper {
    pub fn new(wasm_bytes: Vec<u8>) -> Self {
        let has_symbols = Self::check_debug_symbols(&wasm_bytes);
        Self { has_symbols }
    }

    fn check_debug_symbols(wasm_bytes: &[u8]) -> bool {
        // Check if WASM contains debug sections
        if let Ok(obj_file) = object::File::parse(wasm_bytes) {
            obj_file.section_by_name(".debug_info").is_some()
                && obj_file.section_by_name(".debug_line").is_some()
        } else {
            false
        }
    }

    pub fn map_wasm_offset_to_source(&self, _wasm_offset: u64) -> Option<SourceLocation> {
        if !self.has_symbols {
            return None;
        }

        // For demonstration purposes, simulate mapping
        // In a real implementation, this would use addr2line or similar
        Some(SourceLocation {
            file: "token.rs".to_string(),
            line: 45,
            column: Some(12),
        })
    }

    pub fn has_debug_symbols(&self) -> bool {
        self.has_symbols
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_source_mapper_without_symbols() {
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d]; // Basic WASM header
        let mapper = SourceMapper::new(wasm_bytes);

        assert!(!mapper.has_debug_symbols());
        assert!(mapper.map_wasm_offset_to_source(0x1234).is_none());
    }

    #[test]
    fn test_source_mapper_with_mock_symbols() {
        // This would be a WASM file with debug symbols in a real test
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let mapper = SourceMapper::new(wasm_bytes);

        // For now, this will return false since we don't have real debug symbols
        // In a real implementation with proper WASM + debug symbols, this would be true
        assert!(!mapper.has_debug_symbols());
    }

    #[test]
    fn test_source_location_serialization() {
        let location = SourceLocation {
            file: "test.rs".to_string(),
            line: 42,
            column: Some(10),
        };

        let json = serde_json::to_string(&location).unwrap();
        assert!(json.contains("test.rs"));
        assert!(json.contains("42"));
    }
}
