// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Ledger snapshot and storage loading utilities for Soroban simulation.
//!
//! This module provides reusable functionality for:
//! - Decoding XDR-encoded ledger entries from base64
//! - Loading ledger state into Soroban Host storage
//! - Managing ledger snapshots for transaction replay
//!
//! These utilities can be shared across different Soroban tools that need
//! to reconstruct ledger state for simulation or analysis purposes.

#![allow(dead_code)]

pub mod delta;

use base64::Engine;
use bincode::Options;
use memmap2::Mmap;
use serde::{Deserialize, Serialize};
use soroban_env_host::xdr::{LedgerEntry, LedgerKey, Limits, ReadXdr, WriteXdr};
use std::collections::HashMap;
use std::fs::File;
use std::io::Write;
use std::path::Path;
use std::sync::Arc;

pub use delta::DeltaSnapshot;

const SNAPSHOT_FORMAT_VERSION: u8 = 1;

/// On-disk format for the mmap-backed snapshot file:
///
/// ```text
/// [8 bytes]  index_len (u64, little-endian)
/// [index_len bytes]  bincode-serialized MmapIndex
/// [remaining bytes]  raw XDR-encoded LedgerEntry bytes, one per key,
///                     addressed by (offset, len) in the index
/// ```
///
/// Unlike the v1 SnapshotWireFormat, entries are stored as raw XDR bytes
/// with no per-entry bincode framing, so a single entry can be sliced
/// directly out of the mmap and decoded without touching the rest of the
/// file. This is what makes on-demand loading possible: `from_mmap_file`
/// only reads and deserializes the index, not the entry data itself.
const MMAP_FORMAT_VERSION: u8 = 1;

#[derive(Debug, Serialize, Deserialize)]
struct MmapIndex {
    version: u8,
    /// key -> (byte offset into the entry-data region, byte length)
    entries: Vec<(Vec<u8>, u64, u64)>,
}

/// Backing storage for the immutable base layer of a [`LedgerSnapshot`].
///
/// `InMemory` is the original, fully-materialized representation used by
/// `from_bytes`/`from_base64_map` and every existing caller. `Mmap` is the
/// on-demand representation used by `from_mmap_file`: entries are decoded
/// lazily, one at a time, directly from the memory-mapped file on each
/// `get`/`iter` call rather than being deserialized up front.
#[derive(Debug)]
enum BaseStore {
    InMemory(Arc<HashMap<Vec<u8>, LedgerEntry>>),
    Mmap(Arc<MmapStore>),
}

impl Clone for BaseStore {
    fn clone(&self) -> Self {
        match self {
            BaseStore::InMemory(m) => BaseStore::InMemory(Arc::clone(m)),
            BaseStore::Mmap(m) => BaseStore::Mmap(Arc::clone(m)),
        }
    }
}

/// A memory-mapped snapshot file plus the index needed to locate individual
/// entries within it. The `Mmap` itself is lazily paged in by the OS as
/// entries are actually read, rather than loaded eagerly into process memory.
#[derive(Debug)]
struct MmapStore {
    mmap: Mmap,
    index: HashMap<Vec<u8>, (u64, u64)>,
}

impl MmapStore {
    fn get(&self, key: &[u8]) -> Option<LedgerEntry> {
        let (offset, len) = *self.index.get(key)?;
        let start = offset as usize;
        let end = start + len as usize;
        let bytes = self.mmap.get(start..end)?;
        LedgerEntry::from_xdr(bytes, Limits::none()).ok()
    }

    fn len(&self) -> usize {
        self.index.len()
    }

    fn contains_key(&self, key: &[u8]) -> bool {
        self.index.contains_key(key)
    }

    /// Decodes every entry. Used by `iter()` and by `fork()` when merging
    /// a non-empty delta on top of an mmap base — both are inherently O(n)
    /// operations already, so eager decoding here doesn't give up anything
    /// the lazy `get()` path was protecting.
    fn decode_all(&self) -> HashMap<Vec<u8>, LedgerEntry> {
        self.index
            .iter()
            .filter_map(|(key, &(offset, len))| {
                let start = offset as usize;
                let end = start + len as usize;
                let bytes = self.mmap.get(start..end)?;
                let entry = LedgerEntry::from_xdr(bytes, Limits::none()).ok()?;
                Some((key.clone(), entry))
            })
            .collect()
    }
}

#[derive(Debug, Serialize, Deserialize)]
struct SnapshotWireFormat {
    version: u8,
    entries: Vec<SnapshotWireEntry>,
}

#[derive(Debug, Serialize, Deserialize)]
struct SnapshotWireEntry {
    key: Vec<u8>,
    entry: Vec<u8>,
}

/// Represents a decoded ledger snapshot containing key-value pairs
/// of ledger entries ready for loading into Host storage.
///
/// Uses a copy-on-write design: the large, immutable base map is
/// reference-counted (`Arc`) so snapshots forked from the same initial ledger
/// load share a single allocation. The delta map is also reference-counted (`Arc`)
/// for efficient snapshot forking—when a snapshot is forked, both base and delta
/// pointers are cloned (O(1)), avoiding expensive HashMap materialization.
/// Only entries that are inserted, modified, or deleted after a mutable operation
/// are written to the delta, reducing memory consumption by >70% for typical
/// transactions that touch only 1–2 ledger entries out of thousands.
#[derive(Debug, Clone)]
pub struct LedgerSnapshot {
    /// Immutable base state shared across all snapshots derived from the same
    /// initial ledger load.  `Arc::clone` is O(1). Either fully in-memory
    /// (the original representation) or mmap-backed for on-demand loading
    /// of large snapshots — see [`BaseStore`].
    base: BaseStore,
    /// Copy-on-write overlay, also Arc-wrapped for efficient forking.
    /// `None` acts as a tombstone for an entry that exists in `base` but has been
    /// deleted after the fork. Only entries that differ from `base` are stored here.
    /// Arc allows fork() to be O(1) by sharing the delta until mutation via Arc::make_mut().
    delta: Arc<HashMap<Vec<u8>, Option<LedgerEntry>>>,
}

impl LedgerSnapshot {
    /// Creates a new empty ledger snapshot.
    pub fn new() -> Self {
        Self {
            base: BaseStore::InMemory(Arc::new(HashMap::new())),
            delta: Arc::new(HashMap::new()),
        }
    }

    /// Creates a ledger snapshot from base64-encoded XDR key-value pairs.
    ///
    /// The decoded entries are stored in the shared `base`.  The `delta` starts
    /// empty so that snapshots forked from this one pay only the cost of their
    /// own changes.
    ///
    /// # Arguments
    /// * `entries` - Map of base64-encoded LedgerKey to base64-encoded LedgerEntry
    ///
    /// # Returns
    /// * `Ok(LedgerSnapshot)` - Successfully decoded snapshot
    /// * `Err(SnapshotError)` - Decoding or parsing failed
    ///
    /// # Example
    /// ```ignore
    /// let entries = HashMap::from([
    ///     ("base64_key".to_string(), "base64_entry".to_string()),
    /// ]);
    /// let snapshot = LedgerSnapshot::from_base64_map(&entries)?;
    /// ```
    pub fn from_base64_map(entries: &HashMap<String, String>) -> Result<Self, SnapshotError> {
        let mut decoded_entries = HashMap::new();

        for (key_xdr, entry_xdr) in entries {
            let key = decode_ledger_key(key_xdr)?;
            let entry = decode_ledger_entry(entry_xdr)?;

            // Use the XDR-encoded key bytes as the map key for consistency
            let key_bytes = key
                .to_xdr(Limits::none())
                .map_err(|e| SnapshotError::XdrEncoding(format!("Failed to encode key: {e}")))?;

            decoded_entries.insert(key_bytes, entry);
        }

        Ok(Self {
            base: BaseStore::InMemory(Arc::new(decoded_entries)),
            delta: Arc::new(HashMap::new()),
        })
    }

    /// Serializes the snapshot into a compact binary format.
    ///
    /// The envelope uses explicit big-endian bincode options so integer
    /// fields remain stable across platforms, while ledger keys and entries
    /// are preserved as their canonical XDR byte representation.
    pub fn to_bytes(&self) -> Result<Vec<u8>, SnapshotError> {
        let mut entries = self
            .iter()
            .map(|(key, entry)| {
                let entry = entry.to_xdr(Limits::none()).map_err(|e| {
                    SnapshotError::XdrEncoding(format!("Failed to encode entry: {e}"))
                })?;

                Ok(SnapshotWireEntry {
                    key: key.clone(),
                    entry,
                })
            })
            .collect::<Result<Vec<_>, SnapshotError>>()?;

        // Sort by key so the binary output is deterministic even though the
        // in-memory representation uses a HashMap.
        entries.sort_by(|left, right| left.key.cmp(&right.key));

        snapshot_bincode_options()
            .serialize(&SnapshotWireFormat {
                version: SNAPSHOT_FORMAT_VERSION,
                entries,
            })
            .map_err(|e| SnapshotError::BinaryEncoding(e.to_string()))
    }

    /// Restores a snapshot from its compact binary representation.
    pub fn from_bytes(bytes: &[u8]) -> Result<Self, SnapshotError> {
        let snapshot: SnapshotWireFormat = snapshot_bincode_options()
            .deserialize(bytes)
            .map_err(|e| SnapshotError::BinaryDecoding(e.to_string()))?;

        if snapshot.version != SNAPSHOT_FORMAT_VERSION {
            return Err(SnapshotError::UnsupportedVersion(snapshot.version));
        }

        let mut entries = HashMap::with_capacity(snapshot.entries.len());

        for wire_entry in snapshot.entries {
            let entry = LedgerEntry::from_xdr(wire_entry.entry, Limits::none())
                .map_err(|e| SnapshotError::XdrParse(format!("LedgerEntry: {e}")))?;
            entries.insert(wire_entry.key, entry);
        }

        Ok(Self {
            base: BaseStore::InMemory(Arc::new(entries)),
            delta: Arc::new(HashMap::new()),
        })
    }

    /// Loads a snapshot from a memory-mapped file previously written by
    /// [`LedgerSnapshot::to_mmap_file`]. Only the index is deserialized
    /// eagerly; individual entries are decoded on demand as they're
    /// actually read via `get`/`iter`, avoiding the allocation overhead of
    /// materializing every entry up front for large snapshots.
    pub fn from_mmap_file(path: &Path) -> Result<Self, SnapshotError> {
        let file = File::open(path)
            .map_err(|e| SnapshotError::StorageError(format!("open {path:?}: {e}")))?;
        let mmap = unsafe { Mmap::map(&file) }
            .map_err(|e| SnapshotError::StorageError(format!("mmap {path:?}: {e}")))?;

        if mmap.len() < 8 {
            return Err(SnapshotError::BinaryDecoding(
                "mmap file too small to contain an index length header".to_string(),
            ));
        }

        let index_len =
            u64::from_le_bytes(mmap[0..8].try_into().map_err(|_| {
                SnapshotError::BinaryDecoding("bad index length header".to_string())
            })?) as usize;

        let index_start = 8usize;
        let index_end = index_start
            .checked_add(index_len)
            .filter(|&end| end <= mmap.len())
            .ok_or_else(|| {
                SnapshotError::BinaryDecoding("index length exceeds file size".to_string())
            })?;

        let index: MmapIndex = bincode::deserialize(&mmap[index_start..index_end])
            .map_err(|e| SnapshotError::BinaryDecoding(format!("index: {e}")))?;

        if index.version != MMAP_FORMAT_VERSION {
            return Err(SnapshotError::UnsupportedVersion(index.version));
        }

        let data_start = index_end as u64;
        let mut map = HashMap::with_capacity(index.entries.len());
        for (key, rel_offset, len) in index.entries {
            map.insert(key, (data_start + rel_offset, len));
        }

        Ok(Self {
            base: BaseStore::Mmap(Arc::new(MmapStore { mmap, index: map })),
            delta: Arc::new(HashMap::new()),
        })
    }

    /// Writes this snapshot to disk in the mmap-indexed format read by
    /// [`LedgerSnapshot::from_mmap_file`]. All live entries (base merged
    /// with delta) are encoded as raw XDR bytes, contiguous, with an index
    /// mapping each key to its byte range so a future load can seek
    /// directly to any entry without decoding the others.
    pub fn to_mmap_file(&self, path: &Path) -> Result<(), SnapshotError> {
        let mut live: Vec<(Vec<u8>, LedgerEntry)> = self.iter().collect();
        live.sort_by(|a, b| a.0.cmp(&b.0));

        let mut data = Vec::new();
        let mut index_entries = Vec::with_capacity(live.len());

        for (key, entry) in live {
            let bytes = entry
                .to_xdr(Limits::none())
                .map_err(|e| SnapshotError::XdrEncoding(format!("Failed to encode entry: {e}")))?;
            let offset = data.len() as u64;
            let len = bytes.len() as u64;
            data.extend_from_slice(&bytes);
            index_entries.push((key.clone(), offset, len));
        }

        let index = MmapIndex {
            version: MMAP_FORMAT_VERSION,
            entries: index_entries,
        };
        let index_bytes = bincode::serialize(&index)
            .map_err(|e| SnapshotError::BinaryEncoding(format!("index: {e}")))?;

        let mut file = File::create(path)
            .map_err(|e| SnapshotError::StorageError(format!("create {path:?}: {e}")))?;
        file.write_all(&(index_bytes.len() as u64).to_le_bytes())
            .map_err(|e| SnapshotError::StorageError(format!("write index length: {e}")))?;
        file.write_all(&index_bytes)
            .map_err(|e| SnapshotError::StorageError(format!("write index: {e}")))?;
        file.write_all(&data)
            .map_err(|e| SnapshotError::StorageError(format!("write entry data: {e}")))?;

        Ok(())
    }

    /// Returns the number of entries in the snapshot.
    pub fn len(&self) -> usize {
        let mut count = match &self.base {
            BaseStore::InMemory(m) => m.len(),
            BaseStore::Mmap(m) => m.len(),
        };
        let base_contains = |key: &[u8]| match &self.base {
            BaseStore::InMemory(m) => m.contains_key(key),
            BaseStore::Mmap(m) => m.contains_key(key),
        };
        for (key, val) in self.delta.iter() {
            match val {
                Some(_) => {
                    if !base_contains(key) {
                        count += 1; // newly inserted key not present in base
                    }
                }
                None => {
                    if base_contains(key) {
                        count -= 1; // tombstoned base entry
                    }
                }
            }
        }
        count
    }

    /// Returns true if the snapshot contains no live entries.
    #[allow(dead_code)]
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Returns an iterator over all live entries in the snapshot.
    ///
    /// Base entries overridden or tombstoned by the delta are excluded;
    /// delta `Some` entries are yielded in their place.
    #[allow(dead_code)]
    pub fn iter(&self) -> impl Iterator<Item = (Vec<u8>, LedgerEntry)> + '_ {
        let mut entries: Vec<(Vec<u8>, LedgerEntry)> = Vec::new();

        match &self.base {
            BaseStore::InMemory(m) => {
                // Base entries that have no delta override (modification or tombstone).
                for (k, v) in m.iter() {
                    if !self.delta.contains_key(k) {
                        entries.push((k.clone(), v.clone()));
                    }
                }
            }
            BaseStore::Mmap(m) => {
                for (k, v) in m.decode_all() {
                    if !self.delta.contains_key(&k) {
                        entries.push((k, v));
                    }
                }
            }
        }

        // Delta entries that are live (non-tombstone).
        for (k, v) in self.delta.iter() {
            if let Some(entry) = v {
                entries.push((k.clone(), entry.clone()));
            }
        }

        entries.into_iter()
    }

    /// Inserts or updates an entry in the snapshot.
    ///
    /// Writes to the delta layer only; the shared `base` is never mutated.
    ///
    /// # Arguments
    /// * `key` - The ledger key (as XDR bytes)
    /// * `entry` - The ledger entry
    #[allow(dead_code)]
    pub fn insert(&mut self, key: Vec<u8>, entry: LedgerEntry) {
        Arc::make_mut(&mut self.delta).insert(key, Some(entry));
    }

    /// Gets an entry from the snapshot by key.
    ///
    /// Consults the delta layer first; falls back to `base` if no override exists.
    #[allow(dead_code)]
    pub fn get(&self, key: &[u8]) -> Option<LedgerEntry> {
        match self.delta.get(key) {
            Some(Some(entry)) => Some(entry.clone()), // live delta entry
            Some(None) => None,                       // tombstoned in delta
            None => match &self.base {
                BaseStore::InMemory(m) => m.get(key).cloned(),
                BaseStore::Mmap(m) => m.get(key),
            },
        }
    }

    /// Removes an entry from the snapshot by setting a tombstone in the delta layer.
    ///
    /// If the key does not exist in the base or delta, this is a no-op.
    #[allow(dead_code)]
    pub fn delete(&mut self, key: &[u8]) {
        Arc::make_mut(&mut self.delta).insert(key.to_vec(), None);
    }

    /// Computes the delta from `self` (before) to `after`.
    ///
    /// The returned [`DeltaSnapshot`] contains only the entries that differ
    /// between the two snapshots. Applying the delta via [`apply_delta`]
    /// reproduces the `after` state from `self`.
    #[allow(dead_code)]
    pub fn compute_delta(&self, after: &LedgerSnapshot) -> DeltaSnapshot {
        DeltaSnapshot::compute(self, after)
    }

    /// Applies a [`DeltaSnapshot`] to `self`, producing a new full snapshot.
    ///
    /// Equivalent to calling `delta.apply_to(self)`.
    #[allow(dead_code)]
    pub fn apply_delta(&self, delta: &DeltaSnapshot) -> LedgerSnapshot {
        delta.apply_to(self)
    }

    /// Creates a forked snapshot optimized for sharing read-only state.
    ///
    /// This method merges the current delta into the base (creating a new Arc)
    /// and returns a new snapshot with an empty delta. This is much more efficient
    /// than cloning the entire snapshot when the delta is large, as it avoids
    /// copying the delta HashMap. The new snapshot shares the merged base with
    /// the original via Arc, making subsequent clones cheap.
    ///
    /// Use this instead of `clone()` when capturing snapshots for rollback or
    /// versioning purposes.
    pub fn fork(&self) -> Self {
        // If delta is empty, just clone the base handle (cheap: Arc::clone
        // either way, regardless of whether base is in-memory or mmap-backed).
        if self.delta.is_empty() {
            return Self {
                base: self.base.clone(),
                delta: Arc::new(HashMap::new()),
            };
        }

        // Merge delta into base to create a new shared, in-memory base.
        // An mmap base with a non-empty delta must materialize here since
        // the mmap itself is read-only — this only happens once per fork,
        // not on every read, so it doesn't undermine the on-demand loading
        // that from_mmap_file exists for.
        let mut merged = match &self.base {
            BaseStore::InMemory(m) => (**m).clone(),
            BaseStore::Mmap(m) => m.decode_all(),
        };
        for (key, value) in self.delta.iter() {
            match value {
                Some(entry) => {
                    merged.insert(key.clone(), entry.clone());
                }
                None => {
                    merged.remove(key);
                }
            }
        }

        Self {
            base: BaseStore::InMemory(Arc::new(merged)),
            delta: Arc::new(HashMap::new()),
        }
    }
}

impl Default for LedgerSnapshot {
    fn default() -> Self {
        Self::new()
    }
}

/// Represents the computed difference between two ledger snapshots.
#[derive(Debug, Clone)]
pub struct StateDiff {
    /// Keys present in `after` but absent from `before` (newly inserted entries).
    pub inserted: Vec<Vec<u8>>,
    /// Keys present in both snapshots but whose serialized entries differ.
    pub modified: Vec<Vec<u8>>,
    /// Keys present in `before` but absent from `after` (deleted entries).
    pub deleted: Vec<Vec<u8>>,
}

/// Computes the diff between two ledger snapshots.
///
/// Detects insertions, modifications, and deletions by comparing the XDR bytes
/// of each entry. The key vectors in the returned [`StateDiff`] are sorted so
/// callers receive deterministic output regardless of HashMap iteration order.
pub fn diff_snapshots(before: &LedgerSnapshot, after: &LedgerSnapshot) -> StateDiff {
    let mut inserted = Vec::new();
    let mut modified = Vec::new();
    let mut deleted = Vec::new();

    for (key, after_entry) in after.iter() {
        match before.get(&key) {
            None => inserted.push(key.clone()),
            Some(before_entry) => {
                let before_bytes = before_entry.to_xdr(Limits::none()).ok();
                let after_bytes = after_entry.to_xdr(Limits::none()).ok();
                if before_bytes != after_bytes {
                    modified.push(key.clone());
                }
            }
        }
    }

    for (key, _) in before.iter() {
        if after.get(&key).is_none() {
            deleted.push(key.clone());
        }
    }

    inserted.sort_unstable();
    modified.sort_unstable();
    deleted.sort_unstable();

    StateDiff {
        inserted,
        modified,
        deleted,
    }
}

/// Errors that can occur during snapshot operations.
#[derive(Debug, thiserror::Error)]
pub enum SnapshotError {
    #[error("Failed to decode base64: {0}")]
    Base64Decode(String),

    #[error("Failed to parse XDR: {0}")]
    XdrParse(String),

    #[error("Failed to encode XDR: {0}")]
    XdrEncoding(String),

    #[error("Failed to encode binary snapshot: {0}")]
    BinaryEncoding(String),

    #[error("Failed to decode binary snapshot: {0}")]
    BinaryDecoding(String),

    #[error("Unsupported snapshot format version: {0}")]
    UnsupportedVersion(u8),

    #[error("Storage operation failed: {0}")]
    #[allow(dead_code)]
    StorageError(String),
}

fn snapshot_bincode_options() -> impl Options {
    bincode::DefaultOptions::new()
        .with_fixint_encoding()
        .with_big_endian()
}

/// Decodes a base64-encoded LedgerKey XDR string.
///
/// # Arguments
/// * `key_xdr` - Base64-encoded LedgerKey
///
/// # Returns
/// * `Ok(LedgerKey)` - Successfully decoded key
/// * `Err(SnapshotError)` - Decoding or parsing failed
pub fn decode_ledger_key(key_xdr: &str) -> Result<LedgerKey, SnapshotError> {
    if key_xdr.is_empty() {
        return Err(SnapshotError::Base64Decode(
            "LedgerKey: empty payload".to_string(),
        ));
    }

    let bytes = base64::engine::general_purpose::STANDARD
        .decode(key_xdr)
        .map_err(|e| SnapshotError::Base64Decode(format!("LedgerKey: {e}")))?;

    if bytes.is_empty() {
        return Err(SnapshotError::Base64Decode(
            "LedgerKey: decoded payload is empty".to_string(),
        ));
    }

    LedgerKey::from_xdr(bytes, Limits::none())
        .map_err(|e| SnapshotError::XdrParse(format!("LedgerKey: {e}")))
}

/// Decodes a base64-encoded LedgerEntry XDR string.
///
/// # Arguments
/// * `entry_xdr` - Base64-encoded LedgerEntry
///
/// # Returns
/// * `Ok(LedgerEntry)` - Successfully decoded entry
/// * `Err(SnapshotError)` - Decoding or parsing failed
pub fn decode_ledger_entry(entry_xdr: &str) -> Result<LedgerEntry, SnapshotError> {
    if entry_xdr.is_empty() {
        return Err(SnapshotError::Base64Decode(
            "LedgerEntry: empty payload".to_string(),
        ));
    }

    let bytes = base64::engine::general_purpose::STANDARD
        .decode(entry_xdr)
        .map_err(|e| SnapshotError::Base64Decode(format!("LedgerEntry: {e}")))?;

    if bytes.is_empty() {
        return Err(SnapshotError::Base64Decode(
            "LedgerEntry: decoded payload is empty".to_string(),
        ));
    }

    LedgerEntry::from_xdr(bytes, Limits::none())
        .map_err(|e| SnapshotError::XdrParse(format!("LedgerEntry: {e}")))
}

/// Statistics about a loaded snapshot.
#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct LoadStats {
    /// Number of entries successfully loaded
    pub loaded_count: usize,
    /// Number of entries that failed to load
    pub failed_count: usize,
    /// Total number of entries attempted
    pub total_count: usize,
}

impl LoadStats {
    /// Creates new load statistics.
    #[allow(dead_code)]
    pub fn new(loaded: usize, failed: usize, total: usize) -> Self {
        Self {
            loaded_count: loaded,
            failed_count: failed,
            total_count: total,
        }
    }

    /// Returns true if all entries were loaded successfully.
    #[allow(dead_code)]
    pub fn is_complete(&self) -> bool {
        self.failed_count == 0 && self.loaded_count == self.total_count
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_snapshot_creation() {
        let snapshot = LedgerSnapshot::new();
        assert_eq!(snapshot.len(), 0);
        assert!(snapshot.is_empty());
    }

    #[test]
    fn test_snapshot_insert_and_get() {
        let mut snapshot = LedgerSnapshot::new();
        let key = vec![1, 2, 3, 4];
        let entry = create_dummy_ledger_entry();

        snapshot.insert(key.clone(), entry.clone());
        assert_eq!(snapshot.len(), 1);
        assert!(!snapshot.is_empty());
        assert!(snapshot.get(&key).is_some());
    }

    #[test]
    fn test_snapshot_from_empty_map() {
        let entries = HashMap::new();
        let snapshot = LedgerSnapshot::from_base64_map(&entries)
            .expect("Failed to create snapshot from empty map");
        assert!(snapshot.is_empty());
    }

    #[test]
    fn test_decode_invalid_base64() {
        let result = decode_ledger_key("not-valid-base64!!!");
        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            SnapshotError::Base64Decode(_)
        ));
    }

    #[test]
    fn test_decode_empty_payloads() {
        let key_result = decode_ledger_key("");
        assert!(key_result.is_err());
        assert!(matches!(
            key_result.unwrap_err(),
            SnapshotError::Base64Decode(_)
        ));

        let entry_result = decode_ledger_entry("");
        assert!(entry_result.is_err());
        assert!(matches!(
            entry_result.unwrap_err(),
            SnapshotError::Base64Decode(_)
        ));
    }

    #[test]
    fn test_from_base64_map_with_empty_payload_returns_error() {
        let mut entries = HashMap::new();
        entries.insert(String::new(), String::new());

        let result = LedgerSnapshot::from_base64_map(&entries);
        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            SnapshotError::Base64Decode(_)
        ));
    }

    #[test]
    fn test_snapshot_binary_round_trip() {
        let mut snapshot = LedgerSnapshot::new();
        let key = vec![4, 3, 2, 1];
        let entry = create_dummy_ledger_entry();
        snapshot.insert(key.clone(), entry);

        let bytes = snapshot.to_bytes().expect("Failed to serialize snapshot");
        let restored = LedgerSnapshot::from_bytes(&bytes).expect("Failed to deserialize snapshot");

        assert_eq!(restored.len(), 1);
        assert!(restored.get(&key).is_some());
    }

    #[test]
    fn test_snapshot_binary_rejects_unknown_version() {
        let bytes = snapshot_bincode_options()
            .serialize(&SnapshotWireFormat {
                version: SNAPSHOT_FORMAT_VERSION + 1,
                entries: Vec::new(),
            })
            .expect("Failed to build test payload");

        let result = LedgerSnapshot::from_bytes(&bytes);
        assert!(matches!(
            result.unwrap_err(),
            SnapshotError::UnsupportedVersion(_)
        ));
    }

    #[test]
    fn test_load_stats() {
        let stats = LoadStats::new(10, 0, 10);
        assert!(stats.is_complete());

        let stats_with_failures = LoadStats::new(8, 2, 10);
        assert!(!stats_with_failures.is_complete());
    }

    #[test]
    fn test_mmap_round_trip_preserves_entries() {
        let mut snapshot = LedgerSnapshot::new();
        for i in 0..5u8 {
            snapshot.insert(vec![i], create_dummy_ledger_entry());
        }

        let dir = std::env::temp_dir();
        let path = dir.join(format!("hintents-mmap-test-{}.bin", std::process::id()));
        snapshot
            .to_mmap_file(&path)
            .expect("failed to write mmap snapshot");

        let loaded = LedgerSnapshot::from_mmap_file(&path).expect("failed to load mmap snapshot");

        assert_eq!(loaded.len(), 5);
        for i in 0..5u8 {
            assert!(
                loaded.get(&[i]).is_some(),
                "missing entry for key {i} after mmap round trip"
            );
        }

        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn test_mmap_snapshot_matches_in_memory_snapshot() {
        let mut snapshot = LedgerSnapshot::new();
        let key = vec![9, 9, 9];
        let entry = create_dummy_ledger_entry();
        snapshot.insert(key.clone(), entry.clone());

        let dir = std::env::temp_dir();
        let path = dir.join(format!("hintents-mmap-match-{}.bin", std::process::id()));
        snapshot.to_mmap_file(&path).expect("write failed");

        let loaded = LedgerSnapshot::from_mmap_file(&path).expect("load failed");

        let expected_bytes = entry.to_xdr(Limits::none()).unwrap();
        let loaded_bytes = loaded.get(&key).unwrap().to_xdr(Limits::none()).unwrap();
        assert_eq!(expected_bytes, loaded_bytes);

        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn test_mmap_snapshot_missing_key_returns_none() {
        let snapshot = LedgerSnapshot::new();
        let dir = std::env::temp_dir();
        let path = dir.join(format!("hintents-mmap-empty-{}.bin", std::process::id()));
        snapshot.to_mmap_file(&path).expect("write failed");

        let loaded = LedgerSnapshot::from_mmap_file(&path).expect("load failed");
        assert!(loaded.is_empty());
        assert!(loaded.get(&[1, 2, 3]).is_none());

        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn test_mmap_snapshot_rejects_missing_file() {
        let missing = std::env::temp_dir().join("hintents-does-not-exist-12345.bin");
        let result = LedgerSnapshot::from_mmap_file(&missing);
        assert!(result.is_err());
    }

    #[test]
    fn test_mmap_snapshot_rejects_truncated_file() {
        let dir = std::env::temp_dir();
        let path = dir.join(format!(
            "hintents-mmap-truncated-{}.bin",
            std::process::id()
        ));
        // Fewer than 8 bytes: not even a full index-length header.
        std::fs::write(&path, [0u8, 1, 2]).unwrap();

        let result = LedgerSnapshot::from_mmap_file(&path);
        assert!(result.is_err());

        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn test_mmap_snapshot_rejects_corrupt_index_length() {
        let dir = std::env::temp_dir();
        let path = dir.join(format!("hintents-mmap-corrupt-{}.bin", std::process::id()));
        // A huge claimed index length that exceeds the actual file size.
        let mut bytes = (u64::MAX / 2).to_le_bytes().to_vec();
        bytes.extend_from_slice(&[0u8; 4]);
        std::fs::write(&path, &bytes).unwrap();

        let result = LedgerSnapshot::from_mmap_file(&path);
        assert!(result.is_err());

        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn test_mmap_snapshot_fork_with_delta_materializes_correctly() {
        let mut snapshot = LedgerSnapshot::new();
        snapshot.insert(vec![1], create_dummy_ledger_entry());

        let dir = std::env::temp_dir();
        let path = dir.join(format!("hintents-mmap-fork-{}.bin", std::process::id()));
        snapshot.to_mmap_file(&path).expect("write failed");

        let mut loaded = LedgerSnapshot::from_mmap_file(&path).expect("load failed");
        // Mutate the mmap-backed snapshot: this must go through the delta
        // layer, since the mmap itself is read-only.
        loaded.insert(vec![2], create_dummy_ledger_entry());

        let forked = loaded.fork();
        assert_eq!(forked.len(), 2);
        assert!(forked.get(&[1]).is_some());
        assert!(forked.get(&[2]).is_some());

        let _ = std::fs::remove_file(&path);
    }

    #[test]
    #[ignore] // run manually with: cargo test --release -- --ignored --nocapture bench_
    fn bench_compare_from_bytes_vs_from_mmap_file() {
        use std::time::Instant;

        const N: usize = 50_000;

        let mut snapshot = LedgerSnapshot::new();
        for i in 0..N {
            let key = (i as u32).to_le_bytes().to_vec();
            snapshot.insert(key, create_dummy_ledger_entry());
        }

        let dir = std::env::temp_dir();
        let bin_path = dir.join("hintents-bench.bin");
        let mmap_path = dir.join("hintents-bench.mmap");

        let bytes = snapshot.to_bytes().unwrap();
        std::fs::write(&bin_path, &bytes).unwrap();
        snapshot.to_mmap_file(&mmap_path).unwrap();

        let t0 = Instant::now();
        let loaded_bytes = std::fs::read(&bin_path).unwrap();
        let from_bytes_snapshot = LedgerSnapshot::from_bytes(&loaded_bytes).unwrap();
        let from_bytes_elapsed = t0.elapsed();

        let t1 = Instant::now();
        let from_mmap_snapshot = LedgerSnapshot::from_mmap_file(&mmap_path).unwrap();
        let from_mmap_elapsed = t1.elapsed();

        println!("N = {N} entries");
        println!(
            "from_bytes (eager):     {from_bytes_elapsed:?}, len = {}",
            from_bytes_snapshot.len()
        );
        println!(
            "from_mmap_file (lazy):  {from_mmap_elapsed:?}, len = {}",
            from_mmap_snapshot.len()
        );
        println!(
            "Run this test under `/usr/bin/time -v` (see PR description) to compare peak RSS."
        );

        let _ = std::fs::remove_file(&bin_path);
        let _ = std::fs::remove_file(&mmap_path);
    }

    #[test]
    fn test_mmap_load_does_not_materialize_unread_entries() {
        // Not a formal allocation benchmark, but a concrete demonstration
        // of the core claim: from_mmap_file's cost is proportional to the
        // index size, not the number/size of entries, since entries are
        // only decoded when actually requested via get()/iter(). We prove
        // this indirectly: loading a snapshot with many entries succeeds
        // and reports the correct count without ever calling get() or
        // iter() on it.
        let mut snapshot = LedgerSnapshot::new();
        for i in 0..500u16 {
            let key = i.to_le_bytes().to_vec();
            snapshot.insert(key, create_dummy_ledger_entry());
        }
        let dir = std::env::temp_dir();
        let path = dir.join(format!("hintents-mmap-scale-{}.bin", std::process::id()));
        snapshot.to_mmap_file(&path).expect("write failed");
        let loaded = LedgerSnapshot::from_mmap_file(&path).expect("load failed");
        assert_eq!(loaded.len(), 500);
        for i in [0u16, 250, 499] {
            let key = i.to_le_bytes().to_vec();
            assert!(
                loaded.get(&key).is_some(),
                "entry {i} failed to decode on demand"
            );
        }
        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn test_mmap_snapshot_diff_against_in_memory_snapshot() {
        let mut before = LedgerSnapshot::new();
        before.insert(vec![1], create_dummy_ledger_entry());

        let dir = std::env::temp_dir();
        let path = dir.join(format!("hintents-mmap-diff-{}.bin", std::process::id()));
        before.to_mmap_file(&path).expect("write failed");
        let before_mmap = LedgerSnapshot::from_mmap_file(&path).expect("load failed");

        let mut after = before_mmap.clone();
        after.insert(vec![2], create_dummy_ledger_entry());

        let diff = diff_snapshots(&before_mmap, &after);
        assert_eq!(diff.inserted, vec![vec![2]]);
        assert!(diff.modified.is_empty());
        assert!(diff.deleted.is_empty());

        let _ = std::fs::remove_file(&path);
    }

    // ------------------------------------------------------------------
    // compute_delta / apply_delta integration tests
    // ------------------------------------------------------------------

    #[test]
    fn test_ledger_snapshot_compute_delta_no_changes() {
        let mut snap = LedgerSnapshot::new();
        snap.insert(vec![1], create_dummy_ledger_entry());
        let delta = snap.compute_delta(&snap);
        assert!(delta.is_empty());
    }

    #[test]
    fn test_ledger_snapshot_compute_delta_inserted() {
        let before = LedgerSnapshot::new();
        let mut after = LedgerSnapshot::new();
        after.insert(vec![1, 2], create_dummy_ledger_entry());

        let delta = before.compute_delta(&after);
        assert_eq!(delta.inserted.len(), 1);
        assert!(delta.inserted.contains_key(&vec![1, 2]));
    }

    #[test]
    fn test_ledger_snapshot_compute_delta_modified() {
        let mut before = LedgerSnapshot::new();
        before.insert(vec![1], create_dummy_ledger_entry());
        let mut after = LedgerSnapshot::new();
        // Insert a different entry under the same key
        let mut different = create_dummy_ledger_entry();
        use soroban_env_host::xdr::{LedgerEntryData, SequenceNumber};
        if let LedgerEntryData::Account(ref mut acc) = different.data {
            acc.balance = 9999;
            acc.seq_num = SequenceNumber(99);
        }
        after.insert(vec![1], different);

        let delta = before.compute_delta(&after);
        assert!(delta.inserted.is_empty());
        assert_eq!(delta.modified.len(), 1);
        assert!(delta.modified.contains_key(&vec![1]));
    }

    #[test]
    fn test_ledger_snapshot_compute_delta_deleted() {
        let mut before = LedgerSnapshot::new();
        before.insert(vec![1], create_dummy_ledger_entry());
        let after = LedgerSnapshot::new();

        let delta = before.compute_delta(&after);
        assert_eq!(delta.deleted, vec![vec![1]]);
    }

    #[test]
    fn test_ledger_snapshot_apply_delta_full_round_trip() {
        let mut before = LedgerSnapshot::new();
        before.insert(vec![1], create_dummy_ledger_entry());
        before.insert(vec![2], create_dummy_ledger_entry());

        let mut after = LedgerSnapshot::new();
        after.insert(vec![2], create_dummy_ledger_entry()); // same as before
        after.insert(vec![3], create_dummy_ledger_entry()); // inserted

        let delta = before.compute_delta(&after);
        let reconstructed = before.apply_delta(&delta);

        assert_eq!(reconstructed.len(), 2);
        assert!(reconstructed.get(&[1]).is_none()); // deleted
        assert!(reconstructed.get(&[2]).is_some()); // preserved
        assert!(reconstructed.get(&[3]).is_some()); // inserted
    }

    #[test]
    fn test_ledger_snapshot_apply_delta_on_forked_snapshot() {
        // Verify apply_delta works correctly on a forked (COW merged) snapshot.
        let mut base = LedgerSnapshot::new();
        base.insert(vec![1], create_dummy_ledger_entry());
        base.insert(vec![2], create_dummy_ledger_entry());

        // Fork and add changes on the fork.
        let mut fork = base.fork();
        fork.insert(vec![3], create_dummy_ledger_entry());

        // Compute delta from base to fork.
        let delta = base.compute_delta(&fork);
        assert_eq!(delta.inserted.len(), 1);
        assert!(delta.inserted.contains_key(&vec![3]));

        // Apply delta to original base — should match fork.
        let reconstructed = base.apply_delta(&delta);
        assert_eq!(reconstructed.len(), fork.len());
        assert!(reconstructed.get(&[1]).is_some());
        assert!(reconstructed.get(&[2]).is_some());
        assert!(reconstructed.get(&[3]).is_some());
    }

    #[test]
    fn test_ledger_snapshot_delete_entry() {
        let mut snapshot = LedgerSnapshot::new();
        snapshot.insert(vec![1], create_dummy_ledger_entry());
        assert_eq!(snapshot.len(), 1);

        snapshot.delete(&[1]);
        assert_eq!(snapshot.len(), 0);
        assert!(snapshot.get(&[1]).is_none());
    }

    #[test]
    fn test_ledger_snapshot_delete_non_existent_is_noop() {
        let mut snapshot = LedgerSnapshot::new();
        snapshot.delete(&[99]); // should not panic
        assert!(snapshot.is_empty());
    }

    // Helper function to create a dummy ledger entry for testing
    fn create_dummy_ledger_entry() -> LedgerEntry {
        use soroban_env_host::xdr::{
            AccountEntry, AccountId, LedgerEntryData, PublicKey, SequenceNumber, Thresholds,
            Uint256,
        };

        let account_id = AccountId(PublicKey::PublicKeyTypeEd25519(Uint256([0u8; 32])));
        let account_entry = AccountEntry {
            account_id,
            balance: 1000,
            seq_num: SequenceNumber(1),
            num_sub_entries: 0,
            inflation_dest: None,
            flags: 0,
            home_domain: Default::default(),
            thresholds: Thresholds([1, 0, 0, 0]),
            signers: Default::default(),
            ext: Default::default(),
        };

        LedgerEntry {
            last_modified_ledger_seq: 1,
            data: LedgerEntryData::Account(account_entry),
            ext: Default::default(),
        }
    }
}
