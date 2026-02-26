// Copyright 2025 Erst Users
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

use base64::Engine;
use soroban_env_host::xdr::{LedgerEntry, LedgerKey, Limits, ReadXdr, WriteXdr};
use std::collections::HashMap;

/// Represents a decoded ledger snapshot containing key-value pairs
/// of ledger entries ready for loading into Host storage.
#[derive(Debug, Clone)]
pub struct LedgerSnapshot {
    /// Map of ledger keys to their corresponding entries
    entries: HashMap<Vec<u8>, LedgerEntry>,
}

impl LedgerSnapshot {
    /// Creates a new empty ledger snapshot.
    pub fn new() -> Self {
        Self {
            entries: HashMap::new(),
        }
    }

    /// Creates a ledger snapshot from base64-encoded XDR key-value pairs.
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
            entries: decoded_entries,
        })
    }

    /// Returns the number of entries in the snapshot.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// Returns true if the snapshot contains no entries.
    #[allow(dead_code)]
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// Returns an iterator over the entries in the snapshot.
    #[allow(dead_code)]
    pub fn iter(&self) -> impl Iterator<Item = (&Vec<u8>, &LedgerEntry)> {
        self.entries.iter()
    }

    /// Inserts a new entry into the snapshot.
    ///
    /// # Arguments
    /// * `key` - The ledger key (as XDR bytes)
    /// * `entry` - The ledger entry
    #[allow(dead_code)]
    pub fn insert(&mut self, key: Vec<u8>, entry: LedgerEntry) {
        self.entries.insert(key, entry);
    }

    /// Gets an entry from the snapshot by key.
    #[allow(dead_code)]
    pub fn get(&self, key: &[u8]) -> Option<&LedgerEntry> {
        self.entries.get(key)
    }
}

impl Default for LedgerSnapshot {
    fn default() -> Self {
        Self::new()
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

    #[error("Storage operation failed: {0}")]
    #[allow(dead_code)]
    StorageError(String),
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
        let snapshot = LedgerSnapshot::from_base64_map(&entries).expect("Failed to create snapshot from empty map");
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
        assert!(matches!(result.unwrap_err(), SnapshotError::Base64Decode(_)));
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
