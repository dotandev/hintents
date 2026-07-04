// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use simulator::source_map_cache::{SourceMapCache, SourceMapCacheEntry};
use simulator::source_mapper::SourceLocation;
use std::collections::HashMap;
use std::sync::Arc;
use std::thread;
use tempfile::TempDir;

#[test]
fn test_concurrency_source_map_cache() {
    let temp_dir = TempDir::new()
        .unwrap_or_else(|err| panic!("failed to create temporary directory: {err:?}"));
    let cache_dir = temp_dir.path().to_path_buf();
    let cache = Arc::new(
        SourceMapCache::with_cache_dir(cache_dir)
            .unwrap_or_else(|err| panic!("failed to create source map cache: {err:?}")),
    );

    let wasm_hash = "test_hash_1234567890abcdef1234567890abcdef1234567890abcdef1234567890";
    let num_threads = 10u64;
    let iterations = 20u64;

    let mut handles = Vec::new();

    for t in 0..num_threads {
        let cache = Arc::clone(&cache);
        let wasm_hash = wasm_hash.to_string();

        let handle = thread::spawn(move || {
            for i in 0..iterations {
                // Mix of reads and writes
                if i % 3 == 0 {
                    // Write
                    let mut mappings = HashMap::new();
                    mappings.insert(
                        t * 1000 + i,
                        SourceLocation {
                            file: format!("file_{t}_{i}.rs"),
                            #[allow(clippy::cast_possible_truncation)]
                            line: i as u32,
                            column: Some(0),
                            column_end: None,
                            github_link: None,
                        },
                    );

                    let entry = SourceMapCacheEntry {
                        wasm_hash: wasm_hash.clone(),
                        wasm_mtime: None,
                        has_symbols: true,
                        mappings,
                        created_at: 1000 + i,
                    };

                    cache
                        .store(entry)
                        .unwrap_or_else(|err| panic!("failed to store cache entry: {err:?}"));
                } else {
                    // Read
                    let _ = cache.get(wasm_hash.as_str(), false);
                }

                // Small sleep to increase chance of contention
                thread::sleep(std::time::Duration::from_millis(1));
            }
        });
        handles.push(handle);
    }

    for handle in handles {
        handle
            .join()
            .unwrap_or_else(|err| panic!("thread panicked: {err:?}"));
    }

    // Final state should be readable
    let final_entry = cache
        .get(wasm_hash, false)
        .unwrap_or_else(|| panic!("failed to retrieve final cache entry"));
    assert_eq!(final_entry.wasm_hash, wasm_hash);
}
