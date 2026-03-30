// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! State diffing utilities for Soroban ledger snapshots.
//!
//! This module exposes [`diff_snapshots`], which compares two [`LedgerSnapshot`]
//! values and returns a [`StateDiff`] whose key vectors are encoded as
//! human-readable lowercase hex strings.

use crate::snapshot::{self, LedgerSnapshot};

/// Represents the computed difference between two ledger snapshots.
///
/// All key vectors contain lowercase hex strings derived from the raw XDR
/// key bytes, making them easy to log, display, and compare.
#[derive(Debug, Clone)]
pub struct StateDiff {
    /// Keys present in `after` but absent from `before` (newly inserted entries).
    pub new_keys: Vec<String>,
    /// Keys present in both snapshots but whose serialized XDR entries differ.
    pub modified_keys: Vec<String>,
    /// Keys present in `before` but absent from `after` (deleted entries).
    pub deleted_keys: Vec<String>,
}

/// Computes the diff between two ledger snapshots.
///
/// Internally delegates to [`crate::snapshot::diff_snapshots`] for the raw
/// byte-level comparison, then converts every key to a lowercase hex string
/// for human-readable output.
///
/// The key lists in the returned [`StateDiff`] are sorted lexicographically
/// (inherited from the underlying implementation) so callers receive
/// deterministic output regardless of [`HashMap`] iteration order.
///
/// # Arguments
/// * `before` – Snapshot of ledger state before the transaction.
/// * `after`  – Snapshot of ledger state after the transaction.
///
/// # Example
/// ```ignore
/// let diff = diff_snapshots(&before, &after);
/// println!("inserted: {:?}", diff.new_keys);
/// println!("modified: {:?}", diff.modified_keys);
/// println!("deleted:  {:?}", diff.deleted_keys);
/// ```
pub fn diff_snapshots(before: &LedgerSnapshot, after: &LedgerSnapshot) -> StateDiff {
    let raw = snapshot::diff_snapshots(before, after);

    StateDiff {
        new_keys: raw.inserted.iter().map(hex::encode).collect(),
        modified_keys: raw.modified.iter().map(hex::encode).collect(),
        deleted_keys: raw.deleted.iter().map(hex::encode).collect(),
    }
}
