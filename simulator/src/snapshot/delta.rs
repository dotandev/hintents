// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Delta-encoded state changes for ledger snapshots.
//!
//! Instead of storing full deep-copy snapshots for every instruction boundary,
//! [`DeltaSnapshot`] stores only the set of changes (insertions, modifications,
//! deletions) that occurred between two consecutive snapshots. This minimizes
//! memory overhead because most instructions touch only 1–2 ledger entries out
//! of potentially hundreds of thousands.
//!
//! # Usage
//!
//! ```ignore
//! use crate::snapshot::delta::DeltaSnapshot;
//!
//! let delta = DeltaSnapshot::compute(&before, &after);
//! let reconstructed = delta.apply_to(&before);
//! assert!(snapshots_equal(&reconstructed, &after));
//! ```
//!
//! Deltas can also be serialized to a compact binary format:
//!
//! ```ignore
//! let bytes = delta.to_bytes()?;
//! let restored = DeltaSnapshot::from_bytes(&bytes)?;
//! ```

use super::{LedgerSnapshot, SnapshotError};
use bincode::Options;
use serde::{Deserialize, Serialize};
use soroban_env_host::xdr::{LedgerEntry, Limits, ReadXdr, WriteXdr};
use std::collections::HashMap;

/// Current wire format version for [`DeltaSnapshot`].
const DELTA_FORMAT_VERSION: u8 = 1;

// ---------------------------------------------------------------------------
// Wire-format types (private)
// ---------------------------------------------------------------------------

#[derive(Debug, Serialize, Deserialize)]
struct DeltaWireFormat {
    version: u8,
    inserted: Vec<DeltaWireEntry>,
    modified: Vec<DeltaWireEntry>,
    deleted: Vec<Vec<u8>>,
}

#[derive(Debug, Serialize, Deserialize)]
struct DeltaWireEntry {
    key: Vec<u8>,
    entry_bytes: Vec<u8>,
}

// ---------------------------------------------------------------------------
// DeltaSnapshot
// ---------------------------------------------------------------------------

/// The difference between two consecutive ledger snapshots.
///
/// A [`DeltaSnapshot`] captures only the entries that changed between the
/// "before" and "after" state of a single instruction or host function call.
/// This is significantly cheaper to store than a full [`LedgerSnapshot`] copy
/// when the number of touched entries is small (which is the common case).
#[derive(Debug, Clone)]
pub struct DeltaSnapshot {
    /// New entries that were created (key present in `after` but not `before`).
    pub inserted: HashMap<Vec<u8>, LedgerEntry>,
    /// Existing entries that were modified (key present in both, value differs).
    pub modified: HashMap<Vec<u8>, LedgerEntry>,
    /// Entries that were deleted (key present in `before` but not `after`).
    pub deleted: Vec<Vec<u8>>,
}

impl DeltaSnapshot {
    /// Creates a new empty delta.
    pub fn new() -> Self {
        Self {
            inserted: HashMap::new(),
            modified: HashMap::new(),
            deleted: Vec::new(),
        }
    }

    /// Returns `true` if this delta contains no changes.
    pub fn is_empty(&self) -> bool {
        self.inserted.is_empty() && self.modified.is_empty() && self.deleted.is_empty()
    }

    /// Returns the total number of changed entries (insertions + modifications + deletions).
    pub fn len(&self) -> usize {
        self.inserted.len() + self.modified.len() + self.deleted.len()
    }

    /// Computes the delta from `before` to `after`.
    ///
    /// The returned [`DeltaSnapshot`] contains only the entries that differ
    /// between the two snapshots. Applying it to `before` via [`apply_to`]
    /// reproduces the `after` state.
    pub fn compute(before: &LedgerSnapshot, after: &LedgerSnapshot) -> Self {
        let mut inserted = HashMap::new();
        let mut modified = HashMap::new();
        let mut deleted = Vec::new();

        // Find inserted and modified entries.
        for (key, after_entry) in after.iter() {
            match before.get(&key) {
                None => {
                    inserted.insert(key.clone(), after_entry);
                }
                Some(before_entry) => {
                    let before_bytes = before_entry.to_xdr(Limits::none()).ok();
                    let after_bytes = after_entry.to_xdr(Limits::none()).ok();
                    if before_bytes != after_bytes {
                        modified.insert(key.clone(), after_entry);
                    }
                }
            }
        }

        // Find deleted entries.
        for (key, _) in before.iter() {
            if after.get(&key).is_none() {
                deleted.push(key.clone());
            }
        }

        // Sort deleted keys for deterministic output.
        deleted.sort_unstable();

        Self {
            inserted,
            modified,
            deleted,
        }
    }

    /// Applies this delta to `base`, producing a new full [`LedgerSnapshot`].
    ///
    /// The base snapshot is not mutated — the new state is built on top of a
    /// fork of `base`.
    pub fn apply_to(&self, base: &LedgerSnapshot) -> LedgerSnapshot {
        // Fork first to merge any existing delta into a clean, shareable base.
        let mut result = base.fork();

        // Apply insertions and modifications.
        for (key, entry) in &self.inserted {
            result.insert(key.clone(), entry.clone());
        }
        for (key, entry) in &self.modified {
            result.insert(key.clone(), entry.clone());
        }

        // Apply deletions.
        for key in &self.deleted {
            result.delete(key);
        }

        result
    }

    /// Serializes this delta into a compact binary format.
    ///
    /// The envelope uses the same big-endian bincode options as
    /// [`LedgerSnapshot::to_bytes`] for platform stability. Keys and
    /// entries are stored as their canonical XDR byte representations.
    pub fn to_bytes(&self) -> Result<Vec<u8>, SnapshotError> {
        let to_wire_entry =
            |(key, entry): (&Vec<u8>, &LedgerEntry)| -> Result<DeltaWireEntry, SnapshotError> {
                let entry_bytes = entry.to_xdr(Limits::none()).map_err(|e| {
                    SnapshotError::XdrEncoding(format!("Failed to encode entry: {e}"))
                })?;
                Ok(DeltaWireEntry {
                    key: key.clone(),
                    entry_bytes,
                })
            };

        let mut inserted: Vec<DeltaWireEntry> = self
            .inserted
            .iter()
            .map(to_wire_entry)
            .collect::<Result<Vec<_>, SnapshotError>>()?;
        inserted.sort_by(|a, b| a.key.cmp(&b.key));

        let mut modified: Vec<DeltaWireEntry> = self
            .modified
            .iter()
            .map(to_wire_entry)
            .collect::<Result<Vec<_>, SnapshotError>>()?;
        modified.sort_by(|a, b| a.key.cmp(&b.key));

        let mut deleted = self.deleted.clone();
        deleted.sort_unstable();

        delta_bincode_options()
            .serialize(&DeltaWireFormat {
                version: DELTA_FORMAT_VERSION,
                inserted,
                modified,
                deleted,
            })
            .map_err(|e| SnapshotError::BinaryEncoding(format!("delta: {e}")))
    }

    /// Restores a delta from its compact binary representation produced by
    /// [`to_bytes`].
    pub fn from_bytes(bytes: &[u8]) -> Result<Self, SnapshotError> {
        let wire: DeltaWireFormat = delta_bincode_options()
            .deserialize(bytes)
            .map_err(|e| SnapshotError::BinaryDecoding(format!("delta: {e}")))?;

        if wire.version != DELTA_FORMAT_VERSION {
            return Err(SnapshotError::UnsupportedVersion(wire.version));
        }

        let from_wire_entry = |w: DeltaWireEntry| -> Result<(Vec<u8>, LedgerEntry), SnapshotError> {
            let entry = LedgerEntry::from_xdr(w.entry_bytes, Limits::none())
                .map_err(|e| SnapshotError::XdrParse(format!("LedgerEntry: {e}")))?;
            Ok((w.key, entry))
        };

        let inserted: HashMap<Vec<u8>, LedgerEntry> = wire
            .inserted
            .into_iter()
            .map(from_wire_entry)
            .collect::<Result<HashMap<_, _>, SnapshotError>>()?;

        let modified: HashMap<Vec<u8>, LedgerEntry> = wire
            .modified
            .into_iter()
            .map(from_wire_entry)
            .collect::<Result<HashMap<_, _>, SnapshotError>>()?;

        Ok(Self {
            inserted,
            modified,
            deleted: wire.deleted,
        })
    }
}

impl Default for DeltaSnapshot {
    fn default() -> Self {
        Self::new()
    }
}

/// Returns the bincode options used for delta serialization.
fn delta_bincode_options() -> impl Options {
    bincode::DefaultOptions::new()
        .with_fixint_encoding()
        .with_big_endian()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_env_host::xdr::{
        AccountEntry, AccountId, LedgerEntry, LedgerEntryData, PublicKey, SequenceNumber,
        Thresholds, Uint256,
    };

    // ------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------

    fn make_entry(balance: i64) -> LedgerEntry {
        let account_id = AccountId(PublicKey::PublicKeyTypeEd25519(Uint256([0u8; 32])));
        let account_entry = AccountEntry {
            account_id,
            balance,
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

    const KEY_A: &[u8] = &[1, 0, 0, 0];
    const KEY_B: &[u8] = &[2, 0, 0, 0];
    const KEY_C: &[u8] = &[3, 0, 0, 0];

    // ------------------------------------------------------------------
    // Basic tests
    // ------------------------------------------------------------------

    #[test]
    fn test_delta_new_is_empty() {
        let delta = DeltaSnapshot::new();
        assert!(delta.is_empty());
        assert_eq!(delta.len(), 0);
    }

    #[test]
    fn test_delta_compute_no_changes() {
        let mut snap = LedgerSnapshot::new();
        snap.insert(KEY_A.to_vec(), make_entry(100));
        snap.insert(KEY_B.to_vec(), make_entry(200));

        let delta = DeltaSnapshot::compute(&snap, &snap);
        assert!(delta.is_empty());
    }

    #[test]
    fn test_delta_compute_inserted() {
        let before = LedgerSnapshot::new();
        let mut after = LedgerSnapshot::new();
        after.insert(KEY_A.to_vec(), make_entry(10));

        let delta = DeltaSnapshot::compute(&before, &after);
        assert_eq!(delta.inserted.len(), 1);
        assert!(delta.inserted.contains_key(KEY_A));
        assert!(delta.modified.is_empty());
        assert!(delta.deleted.is_empty());
    }

    #[test]
    fn test_delta_compute_modified() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_entry(10));
        let mut after = LedgerSnapshot::new();
        after.insert(KEY_A.to_vec(), make_entry(999));

        let delta = DeltaSnapshot::compute(&before, &after);
        assert!(delta.inserted.is_empty());
        assert_eq!(delta.modified.len(), 1);
        assert!(delta.modified.contains_key(KEY_A));
        assert!(delta.deleted.is_empty());
    }

    #[test]
    fn test_delta_compute_deleted() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_entry(10));
        let after = LedgerSnapshot::new();

        let delta = DeltaSnapshot::compute(&before, &after);
        assert!(delta.inserted.is_empty());
        assert!(delta.modified.is_empty());
        assert_eq!(delta.deleted, vec![KEY_A.to_vec()]);
    }

    #[test]
    fn test_delta_compute_mixed() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_entry(100)); // deleted
        before.insert(KEY_B.to_vec(), make_entry(200)); // modified

        let mut after = LedgerSnapshot::new();
        after.insert(KEY_B.to_vec(), make_entry(999)); // modified (new value)
        after.insert(KEY_C.to_vec(), make_entry(300)); // inserted

        let delta = DeltaSnapshot::compute(&before, &after);

        assert_eq!(delta.inserted.len(), 1);
        assert!(delta.inserted.contains_key(KEY_C));

        assert_eq!(delta.modified.len(), 1);
        assert!(delta.modified.contains_key(KEY_B));

        assert_eq!(delta.deleted, vec![KEY_A.to_vec()]);
    }

    // ------------------------------------------------------------------
    // apply_to round-trip tests
    // ------------------------------------------------------------------

    #[test]
    fn test_delta_apply_empty_delta_is_identity() {
        let mut snap = LedgerSnapshot::new();
        snap.insert(KEY_A.to_vec(), make_entry(42));

        let delta = DeltaSnapshot::new();
        let result = delta.apply_to(&snap);

        assert_eq!(result.len(), 1);
        assert!(result.get(KEY_A).is_some());
    }

    #[test]
    fn test_delta_apply_insertion() {
        let before = LedgerSnapshot::new();
        let mut after = LedgerSnapshot::new();
        after.insert(KEY_A.to_vec(), make_entry(10));

        let delta = DeltaSnapshot::compute(&before, &after);
        let reconstructed = delta.apply_to(&before);

        assert_eq!(reconstructed.len(), 1);
        let entry = reconstructed.get(KEY_A).expect("entry should exist");
        let entry_bytes = entry.to_xdr(Limits::none()).unwrap();
        let expected_bytes = make_entry(10).to_xdr(Limits::none()).unwrap();
        assert_eq!(entry_bytes, expected_bytes);
    }

    #[test]
    fn test_delta_apply_modification() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_entry(10));
        let mut after = LedgerSnapshot::new();
        after.insert(KEY_A.to_vec(), make_entry(999));

        let delta = DeltaSnapshot::compute(&before, &after);
        let reconstructed = delta.apply_to(&before);

        assert_eq!(reconstructed.len(), 1);
        let entry = reconstructed.get(KEY_A).expect("entry should exist");
        let entry_bytes = entry.to_xdr(Limits::none()).unwrap();
        let expected_bytes = make_entry(999).to_xdr(Limits::none()).unwrap();
        assert_eq!(entry_bytes, expected_bytes);
    }

    #[test]
    fn test_delta_apply_deletion() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_entry(10));
        let after = LedgerSnapshot::new();

        let delta = DeltaSnapshot::compute(&before, &after);
        let reconstructed = delta.apply_to(&before);

        assert!(reconstructed.is_empty());
        assert!(reconstructed.get(KEY_A).is_none());
    }

    #[test]
    fn test_delta_apply_full_round_trip() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_entry(100)); // deleted
        before.insert(KEY_B.to_vec(), make_entry(200)); // modified
        before.insert(vec![4, 0, 0, 0], make_entry(400)); // unchanged

        let mut after = LedgerSnapshot::new();
        after.insert(KEY_B.to_vec(), make_entry(999)); // modified
        after.insert(KEY_C.to_vec(), make_entry(300)); // inserted
        after.insert(vec![4, 0, 0, 0], make_entry(400)); // unchanged

        let delta = DeltaSnapshot::compute(&before, &after);
        let reconstructed = delta.apply_to(&before);

        // Verify reconstructed matches after exactly.
        assert_eq!(reconstructed.len(), after.len());
        assert!(reconstructed.get(KEY_B).is_some());
        assert!(reconstructed.get(KEY_C).is_some());
        assert!(reconstructed.get(vec![4, 0, 0, 0].as_slice()).is_some());
        assert!(reconstructed.get(KEY_A).is_none()); // deleted
    }

    // ------------------------------------------------------------------
    // Binary round-trip tests
    // ------------------------------------------------------------------

    #[test]
    fn test_delta_binary_empty_round_trip() {
        let delta = DeltaSnapshot::new();
        let bytes = delta.to_bytes().expect("serialization failed");
        let restored = DeltaSnapshot::from_bytes(&bytes).expect("deserialization failed");

        assert!(restored.is_empty());
    }

    #[test]
    fn test_delta_binary_round_trip() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_entry(100));
        before.insert(KEY_B.to_vec(), make_entry(200));

        let mut after = LedgerSnapshot::new();
        after.insert(KEY_B.to_vec(), make_entry(999));
        after.insert(KEY_C.to_vec(), make_entry(300));

        let delta = DeltaSnapshot::compute(&before, &after);
        let bytes = delta.to_bytes().expect("serialization failed");
        let restored = DeltaSnapshot::from_bytes(&bytes).expect("deserialization failed");

        assert_eq!(restored.inserted.len(), 1);
        assert!(restored.inserted.contains_key(KEY_C));

        assert_eq!(restored.modified.len(), 1);
        assert!(restored.modified.contains_key(KEY_B));

        assert_eq!(restored.deleted, vec![KEY_A.to_vec()]);
    }

    #[test]
    fn test_delta_binary_apply_after_round_trip() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_entry(10));
        before.insert(KEY_B.to_vec(), make_entry(20));

        let mut after = LedgerSnapshot::new();
        after.insert(KEY_B.to_vec(), make_entry(99));
        after.insert(KEY_C.to_vec(), make_entry(30));

        let delta = DeltaSnapshot::compute(&before, &after);
        let bytes = delta.to_bytes().expect("serialization failed");
        let restored = DeltaSnapshot::from_bytes(&bytes).expect("deserialization failed");
        let reconstructed = restored.apply_to(&before);

        assert_eq!(reconstructed.len(), after.len());

        // Verify entry bytes match.
        for (key, expected_entry) in after.iter() {
            let actual = reconstructed
                .get(&key)
                .unwrap_or_else(|| panic!("missing key {key:?}"));
            let actual_bytes = actual.to_xdr(Limits::none()).unwrap();
            let expected_bytes = expected_entry.to_xdr(Limits::none()).unwrap();
            assert_eq!(
                actual_bytes, expected_bytes,
                "entry mismatch for key {key:?}"
            );
        }
    }

    #[test]
    fn test_delta_binary_rejects_unknown_version() {
        let bytes = delta_bincode_options()
            .serialize(&DeltaWireFormat {
                version: DELTA_FORMAT_VERSION + 1,
                inserted: vec![],
                modified: vec![],
                deleted: vec![],
            })
            .expect("serialization failed");

        let result = DeltaSnapshot::from_bytes(&bytes);
        assert!(matches!(
            result.unwrap_err(),
            SnapshotError::UnsupportedVersion(_)
        ));
    }

    // ------------------------------------------------------------------
    // Edge-case tests
    // ------------------------------------------------------------------

    #[test]
    fn test_delta_compute_both_empty() {
        let before = LedgerSnapshot::new();
        let after = LedgerSnapshot::new();
        let delta = DeltaSnapshot::compute(&before, &after);
        assert!(delta.is_empty());
    }

    #[test]
    fn test_delta_apply_on_non_empty_base() {
        // Verify that apply_to preserves entries from the base that
        // weren't touched by the delta.
        let mut base = LedgerSnapshot::new();
        base.insert(KEY_A.to_vec(), make_entry(1));
        base.insert(KEY_B.to_vec(), make_entry(2));

        let mut delta = DeltaSnapshot::new();
        delta.inserted.insert(KEY_C.to_vec(), make_entry(3));

        let result = delta.apply_to(&base);
        assert_eq!(result.len(), 3);
        assert!(result.get(KEY_A).is_some());
        assert!(result.get(KEY_B).is_some());
        assert!(result.get(KEY_C).is_some());
    }

    #[test]
    fn test_delta_delete_non_existent_key_is_safe() {
        let base = LedgerSnapshot::new();
        let mut delta = DeltaSnapshot::new();
        delta.deleted.push(KEY_A.to_vec());

        let result = delta.apply_to(&base);
        assert!(result.is_empty());
    }
}
