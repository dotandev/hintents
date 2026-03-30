// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Unit tests for [`crate::snapshot::diff_snapshots`] and
//! [`crate::state::diff_snapshots`].
//!
//! Covers every branch of the diffing logic:
//! - Keys inserted (present in `after` but not `before`)
//! - Keys modified (present in both with differing XDR bytes)
//! - Keys deleted (present in `before` but not `after`)
//! - Keys unchanged (present in both with identical XDR bytes)

#[cfg(test)]
mod tests {
    use crate::snapshot::{diff_snapshots, LedgerSnapshot};
    use soroban_env_host::xdr::{
        AccountEntry, AccountId, LedgerEntry, LedgerEntryData, PublicKey, SequenceNumber,
        Thresholds, Uint256,
    };

    // ------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------

    /// Builds a minimal account `LedgerEntry` distinguished by `balance`.
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

    // Stable key constants used across multiple tests.
    const KEY_DELETED: &[u8] = &[1, 2, 3];
    const KEY_MODIFIED: &[u8] = &[4, 5, 6];
    const KEY_INSERTED: &[u8] = &[7, 8, 9];
    const KEY_UNCHANGED: &[u8] = &[10, 11, 12];

    /// Returns a `(before, after)` snapshot pair that exercises all four
    /// branches of the diffing logic simultaneously.
    fn mixed_snapshots() -> (LedgerSnapshot, LedgerSnapshot) {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_DELETED.to_vec(), make_entry(100));
        before.insert(KEY_MODIFIED.to_vec(), make_entry(200));
        before.insert(KEY_UNCHANGED.to_vec(), make_entry(400));

        let mut after = LedgerSnapshot::new();
        after.insert(KEY_MODIFIED.to_vec(), make_entry(999)); // balance changed → modified
        after.insert(KEY_INSERTED.to_vec(), make_entry(300)); // new key → inserted
        after.insert(KEY_UNCHANGED.to_vec(), make_entry(400)); // same balance → unchanged

        (before, after)
    }

    // ------------------------------------------------------------------
    // Individual branch tests
    // ------------------------------------------------------------------

    #[test]
    fn test_diff_detects_inserted_key() {
        let (before, after) = mixed_snapshots();
        let diff = diff_snapshots(&before, &after);
        assert_eq!(diff.inserted, vec![KEY_INSERTED.to_vec()]);
    }

    #[test]
    fn test_diff_detects_deleted_key() {
        let (before, after) = mixed_snapshots();
        let diff = diff_snapshots(&before, &after);
        assert_eq!(diff.deleted, vec![KEY_DELETED.to_vec()]);
    }

    #[test]
    fn test_diff_detects_modified_key() {
        let (before, after) = mixed_snapshots();
        let diff = diff_snapshots(&before, &after);
        assert_eq!(diff.modified, vec![KEY_MODIFIED.to_vec()]);
    }

    #[test]
    fn test_diff_unchanged_key_absent_from_all_lists() {
        let (before, after) = mixed_snapshots();
        let diff = diff_snapshots(&before, &after);
        let unchanged = KEY_UNCHANGED.to_vec();
        assert!(
            !diff.inserted.contains(&unchanged),
            "unchanged key must not appear in `inserted`"
        );
        assert!(
            !diff.modified.contains(&unchanged),
            "unchanged key must not appear in `modified`"
        );
        assert!(
            !diff.deleted.contains(&unchanged),
            "unchanged key must not appear in `deleted`"
        );
    }

    // ------------------------------------------------------------------
    // Edge-case / full-coverage tests
    // ------------------------------------------------------------------

    #[test]
    fn test_diff_both_empty_snapshots() {
        let diff = diff_snapshots(&LedgerSnapshot::new(), &LedgerSnapshot::new());
        assert!(diff.inserted.is_empty());
        assert!(diff.modified.is_empty());
        assert!(diff.deleted.is_empty());
    }

    #[test]
    fn test_diff_all_keys_inserted() {
        let before = LedgerSnapshot::new();
        let mut after = LedgerSnapshot::new();
        after.insert(vec![1], make_entry(10));
        after.insert(vec![2], make_entry(20));

        let diff = diff_snapshots(&before, &after);

        assert_eq!(diff.inserted.len(), 2);
        assert!(diff.modified.is_empty());
        assert!(diff.deleted.is_empty());
    }

    #[test]
    fn test_diff_all_keys_deleted() {
        let mut before = LedgerSnapshot::new();
        before.insert(vec![1], make_entry(10));
        before.insert(vec![2], make_entry(20));
        let after = LedgerSnapshot::new();

        let diff = diff_snapshots(&before, &after);

        assert!(diff.inserted.is_empty());
        assert!(diff.modified.is_empty());
        assert_eq!(diff.deleted.len(), 2);
    }

    #[test]
    fn test_diff_all_keys_modified() {
        let key1 = vec![1u8];
        let key2 = vec![2u8];

        let mut before = LedgerSnapshot::new();
        before.insert(key1.clone(), make_entry(1));
        before.insert(key2.clone(), make_entry(2));

        let mut after = LedgerSnapshot::new();
        after.insert(key1.clone(), make_entry(100));
        after.insert(key2.clone(), make_entry(200));

        let diff = diff_snapshots(&before, &after);

        assert!(diff.inserted.is_empty());
        assert_eq!(diff.modified.len(), 2);
        assert!(diff.deleted.is_empty());
    }

    #[test]
    fn test_diff_result_is_sorted() {
        // Insert keys in reverse order so HashMap ordering cannot accidentally
        // produce sorted output without the explicit sort in diff_snapshots.
        let keys: Vec<Vec<u8>> = (1u8..=5).rev().map(|b| vec![b]).collect();

        let before = LedgerSnapshot::new();
        let mut after = LedgerSnapshot::new();
        for (i, key) in keys.iter().enumerate() {
            after.insert(key.clone(), make_entry(i as i64));
        }

        let diff = diff_snapshots(&before, &after);

        let mut expected = diff.inserted.clone();
        expected.sort_unstable();
        assert_eq!(diff.inserted, expected, "`inserted` must be sorted");
    }
}

// ------------------------------------------------------------------
// Tests for state::diff_snapshots (human-readable hex output)
// ------------------------------------------------------------------

#[cfg(test)]
mod state_diff_tests {
    use crate::snapshot::LedgerSnapshot;
    use crate::state::diff_snapshots as state_diff;
    use soroban_env_host::xdr::{
        AccountEntry, AccountId, LedgerEntry, LedgerEntryData, PublicKey, SequenceNumber,
        Thresholds, Uint256,
    };

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

    #[test]
    fn test_state_diff_keys_are_hex_strings() {
        let key = vec![0xde, 0xad, 0xbe, 0xef];

        let before = LedgerSnapshot::new();
        let mut after = LedgerSnapshot::new();
        after.insert(key.clone(), make_entry(1));

        let diff = state_diff(&before, &after);

        assert_eq!(diff.new_keys, vec!["deadbeef".to_string()]);
        assert!(diff.modified_keys.is_empty());
        assert!(diff.deleted_keys.is_empty());
    }

    #[test]
    fn test_state_diff_detects_new_keys() {
        let before = LedgerSnapshot::new();
        let mut after = LedgerSnapshot::new();
        after.insert(vec![1, 2, 3], make_entry(10));

        let diff = state_diff(&before, &after);

        assert_eq!(diff.new_keys, vec![hex::encode(vec![1, 2, 3])]);
        assert!(diff.modified_keys.is_empty());
        assert!(diff.deleted_keys.is_empty());
    }

    #[test]
    fn test_state_diff_detects_deleted_keys() {
        let mut before = LedgerSnapshot::new();
        before.insert(vec![4, 5, 6], make_entry(20));
        let after = LedgerSnapshot::new();

        let diff = state_diff(&before, &after);

        assert!(diff.new_keys.is_empty());
        assert!(diff.modified_keys.is_empty());
        assert_eq!(diff.deleted_keys, vec![hex::encode(vec![4, 5, 6])]);
    }

    #[test]
    fn test_state_diff_detects_modified_keys() {
        let key = vec![7, 8, 9];
        let mut before = LedgerSnapshot::new();
        before.insert(key.clone(), make_entry(100));
        let mut after = LedgerSnapshot::new();
        after.insert(key.clone(), make_entry(999));

        let diff = state_diff(&before, &after);

        assert!(diff.new_keys.is_empty());
        assert_eq!(diff.modified_keys, vec![hex::encode(key)]);
        assert!(diff.deleted_keys.is_empty());
    }

    #[test]
    fn test_state_diff_unchanged_key_absent_from_all_lists() {
        let key = vec![10, 11, 12];
        let entry = make_entry(400);
        let mut before = LedgerSnapshot::new();
        before.insert(key.clone(), entry.clone());
        let mut after = LedgerSnapshot::new();
        after.insert(key.clone(), entry);

        let diff = state_diff(&before, &after);

        let hex_key = hex::encode(&key);
        assert!(
            !diff.new_keys.contains(&hex_key),
            "unchanged key must not appear in `new_keys`"
        );
        assert!(
            !diff.modified_keys.contains(&hex_key),
            "unchanged key must not appear in `modified_keys`"
        );
        assert!(
            !diff.deleted_keys.contains(&hex_key),
            "unchanged key must not appear in `deleted_keys`"
        );
    }

    #[test]
    fn test_state_diff_both_empty() {
        let diff = state_diff(&LedgerSnapshot::new(), &LedgerSnapshot::new());
        assert!(diff.new_keys.is_empty());
        assert!(diff.modified_keys.is_empty());
        assert!(diff.deleted_keys.is_empty());
    }

    #[test]
    fn test_state_diff_all_lists_mixed() {
        let key_del = vec![1u8];
        let key_mod = vec![2u8];
        let key_new = vec![3u8];
        let key_same = vec![4u8];

        let mut before = LedgerSnapshot::new();
        before.insert(key_del.clone(), make_entry(10));
        before.insert(key_mod.clone(), make_entry(20));
        before.insert(key_same.clone(), make_entry(40));

        let mut after = LedgerSnapshot::new();
        after.insert(key_mod.clone(), make_entry(99));
        after.insert(key_new.clone(), make_entry(30));
        after.insert(key_same.clone(), make_entry(40));

        let diff = state_diff(&before, &after);

        assert_eq!(diff.new_keys, vec![hex::encode(&key_new)]);
        assert_eq!(diff.modified_keys, vec![hex::encode(&key_mod)]);
        assert_eq!(diff.deleted_keys, vec![hex::encode(&key_del)]);
    }
}
