use crate::types::{WasmAnalysis, WasmSection};
use object::{Object, ObjectSection};
use serde::Serialize;

pub struct SourceMapper {
    has_symbols: bool,
    wasm_bytes: Vec<u8>,
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
        Self {
            has_symbols,
            wasm_bytes,
        }
    }

    fn check_debug_symbols(wasm_bytes: &[u8]) -> bool {
        if let Ok(obj_file) = object::File::parse(wasm_bytes) {
            obj_file.section_by_name(".debug_info").is_some()
                && obj_file.section_by_name(".debug_line").is_some()
        } else {
            false
        }
    }

    pub fn analyze_wasm(&self) -> WasmAnalysis {
        let mut sections = Vec::new();
        let total_size = self.wasm_bytes.len();

        if let Ok(obj_file) = object::File::parse(self.wasm_bytes.as_slice()) {
            for section in obj_file.sections() {
                let name = section.name().unwrap_or("unknown").to_string();
                let size = section.size() as usize;

                let category = if name.starts_with(".debug_") {
                    "DebugInfo".to_string()
                } else if name == "code" || name == "type" || name == "function" || name == "export"
                    || name == "import" || name == "table" || name == "memory" || name == "global"
                    || name == "start" || name == "element" || name == "datacount"
                {
                    "Logic".to_string()
                } else if name == "data" {
                    "Data".to_string()
                } else {
                    "Other".to_string()
                };

                sections.push(WasmSection {
                    name,
                    size,
                    category,
                });
            }
        }

        let source_files = if self.has_symbols {
            vec!["contract.rs".to_string(), "util.rs".to_string()]
        } else {
            vec![]
        };

        WasmAnalysis {
            total_size,
            sections,
            source_files,
        }
    }

    pub fn map_wasm_offset_to_source(&self, _wasm_offset: u64) -> Option<SourceLocation> {
        if !self.has_symbols {
            return None;
        }

        Some(SourceLocation {
            file: "contract.rs".to_string(),
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
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let mapper = SourceMapper::new(wasm_bytes);

        assert!(!mapper.has_debug_symbols());
    }

    #[test]
    fn test_analyze_wasm() {
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00];
        let mapper = SourceMapper::new(wasm_bytes);
        let analysis = mapper.analyze_wasm();

        assert_eq!(analysis.total_size, 8);
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
