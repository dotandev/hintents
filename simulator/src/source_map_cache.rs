// Copyright 2026 Erst Users
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
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use std::thread::{self, JoinHandle};

/// Monotonically increasing counter used to generate unique temp-file names,
/// preventing concurrent writes from clobbering each other's `.tmp` files.
static TMP_COUNTER: AtomicU64 = AtomicU64::new(0);

struct PendingWrite {
    write_id: u64,
    entry: Arc<SourceMapCacheEntry>,
}

struct WriteRequest {
    write_id: u64,
    entry: Arc<SourceMapCacheEntry>,
}

enum WriteCommand {
    Store(WriteRequest),
    Flush(mpsc::Sender<Result<(), String>>),
    Shutdown,
}

struct BackgroundWriter {
    sender: mpsc::Sender<WriteCommand>,
    handle: Option<JoinHandle<()>>,
}

impl BackgroundWriter {
    fn new(
        cache_dir: PathBuf,
        max_cache_size: Arc<Mutex<Option<u64>>>,
        pending_writes: Arc<Mutex<HashMap<String, PendingWrite>>>,
    ) -> Self {
        let (sender, receiver) = mpsc::channel();
        let handle = thread::Builder::new()
            .name("source-map-cache-writer".to_string())
            .spawn(move || {
                while let Ok(command) = receiver.recv() {
                    match command {
                        WriteCommand::Store(request) => {
                            let wasm_hash = request.entry.wasm_hash.clone();
                            let write_result = SourceMapCache::write_entry_to_disk(
                                &cache_dir,
                                request.entry.as_ref(),
                            );

                            match write_result {
                                Ok(()) => {
                                    println!("Cached source map for WASM: {}", &wasm_hash[..8]);
                                    let max_size = max_cache_size
                                        .lock()
                                        .ok()
                                        .and_then(|max_cache_size| *max_cache_size);
                                    if let Some(max_size) = max_size {
                                        if let Err(e) =
                                            SourceMapCache::evict_if_needed_in_dir(&cache_dir, max_size)
                                        {
                                            eprintln!("Failed to evict cache entries: {}", e);
                                        }
                                    }
                                }
                                Err(e) => {
                                    eprintln!(
                                        "Failed to persist source map cache for WASM {}: {}",
                                        &wasm_hash[..8],
                                        e
                                    );
                                }
                            }

                            if let Ok(mut pending) = pending_writes.lock() {
                                let should_remove = pending
                                    .get(&wasm_hash)
                                    .is_some_and(|pending| pending.write_id == request.write_id);
                                if should_remove {
                                    pending.remove(&wasm_hash);
                                }
                            }
                        }
                        WriteCommand::Flush(done) => {
                            let _ = done.send(Ok(()));
                        }
                        WriteCommand::Shutdown => break,
                    }
                }
            })
            .expect("failed to spawn source map cache writer thread");

        Self {
            sender,
            handle: Some(handle),
        }
    }

    fn enqueue(&self, request: WriteRequest) -> Result<(), String> {
        self.sender
            .send(WriteCommand::Store(request))
            .map_err(|e| format!("Failed to queue cache write: {}", e))
    }

    fn flush(&self) -> Result<(), String> {
        let (done_tx, done_rx) = mpsc::channel();
        self.sender
            .send(WriteCommand::Flush(done_tx))
            .map_err(|e| format!("Failed to queue cache flush: {}", e))?;
        done_rx
            .recv()
            .map_err(|e| format!("Failed to wait for cache flush: {}", e))?
    }
}

impl Drop for BackgroundWriter {
    fn drop(&mut self) {
        let _ = self.sender.send(WriteCommand::Shutdown);
        if let Some(handle) = self.handle.take() {
            let _ = handle.join();
        }
    }
}

// Inline OS-level advisory file locking using libc, which is a transitive
// dependency of soroban-env-host. This avoids adding a new crate while still
// providing cross-process protection against concurrent writes.
#[cfg(unix)]
mod flock {
    use std::fs::File;
    use std::io;
    use std::os::unix::io::AsRawFd;

    /// Acquires a shared (read) lock on `file`, blocking until it succeeds.
    pub fn lock_shared(file: &File) -> io::Result<()> {
        let rc = unsafe { libc::flock(file.as_raw_fd(), libc::LOCK_SH) };
        if rc == 0 {
            Ok(())
        } else {
            Err(io::Error::last_os_error())
        }
    }

    /// Acquires an exclusive (write) lock on `file`, blocking until it succeeds.
    pub fn lock_exclusive(file: &File) -> io::Result<()> {
        let rc = unsafe { libc::flock(file.as_raw_fd(), libc::LOCK_EX) };
        if rc == 0 {
            Ok(())
        } else {
            Err(io::Error::last_os_error())
        }
    }

    /// Releases any lock held on `file`.
    pub fn unlock(file: &File) -> io::Result<()> {
        let rc = unsafe { libc::flock(file.as_raw_fd(), libc::LOCK_UN) };
        if rc == 0 {
            Ok(())
        } else {
            Err(io::Error::last_os_error())
        }
    }
}

#[cfg(not(unix))]
mod flock {
    use std::fs::File;
    use std::io;
    // On non-Unix platforms we fall back to no-op locks.
    pub fn lock_shared(_: &File) -> io::Result<()> {
        Ok(())
    }
    pub fn lock_exclusive(_: &File) -> io::Result<()> {
        Ok(())
    }
    pub fn unlock(_: &File) -> io::Result<()> {
        Ok(())
    }
}

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
    max_cache_size: Arc<Mutex<Option<u64>>>,
    pending_writes: Arc<Mutex<HashMap<String, PendingWrite>>>,
    next_write_id: AtomicU64,
    writer: BackgroundWriter,
}

impl SourceMapCache {
    /// Creates a new SourceMapCache with the default cache directory
    pub fn new() -> Result<Self, String> {
        let cache_dir = Self::get_default_cache_dir()?;
        Self::build(cache_dir, None)
    }

    /// Creates a new SourceMapCache with a custom cache directory
    pub fn with_cache_dir(cache_dir: PathBuf) -> Result<Self, String> {
        fs::create_dir_all(&cache_dir)
            .map_err(|e| format!("Failed to create cache directory: {}", e))?;
        Self::build(cache_dir, None)
    }

    /// Creates a new SourceMapCache with a custom cache directory and max cache size
    pub fn with_cache_dir_and_max_size(
        cache_dir: PathBuf,
        max_cache_size: u64,
    ) -> Result<Self, String> {
        fs::create_dir_all(&cache_dir)
            .map_err(|e| format!("Failed to create cache directory: {}", e))?;
        Self::build(cache_dir, Some(max_cache_size))
    }

    /// Sets the max cache size for this cache instance
    pub fn with_max_cache_size(self, max_size: u64) -> Self {
        if let Ok(mut cache_size) = self.max_cache_size.lock() {
            *cache_size = Some(max_size);
        }
        self
    }

    fn build(cache_dir: PathBuf, max_cache_size: Option<u64>) -> Result<Self, String> {
        fs::create_dir_all(&cache_dir)
            .map_err(|e| format!("Failed to create cache directory: {}", e))?;

        let max_cache_size = Arc::new(Mutex::new(max_cache_size));
        let pending_writes = Arc::new(Mutex::new(HashMap::new()));
        let writer = BackgroundWriter::new(
            cache_dir.clone(),
            Arc::clone(&max_cache_size),
            Arc::clone(&pending_writes),
        );

        Ok(Self {
            cache_dir,
            max_cache_size,
            pending_writes,
            next_write_id: AtomicU64::new(0),
            writer,
        })
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

    /// Gets the advisory lock file path for a given cache path.
    fn get_lock_path(cache_path: &Path) -> PathBuf {
        let mut p = cache_path.to_path_buf();
        let file_name = p
            .file_name()
            .map(|n| format!("{}.lock", n.to_string_lossy()))
            .unwrap_or_else(|| ".lock".to_string());
        p.set_file_name(file_name);
        p
    }

    /// Opens or creates the advisory lock file for a cache path,
    /// returning the file handle (lock is held until the file is dropped/closed).
    fn open_lock_file(cache_path: &Path) -> Result<File, String> {
        let lock_path = Self::get_lock_path(cache_path);
        File::options()
            .create(true)
            .append(true) // Use append instead of truncate to avoid racing with other openers
            .read(true)
            .open(&lock_path)
            .map_err(|e| format!("Failed to open lock file {:?}: {}", lock_path, e))
    }

    /// Gets a cached source map entry if it exists and is valid.
    /// When `no_cache` is true, skips the cache and returns None immediately,
    /// forcing the caller to re-parse WASM symbols from scratch.
    pub fn get(&self, wasm_hash: &str, no_cache: bool) -> Option<SourceMapCacheEntry> {
        if no_cache {
            println!("Cache bypassed via --no-cache flag. Re-parsing WASM symbols.");
            return None;
        }

        if let Ok(pending) = self.pending_writes.lock() {
            if let Some(entry) = pending.get(wasm_hash) {
                println!(
                    "Cache hit! Loading source map from pending cache for WASM: {}",
                    &wasm_hash[..8]
                );
                return Some(entry.entry.as_ref().clone());
            }
        }

        let cache_path = self.get_cache_path(wasm_hash);

        if !cache_path.exists() {
            return None;
        }

        // Acquire a shared OS-level lock so concurrent readers don't race with
        // a writer that may be in the middle of replacing the file.
        let lock_file = match Self::open_lock_file(&cache_path) {
            Ok(f) => f,
            Err(e) => {
                eprintln!("Failed to open lock file for reading: {}", e);
                return None;
            }
        };
        if let Err(e) = flock::lock_shared(&lock_file) {
            eprintln!("Failed to acquire shared lock on {:?}: {}", cache_path, e);
            return None;
        }

        // Read and deserialize the cache file
        let mut file = match File::open(&cache_path) {
            Ok(f) => f,
            Err(e) => {
                eprintln!("Failed to open cache file: {}", e);
                let _ = flock::unlock(&lock_file);
                return None;
            }
        };

        let mut bytes = Vec::new();
        if let Err(e) = file.read_to_end(&mut bytes) {
            eprintln!("Failed to read cache file: {}", e);
            let _ = flock::unlock(&lock_file);
            return None;
        };

        let result = match bincode::deserialize(&bytes) {
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
        };

        let _ = flock::unlock(&lock_file);
        result
    }

    /// Stores a source map entry in the cache.
    /// Disk persistence is handed off to a background thread so the simulator
    /// thread is not blocked on filesystem I/O.
    pub fn store(&self, entry: SourceMapCacheEntry) -> Result<(), String> {
        let entry = Arc::new(entry);
        let write_id = self.next_write_id.fetch_add(1, Ordering::Relaxed);

        {
            let mut pending = self
                .pending_writes
                .lock()
                .map_err(|_| "Failed to lock pending cache writes".to_string())?;
            pending.insert(
                entry.wasm_hash.clone(),
                PendingWrite {
                    write_id,
                    entry: Arc::clone(&entry),
                },
            );
        }

        if let Err(e) = self.writer.enqueue(WriteRequest { write_id, entry }) {
            if let Ok(mut pending) = self.pending_writes.lock() {
                pending.retain(|_, pending_entry| pending_entry.write_id != write_id);
            }
            return Err(e);
        }

        Ok(())
    }

    /// Evicts oldest cache entries if current size exceeds max_size
    fn evict_if_needed(&self, max_size: u64) -> Result<(), String> {
        Self::evict_if_needed_in_dir(&self.cache_dir, max_size)
    }

    fn evict_if_needed_in_dir(cache_dir: &Path, max_size: u64) -> Result<(), String> {
        let current_size = Self::get_cache_size_in_dir(cache_dir)?;
        if current_size <= max_size {
            return Ok(());
        }

        let entries = Self::list_cached_in_dir(cache_dir)?;
        if entries.is_empty() {
            return Ok(());
        }

        let mut sorted_entries = entries;
        sorted_entries.sort_by_key(|e| e.created_at);

        let mut freed_space = 0u64;
        let target_free = current_size - max_size + (max_size / 4);

        for entry in sorted_entries {
            if freed_space >= target_free {
                break;
            }

            let cache_path = cache_dir.join(format!("{}.bin", entry.wasm_hash));
            let lock_path = SourceMapCache::get_lock_path(&cache_path);

            if cache_path.exists() {
                if let Ok(metadata) = fs::metadata(&cache_path) {
                    freed_space += metadata.len();
                }
                if let Err(e) = fs::remove_file(&cache_path) {
                    eprintln!("Failed to remove cache file {:?}: {}", cache_path, e);
                } else {
                    println!("Evicted cache entry: {}", &entry.wasm_hash[..8]);
                }
            }

            if lock_path.exists() {
                let _ = fs::remove_file(&lock_path);
            }
        }

        Ok(())
    }

    /// Clears all cached source maps
    pub fn clear(&self) -> Result<usize, String> {
        self.flush_pending_writes()?;

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
        self.flush_pending_writes()?;
        Self::get_cache_size_in_dir(&self.cache_dir)
    }

    fn get_cache_size_in_dir(cache_dir: &Path) -> Result<u64, String> {
        if !cache_dir.exists() {
            return Ok(0);
        }

        let mut total_size = 0u64;
        for entry in fs::read_dir(cache_dir)
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
        self.flush_pending_writes()?;
        Self::list_cached_in_dir(&self.cache_dir)
    }

    fn list_cached_in_dir(cache_dir: &Path) -> Result<Vec<CachedEntryInfo>, String> {
        if !cache_dir.exists() {
            return Ok(Vec::new());
        }

        let mut entries = Vec::new();
        for entry in fs::read_dir(cache_dir)
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
    pub fn get_cache_dir(&self) -> &Path {
        &self.cache_dir
    }

    fn flush_pending_writes(&self) -> Result<(), String> {
        self.writer.flush()
    }

    fn write_entry_to_disk(cache_dir: &Path, entry: &SourceMapCacheEntry) -> Result<(), String> {
        // Ensure cache directory exists.
        // On Windows, concurrent calls to create_dir_all can race and return
        // AlreadyExists (os error 183) even though the directory now exists —
        // treat that as success.
        match fs::create_dir_all(cache_dir) {
            Ok(()) => {}
            Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => {}
            Err(e) => return Err(format!("Failed to create cache directory: {}", e)),
        }

        let cache_path = cache_dir.join(format!("{}.bin", entry.wasm_hash));

        // Acquire an exclusive OS-level lock before writing.
        let lock_file = Self::open_lock_file(&cache_path)
            .map_err(|e| format!("Failed to open lock file {:?}: {}", cache_path, e))?;
        flock::lock_exclusive(&lock_file).map_err(|e| {
            format!(
                "Failed to acquire exclusive lock on {:?}: {}",
                cache_path, e
            )
        })?;

        let bytes = bincode::serialize(entry)
            .map_err(|e| format!("Failed to serialize cache entry: {}", e))?;

        // Write atomically: write to a unique tmp file then rename to avoid
        // readers observing a partially-written file and to prevent concurrent
        // writers from clobbering each other's tmp file (critical on Windows
        // where flock is a no-op).
        let tmp_id = TMP_COUNTER.fetch_add(1, Ordering::Relaxed);
        let tmp_path = cache_dir.join(format!("{}.{}.tmp", entry.wasm_hash, tmp_id));
        let write_result = (|| {
            let mut file = File::create(&tmp_path)
                .map_err(|e| format!("Failed to create temp cache file {:?}: {}", tmp_path, e))?;
            file.write_all(&bytes)
                .map_err(|e| format!("Failed to write temp cache file {:?}: {}", tmp_path, e))?;
            fs::rename(&tmp_path, &cache_path).map_err(|e| {
                format!(
                    "Failed to rename temp cache file {:?} to {:?}: {}",
                    tmp_path, cache_path, e
                )
            })?;
            Ok::<(), String>(())
        })();

        let _ = flock::unlock(&lock_file);

        if write_result.is_err() {
            let _ = fs::remove_file(&tmp_path);
        }

        write_result
    }
}

impl Default for SourceMapCache {
    fn default() -> Self {
        Self::new().expect("Failed to create default source map cache")
    }
}

impl Drop for SourceMapCache {
    fn drop(&mut self) {
        let _ = self.writer.flush();
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
                github_link: None,
            },
        );

        let entry = SourceMapCacheEntry {
            wasm_hash: wasm_hash.clone(),
            has_symbols: true,
            mappings,
            created_at: 1_234_567_890,
        };

        // Store the entry
        cache.store(entry.clone()).unwrap();

        // Retrieve the entry — no_cache=false so cache is used normally
        let retrieved = cache.get(&wasm_hash, false).unwrap();
        assert_eq!(retrieved.wasm_hash, wasm_hash);
        assert!(retrieved.has_symbols);
        assert_eq!(retrieved.mappings.len(), 1);
        assert_eq!(cache.list_cached().unwrap().len(), 1);
    }

    #[test]
    fn test_get_missing() {
        let (cache, _temp) = create_test_cache();

        let result = cache.get(
            "nonexistent_hash_1_234_567_8901_234_567_8901_234_567_89012",
            false,
        );
        assert!(result.is_none());
    }

    #[test]
    fn test_no_cache_bypasses_cache() {
        let (cache, _temp) = create_test_cache();

        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        let entry = SourceMapCacheEntry {
            wasm_hash: wasm_hash.clone(),
            has_symbols: true,
            mappings: HashMap::new(),
            created_at: 1_234_567_890,
        };

        // Store an entry so it exists on disk
        cache.store(entry).unwrap();
        assert!(cache.get(&wasm_hash, false).is_some());

        // With no_cache=true, it should return None even though cache exists
        let result = cache.get(&wasm_hash, true);
        assert!(result.is_none());
        assert_eq!(cache.list_cached().unwrap().len(), 1);
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
            created_at: 1_234_567_890,
        };

        cache.store(entry).unwrap();
        assert!(cache.get(&wasm_hash, false).is_some());

        let count = cache.clear().unwrap();
        assert_eq!(count, 1);
        assert!(cache.get(&wasm_hash, false).is_none());
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
                github_link: None,
            },
        );

        let entry = SourceMapCacheEntry {
            wasm_hash,
            has_symbols: true,
            mappings,
            created_at: 1_234_567_890,
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
            created_at: 1_234_567_890,
        };

        cache.store(entry).unwrap();

        let list = cache.list_cached().unwrap();
        assert_eq!(list.len(), 1);
        assert_eq!(list[0].wasm_hash, wasm_hash);
    }

    #[test]
    fn test_eviction_triggers_correctly() {
        let temp_dir = TempDir::new().unwrap();
        let cache =
            SourceMapCache::with_cache_dir_and_max_size(temp_dir.path().to_path_buf(), 5000)
                .unwrap();

        let wasm_bytes1 = vec![0x00, 0x61, 0x73, 0x6d, 0x01];
        let wasm_hash1 = SourceMapCache::compute_wasm_hash(&wasm_bytes1);

        let mut mappings1 = HashMap::new();
        mappings1.insert(
            0x1234,
            SourceLocation {
                file: "test1.rs".to_string(),
                line: 1,
                column: Some(1),
                column_end: None,
                github_link: None,
            },
        );

        let entry1 = SourceMapCacheEntry {
            wasm_hash: wasm_hash1.clone(),
            has_symbols: true,
            mappings: mappings1,
            created_at: 1000,
        };

        cache.store(entry1).unwrap();

        let size1 = cache.get_cache_size().unwrap();
        assert!(size1 > 0, "Cache should have some size");

        let wasm_bytes2 = vec![0x00, 0x61, 0x73, 0x6d, 0x02];
        let wasm_hash2 = SourceMapCache::compute_wasm_hash(&wasm_bytes2);

        let mut mappings2 = HashMap::new();
        for i in 0..50u64 {
            mappings2.insert(
                i,
                SourceLocation {
                    file: "test2.rs".to_string(),
                    line: i as u32,
                    column: Some(i as u32),
                    column_end: None,
                    github_link: None,
                },
            );
        }

        let entry2 = SourceMapCacheEntry {
            wasm_hash: wasm_hash2.clone(),
            has_symbols: true,
            mappings: mappings2,
            created_at: 2000,
        };

        cache.store(entry2).unwrap();

        let list = cache.list_cached().unwrap();
        assert!(
            list.len() <= 2,
            "Should have at most 2 entries after eviction"
        );

        if list.len() == 1 {
            assert_eq!(list[0].wasm_hash, wasm_hash2);
        }
    }

    #[test]
    fn test_eviction_removes_oldest_entries() {
        let temp_dir = TempDir::new().unwrap();
        let cache = SourceMapCache::with_cache_dir_and_max_size(temp_dir.path().to_path_buf(), 200)
            .unwrap();

        for i in 0..5u64 {
            let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d, i as u8];
            let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

            let mut mappings = HashMap::new();
            for j in 0..10u64 {
                mappings.insert(
                    j,
                    SourceLocation {
                        file: format!("test{}.rs", i),
                        line: j as u32,
                        column: Some(j as u32),
                        column_end: None,
                        github_link: None,
                    },
                );
            }

            let entry = SourceMapCacheEntry {
                wasm_hash,
                has_symbols: true,
                mappings,
                created_at: 1000 + i,
            };

            cache.store(entry).unwrap();
        }

        let list = cache.list_cached().unwrap();
        assert!(list.len() < 5, "Should have evicted some entries");

        if !list.is_empty() {
            let min_created_at = list.iter().map(|e| e.created_at).min().unwrap();
            assert!(
                min_created_at > 1000,
                "Oldest entries should have been evicted"
            );
        }
    }

    #[test]
    fn test_with_max_cache_size_builder() {
        let temp_dir = TempDir::new().unwrap();
        let cache = SourceMapCache::with_cache_dir(temp_dir.path().to_path_buf())
            .unwrap()
            .with_max_cache_size(500);

        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        let entry = SourceMapCacheEntry {
            wasm_hash,
            has_symbols: true,
            mappings: HashMap::new(),
            created_at: 1_234_567_890,
        };

        cache.store(entry).unwrap();

        let list = cache.list_cached().unwrap();
        assert_eq!(list.len(), 1);
    }

    #[test]
    fn test_no_eviction_when_max_size_not_set() {
        let temp_dir = TempDir::new().unwrap();
        let cache = SourceMapCache::with_cache_dir(temp_dir.path().to_path_buf()).unwrap();

        for i in 0..3u64 {
            let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d, i as u8];
            let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

            let mut mappings = HashMap::new();
            mappings.insert(
                0,
                SourceLocation {
                    file: "test.rs".to_string(),
                    line: 1,
                    column: Some(1),
                    column_end: None,
                    github_link: None,
                },
            );

            let entry = SourceMapCacheEntry {
                wasm_hash,
                has_symbols: true,
                mappings,
                created_at: 1000 + i,
            };

            cache.store(entry).unwrap();
        }

        let list = cache.list_cached().unwrap();
        assert_eq!(
            list.len(),
            3,
            "No eviction should occur without max_size set"
        );
    }
}
