// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

use gimli::{self, ColumnType, Dwarf, EndianSlice, Reader, RunTimeEndian, SectionId};
use object::{Object, ObjectSection};
use serde::{Deserialize, Serialize};
use std::borrow::Cow;
use std::path::PathBuf;

pub struct SourceMapper {
    has_symbols: bool,
    line_cache: Vec<CachedLineEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SourceLocation {
    pub file: String,
    pub line: u32,
    pub column: Option<u32>,
    pub column_end: Option<u32>,
}

#[derive(Debug, Clone)]
struct CachedLineEntry {
    start: u64,
    end: Option<u64>,
    location: SourceLocation,
}

impl SourceMapper {
    /// Creates a new SourceMapper with caching enabled
    pub fn new(wasm_bytes: Vec<u8>) -> Self {
        let has_symbols = Self::check_debug_symbols(&wasm_bytes);
        let line_cache = if has_symbols {
            Self::build_line_cache(&wasm_bytes).unwrap_or_default()
        } else {
            Vec::new()
        };

        Self {
            has_symbols,
            line_cache,
        }
    }

    /// Backward-compatible constructor used by tests.
    #[allow(dead_code)]
    pub fn new_with_cache(wasm_bytes: Vec<u8>, _cache_dir: PathBuf) -> Self {
        Self::new(wasm_bytes)
    }

    fn check_debug_symbols(wasm_bytes: &[u8]) -> bool {
        if let Ok(obj_file) = object::File::parse(wasm_bytes) {
            obj_file.section_by_name(".debug_info").is_some()
                && obj_file.section_by_name(".debug_line").is_some()
        } else {
            false
        }
    }

    #[allow(deprecated)]
    fn build_line_cache(wasm_bytes: &[u8]) -> Result<Vec<CachedLineEntry>, String> {
        let obj_file = object::File::parse(wasm_bytes)
            .map_err(|err| format!("failed to parse wasm object: {err}"))?;
        let endian = if obj_file.is_little_endian() {
            RunTimeEndian::Little
        } else {
            RunTimeEndian::Big
        };

        let dwarf_sections = Dwarf::load(|id: SectionId| -> Result<Cow<'_, [u8]>, gimli::Error> {
            if let Some(section) = obj_file.section_by_name(id.name()) {
                match section.uncompressed_data() {
                    Ok(data) => Ok(data),
                    Err(_) => Ok(Cow::Borrowed(&[])),
                }
            } else {
                Ok(Cow::Borrowed(&[]))
            }
        })
        .map_err(|err| format!("failed to load DWARF: {err}"))?;

        let dwarf = dwarf_sections.borrow(|section| EndianSlice::new(section.as_ref(), endian));
        Self::extract_line_entries(&dwarf)
            .map_err(|err| format!("failed to parse .debug_line: {err}"))
    }

    fn extract_line_entries<R>(dwarf: &Dwarf<R>) -> Result<Vec<CachedLineEntry>, gimli::Error>
    where
        R: Reader,
    {
        let mut cache = Vec::new();
        let mut units = dwarf.units();

        while let Some(header) = units.next()? {
            let unit = dwarf.unit(header)?;
            let Some(program) = unit.line_program.clone() else {
                continue;
            };

            let (program, sequences) = program.sequences()?;
            for sequence in sequences {
                let mut rows = program.resume_from(&sequence);
                let mut pending: Option<(u64, SourceLocation)> = None;

                while let Some((line_header, row)) = rows.next_row()? {
                    if row.end_sequence() {
                        if let Some((start, location)) = pending.take() {
                            cache.push(CachedLineEntry {
                                start,
                                end: Some(row.address()),
                                location,
                            });
                        }
                        continue;
                    }

                    let Some(file) = row.file(line_header) else {
                        continue;
                    };

                    let Some(file_name) =
                        Self::attr_value_to_string(dwarf, &unit, file.path_name())
                    else {
                        continue;
                    };

                    let dir_name = file
                        .directory(line_header)
                        .and_then(|dir| Self::attr_value_to_string(dwarf, &unit, dir));

                    let file_name = if let Some(dir) = dir_name {
                        if !dir.is_empty() && !file_name.starts_with('/') {
                            format!("{dir}/{file_name}")
                        } else {
                            file_name
                        }
                    } else {
                        file_name
                    };

                    let Some(line) = row.line() else {
                        continue;
                    };

                    let column = match row.column() {
                        ColumnType::LeftEdge => None,
                        ColumnType::Column(column) => Some(column.get() as u32),
                    };

                    let location = SourceLocation {
                        file: file_name,
                        line: line.get() as u32,
                        column,
                        column_end: None,
                    };

                    if let Some((start, prev_location)) = pending.replace((row.address(), location))
                    {
                        cache.push(CachedLineEntry {
                            start,
                            end: Some(row.address()),
                            location: prev_location,
                        });
                    }
                }

                if let Some((start, location)) = pending.take() {
                    cache.push(CachedLineEntry {
                        start,
                        end: None,
                        location,
                    });
                }
            }
        }

        cache.sort_by_key(|entry| entry.start);
        Self::dedupe_same_address_entries(cache)
    }

    fn dedupe_same_address_entries(
        entries: Vec<CachedLineEntry>,
    ) -> Result<Vec<CachedLineEntry>, gimli::Error> {
        let mut deduped: Vec<CachedLineEntry> = Vec::with_capacity(entries.len());
        for entry in entries {
            if let Some(last) = deduped.last_mut() {
                if last.start == entry.start {
                    *last = entry;
                    continue;
                }
            }
            deduped.push(entry);
        }
        Ok(deduped)
    }

    fn attr_value_to_string<R>(
        dwarf: &Dwarf<R>,
        unit: &gimli::Unit<R>,
        value: gimli::AttributeValue<R>,
    ) -> Option<String>
    where
        R: Reader,
    {
        let raw = dwarf.attr_string(unit, value).ok()?;
        let bytes = raw.to_slice().ok()?;
        Some(String::from_utf8_lossy(bytes.as_ref()).into_owned())
    }

    pub fn map_wasm_offset_to_source(&self, wasm_offset: u64) -> Option<SourceLocation> {
        if !self.has_symbols || self.line_cache.is_empty() {
            return None;
        }

        let idx = match self
            .line_cache
            .binary_search_by_key(&wasm_offset, |entry| entry.start)
        {
            Ok(index) => index,
            Err(0) => return None,
            Err(index) => index.saturating_sub(1),
        };

        let entry = self.line_cache.get(idx)?;
        if let Some(end) = entry.end {
            if wasm_offset >= end {
                return None;
            }
        }

        Some(entry.location.clone())
    }

    pub fn has_debug_symbols(&self) -> bool {
        self.has_symbols
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::source_map_cache::{SourceMapCache, SourceMapCacheEntry};
    use tempfile::TempDir;

    fn mapper_with_cache(entries: Vec<CachedLineEntry>) -> SourceMapper {
        SourceMapper {
            has_symbols: true,
            line_cache: entries,
        }
    }

    #[test]
    fn test_source_mapper_without_symbols() {
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let mapper = SourceMapper::new(wasm_bytes);

        assert!(!mapper.has_debug_symbols());
        assert!(mapper.map_wasm_offset_to_source(0x1234).is_none());
    }

    #[test]
    fn test_cached_lookup_uses_address_ranges() {
        let mapper = mapper_with_cache(vec![
            CachedLineEntry {
                start: 0x10,
                end: Some(0x20),
                location: SourceLocation {
                    file: "lib.rs".into(),
                    line: 10,
                    column: Some(1),
                    column_end: None,
                },
            },
            CachedLineEntry {
                start: 0x20,
                end: None,
                location: SourceLocation {
                    file: "lib.rs".into(),
                    line: 20,
                    column: Some(2),
                    column_end: None,
                },
            },
        ]);

        let loc = mapper.map_wasm_offset_to_source(0x18).expect("mapping");
        assert_eq!(loc.line, 10);
        assert_eq!(loc.column, Some(1));

        let loc = mapper.map_wasm_offset_to_source(0x25).expect("mapping");
        assert_eq!(loc.line, 20);
    }

    #[test]
    fn test_cached_lookup_respects_range_end() {
        let mapper = mapper_with_cache(vec![CachedLineEntry {
            start: 0x10,
            end: Some(0x20),
            location: SourceLocation {
                file: "mod.rs".into(),
                line: 7,
                column: None,
                column_end: None,
            },
        }]);

        assert!(mapper.map_wasm_offset_to_source(0x20).is_none());
    }

    #[test]
    fn test_source_location_serialization() {
        let location = SourceLocation {
            file: "test.rs".to_string(),
            line: 42,
            column: Some(10),
            column_end: Some(15),
        };

        let json = serde_json::to_string(&location).unwrap();
        assert!(json.contains("test.rs"));
        assert!(json.contains("42"));
    }

    #[test]
    fn test_source_mapper_with_cache() {
        let temp_dir = TempDir::new().unwrap();
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        // First create - this will NOT populate cache because has_symbols is false
        // The current implementation only caches when debug symbols are present
        {
            let mapper =
                SourceMapper::new_with_cache(wasm_bytes.clone(), temp_dir.path().to_path_buf());
            assert!(!mapper.has_debug_symbols());

            // Try to map - should work even without symbols
            let result = mapper.map_wasm_offset_to_source(0x1234);
            // Without debug symbols, should return None
            assert!(result.is_none());
        }

        // Verify cache was NOT created (since no debug symbols)
        let cache = SourceMapCache::with_cache_dir(temp_dir.path().to_path_buf()).unwrap();
        let entries = cache.list_cached().unwrap();
        assert_eq!(entries.len(), 0);

        // Test that we can create cache entries directly
        let mut mappings = std::collections::HashMap::new();
        mappings.insert(
            0x1234,
            SourceLocation {
                file: "test.rs".to_string(),
                line: 42,
                column: Some(10),
                column_end: None,
            },
        );

        let entry = SourceMapCacheEntry {
            wasm_hash: wasm_hash.clone(),
            has_symbols: true,
            mappings,
            created_at: 1234567890,
        };

        cache.store(entry).unwrap();

        // Verify cache was created
        let entries = cache.list_cached().unwrap();
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].wasm_hash, wasm_hash);
    }

    #[test]
    fn test_wasm_hash() {
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);
        assert_eq!(hash.len(), 64);
    }
}
