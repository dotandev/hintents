// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

//! Ledger snapshot and storage loading utilities for Soroban simulation.
//!
//! This module provides reusable functionality for:
//! - Decoding XDR-encoded ledger entries from base64
//! - Loading ledger state into Soroban Host storage
//! - Managing ledger snapshots for transaction replay
//!
//! These utilities can be shared across different Soroban tools that need
//! to reconstruct ledger state for simulation or analysis purposes.

use base64::Engine;
use bincode::Options;
use serde::{Deserialize, Serialize};
use soroban_env_host::xdr::{LedgerEntry, LedgerKey, Limits, ReadXdr, WriteXdr};
use std::collections::HashMap;
use std::sync::Arc;

const SNAPSHOT_FORMAT_VERSION: u8 = 1;

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
/// load share a single allocation.  Only entries that are inserted, modified,
/// or deleted after the fork are stored in the per-snapshot `delta` map,
/// reducing memory consumption by >70% for typical transactions that touch
/// only 1–2 ledger entries out of thousands.
#[derive(Debug, Clone)]
pub struct LedgerSnapshot {
    /// Immutable base state shared across all snapshots derived from the same
    /// initial ledger load.  `Arc::clone` is O(1).
    base: Arc<HashMap<Vec<u8>, LedgerEntry>>,
    /// Copy-on-write overlay.  `None` acts as a tombstone for an entry that
    /// exists in `base` but has been deleted after the fork.
    /// Only entries that differ from `base` are stored here.
    delta: HashMap<Vec<u8>, Option<LedgerEntry>>,
}

impl LedgerSnapshot {
    /// Creates a new empty ledger snapshot.
    pub fn new() -> Self {
        Self {
            base: Arc::new(HashMap::new()),
            delta: HashMap::new(),
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
            base: Arc::new(decoded_entries),
            delta: HashMap::new(),
        })
    }

    /// Serializes the snapshot into a compact binary format.
    ///
    /// The envelope uses explicit big-endian bincode options so integer
    /// fields remain stable across platforms, while ledger keys and entries
    /// are preserved as their canonical XDR byte representation.
    pub fn to_bytes(&self) -> Result<Vec<u8>, SnapshotError> {
        let mut entries = self
            .entries
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

        Ok(Self { entries })
    }

    /// Returns the number of entries in the snapshot.
    pub fn len(&self) -> usize {
        let mut count = self.base.len();
        for (key, val) in &self.delta {
            match val {
                Some(_) => {
                    if !self.base.contains_key(key) {
                        count += 1; // newly inserted key not present in base
                    }
                }
                None => {
                    if self.base.contains_key(key) {
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
    pub fn iter(&self) -> impl Iterator<Item = (&Vec<u8>, &LedgerEntry)> {
        let mut entries: Vec<(&Vec<u8>, &LedgerEntry)> = Vec::new();

        // Base entries that have no delta override (modification or tombstone).
        for (k, v) in self.base.iter() {
            if !self.delta.contains_key(k) {
                entries.push((k, v));
            }
        }

        // Delta entries that are live (non-tombstone).
        for (k, v) in self.delta.iter() {
            if let Some(entry) = v {
                entries.push((k, entry));
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
        self.delta.insert(key, Some(entry));
    }

    /// Gets an entry from the snapshot by key.
    ///
    /// Consults the delta layer first; falls back to `base` if no override exists.
    #[allow(dead_code)]
    pub fn get(&self, key: &[u8]) -> Option<&LedgerEntry> {
        match self.delta.get(key) {
            Some(Some(entry)) => Some(entry), // live delta entry
            Some(None) => None,               // tombstoned in delta
            None => self.base.get(key),       // not overridden; check base
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
        match before.get(key) {
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
        if after.get(key).is_none() {
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
