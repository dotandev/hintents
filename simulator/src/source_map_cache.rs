// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Source Map Caching Layer
//!
//! This module provides caching of parsed source map mappings to speed up
//! repetitive debugging sessions. Cached mappings are stored in
//! ~/.erst/cache/sourcemaps indexed by WASM SHA256 hash.

#![allow(dead_code)]

use crate::source_mapper::SourceLocation;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::PathBuf;

/// Default cache directory name
pub const CACHE_DIR_NAME: &str = "sourcemaps";

/// Cache entry containing parsed source mappings
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SourceMapCacheEntry {
    /// The WASM hash this entry corresponds to
    pub wasm_hash: String,
    /// Whether the WASM had debug symbols
    pub has_symbols: bool,
    /// Cached mappings from wasm offset to source location
    pub mappings: HashMap<u64, SourceLocation>,
    /// Timestamp when the entry was created
    pub created_at: u64,
}

/// Source map cache manager
pub struct SourceMapCache {
    cache_dir: PathBuf,
}

impl SourceMapCache {
    /// Creates a new SourceMapCache with the default cache directory
    pub fn new() -> Result<Self, String> {
        let cache_dir = Self::get_default_cache_dir()?;
        Ok(Self { cache_dir })
    }

    /// Creates a new SourceMapCache with a custom cache directory
    pub fn with_cache_dir(cache_dir: PathBuf) -> Result<Self, String> {
        // Ensure the cache directory exists
        fs::create_dir_all(&cache_dir)
            .map_err(|e| format!("Failed to create cache directory: {}", e))?;
        Ok(Self { cache_dir })
    }

    /// Gets the default cache directory (~/.erst/cache/sourcemaps)
    fn get_default_cache_dir() -> Result<PathBuf, String> {
        let home_dir =
            dirs::home_dir().ok_or_else(|| "Failed to determine home directory".to_string())?;
        Ok(home_dir.join(".erst").join("cache").join(CACHE_DIR_NAME))
    }

    /// Computes SHA256 hash of WASM bytes
    pub fn compute_wasm_hash(wasm_bytes: &[u8]) -> String {
        let mut hasher = Sha256::new();
        hasher.update(wasm_bytes);
        let result = hasher.finalize();
        hex::encode(result)
    }

    /// Gets the cache file path for a given WASM hash
    fn get_cache_path(&self, wasm_hash: &str) -> PathBuf {
        self.cache_dir.join(format!("{}.bin", wasm_hash))
    }

    /// Gets a cached source map entry if it exists and is valid
    pub fn get(&self, wasm_hash: &str) -> Option<SourceMapCacheEntry> {
        let cache_path = self.get_cache_path(wasm_hash);

        if !cache_path.exists() {
            return None;
        }

        // Read and deserialize the cache file
        let mut file = match File::open(&cache_path) {
            Ok(f) => f,
            Err(e) => {
                eprintln!("Failed to open cache file: {}", e);
                return None;
            }
        };

        let mut bytes = Vec::new();
        if let Err(e) = file.read_to_end(&mut bytes) {
            eprintln!("Failed to read cache file: {}", e);
            return None;
        };

        match bincode::deserialize(&bytes) {
            Ok(entry) => {
                println!(
                    "Cache hit! Loading source map from cache for WASM: {}",
                    &wasm_hash[..8]
                );
                Some(entry)
            }
            Err(e) => {
                eprintln!("Failed to deserialize cache entry: {}", e);
                None
            }
        }
    }

    /// Stores a source map entry in the cache
    pub fn store(&self, entry: SourceMapCacheEntry) -> Result<(), String> {
        // Ensure cache directory exists
        fs::create_dir_all(&self.cache_dir)
            .map_err(|e| format!("Failed to create cache directory: {}", e))?;

        let cache_path = self.get_cache_path(&entry.wasm_hash);

        // Serialize the entry
        let bytes = bincode::serialize(&entry)
            .map_err(|e| format!("Failed to serialize cache entry: {}", e))?;

        // Write to file
        let mut file =
            File::create(&cache_path).map_err(|e| format!("Failed to create cache file: {}", e))?;

        file.write_all(&bytes)
            .map_err(|e| format!("Failed to write cache file: {}", e))?;

        println!("Cached source map for WASM: {}", &entry.wasm_hash[..8]);

        Ok(())
    }

    /// Clears all cached source maps
    pub fn clear(&self) -> Result<usize, String> {
        if !self.cache_dir.exists() {
            return Ok(0);
        }

        let mut count = 0;
        for entry in fs::read_dir(&self.cache_dir)
            .map_err(|e| format!("Failed to read cache directory: {}", e))?
        {
            let entry = entry.map_err(|e| format!("Failed to read directory entry: {}", e))?;
            let path = entry.path();

            if path.is_file() && path.extension().is_some_and(|ext| ext == "bin") {
                fs::remove_file(&path)
                    .map_err(|e| format!("Failed to delete cache file: {}", e))?;
                count += 1;
            }
        }

        Ok(count)
    }

    /// Returns the current cache size in bytes
    #[allow(dead_code)]
    pub fn get_cache_size(&self) -> Result<u64, String> {
        if !self.cache_dir.exists() {
            return Ok(0);
        }

        let mut total_size = 0u64;
        for entry in fs::read_dir(&self.cache_dir)
            .map_err(|e| format!("Failed to read cache directory: {}", e))?
        {
            let entry = entry.map_err(|e| format!("Failed to read directory entry: {}", e))?;
            let path = entry.path();

            if path.is_file() {
                let metadata = fs::metadata(&path)
                    .map_err(|e| format!("Failed to get file metadata: {}", e))?;
                total_size += metadata.len();
            }
        }

        Ok(total_size)
    }

    /// Lists all cached entries (without loading full mappings)
    pub fn list_cached(&self) -> Result<Vec<CachedEntryInfo>, String> {
        if !self.cache_dir.exists() {
            return Ok(Vec::new());
        }

        let mut entries = Vec::new();
        for entry in fs::read_dir(&self.cache_dir)
            .map_err(|e| format!("Failed to read cache directory: {}", e))?
        {
            let entry = entry.map_err(|e| format!("Failed to read directory entry: {}", e))?;
            let path = entry.path();

            if path.is_file() && path.extension().is_some_and(|ext| ext == "bin") {
                // Read just the header to get metadata
                if let Ok(mut file) = File::open(&path) {
                    let mut bytes = Vec::new();
                    if file.read_to_end(&mut bytes).is_ok() {
                        if let Ok(cache_entry) = bincode::deserialize::<SourceMapCacheEntry>(&bytes)
                        {
                            let file_size = fs::metadata(&path).map(|m| m.len()).unwrap_or(0);

                            entries.push(CachedEntryInfo {
                                wasm_hash: cache_entry.wasm_hash,
                                has_symbols: cache_entry.has_symbols,
                                mappings_count: cache_entry.mappings.len() as u64,
                                created_at: cache_entry.created_at,
                                file_size,
                            });
                        }
                    }
                }
            }
        }

        Ok(entries)
    }

    /// Returns the cache directory path
    pub fn get_cache_dir(&self) -> &PathBuf {
        &self.cache_dir
    }
}

impl Default for SourceMapCache {
    fn default() -> Self {
        Self::new().expect("Failed to create default source map cache")
    }
}

/// Metadata about a cached entry
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CachedEntryInfo {
    pub wasm_hash: String,
    pub has_symbols: bool,
    pub mappings_count: u64,
    pub created_at: u64,
    pub file_size: u64,
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    fn create_test_cache() -> (SourceMapCache, TempDir) {
        let temp_dir = TempDir::new().unwrap();
        let cache = SourceMapCache::with_cache_dir(temp_dir.path().to_path_buf()).unwrap();
        (cache, temp_dir)
    }

    #[test]
    fn test_compute_wasm_hash() {
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d]; // Basic WASM header
        let hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);
        // This is a known hash for the given bytes
        assert_eq!(hash.len(), 64);
    }

    #[test]
    fn test_compute_wasm_hash_different() {
        let wasm_bytes1 = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_bytes2 = vec![0x01, 0x61, 0x73, 0x6d];

        let hash1 = SourceMapCache::compute_wasm_hash(&wasm_bytes1);
        let hash2 = SourceMapCache::compute_wasm_hash(&wasm_bytes2);

        assert_ne!(hash1, hash2);
    }

    #[test]
    fn test_store_and_get() {
        let (cache, _temp) = create_test_cache();

        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        let mut mappings = HashMap::new();
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

        // Store the entry
        cache.store(entry.clone()).unwrap();

        // Retrieve the entry
        let retrieved = cache.get(&wasm_hash).unwrap();
        assert_eq!(retrieved.wasm_hash, wasm_hash);
        assert!(retrieved.has_symbols);
        assert_eq!(retrieved.mappings.len(), 1);
    }

    #[test]
    fn test_get_missing() {
        let (cache, _temp) = create_test_cache();

        let result = cache.get("nonexistent_hash_12345678901234567890123456789012");
        assert!(result.is_none());
    }

    #[test]
    fn test_clear() {
        let (cache, _temp) = create_test_cache();

        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        let entry = SourceMapCacheEntry {
            wasm_hash: wasm_hash.clone(),
            has_symbols: true,
            mappings: HashMap::new(),
            created_at: 1234567890,
        };

        cache.store(entry).unwrap();
        assert!(cache.get(&wasm_hash).is_some());

        let count = cache.clear().unwrap();
        assert_eq!(count, 1);
        assert!(cache.get(&wasm_hash).is_none());
    }

    #[test]
    fn test_cache_size() {
        let (cache, _temp) = create_test_cache();

        let size = cache.get_cache_size().unwrap();
        assert_eq!(size, 0);

        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        let mut mappings = HashMap::new();
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
            wasm_hash,
            has_symbols: true,
            mappings,
            created_at: 1234567890,
        };

        cache.store(entry).unwrap();

        let size = cache.get_cache_size().unwrap();
        assert!(size > 0);
    }

    #[test]
    fn test_list_cached() {
        let (cache, _temp) = create_test_cache();

        let list = cache.list_cached().unwrap();
        assert_eq!(list.len(), 0);

        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        let entry = SourceMapCacheEntry {
            wasm_hash: wasm_hash.clone(),
            has_symbols: true,
            mappings: HashMap::new(),
            created_at: 1234567890,
        };

        cache.store(entry).unwrap();

        let list = cache.list_cached().unwrap();
        assert_eq!(list.len(), 1);
        assert_eq!(list[0].wasm_hash, wasm_hash);
    }
}
