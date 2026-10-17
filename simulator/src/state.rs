// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Simulator state utilities: oversized snapshot handling and ledger state diffing.
//!
//! When simulation state exceeds a configurable threshold, the snapshot subsection
//! provides graceful degradation: full snapshot -> diff-only -> disabled.
//!
//! The diffing subsection exposes [`diff_snapshots`] for comparing ledger snapshots.

use crate::snapshot::{self, LedgerSnapshot};

/// Maximum snapshot size in bytes (10 MB).
pub const MAX_SNAPSHOT_SIZE: usize = 10 * 1024 * 1024;

/// The result of attempting to capture a frame snapshot.
#[derive(Debug, Clone, PartialEq)]
pub enum SnapshotCaptureResult {
    /// Full snapshot captured successfully.
    Full(Vec<u8>),
    /// Snapshot exceeded threshold; only a diff from the previous frame was captured.
    DiffOnly(Vec<u8>),
    /// Both full and diff snapshots exceeded threshold; capture disabled for this frame.
    Disabled,
}

/// Per-frame state used during simulation.
#[derive(Debug, Clone)]
pub struct FrameState {
    /// Raw state bytes for this frame.
    pub data: Vec<u8>,
    /// Whether this frame's snapshot was captured, degraded, or skipped.
    pub capture_result: SnapshotCaptureResult,
}

impl FrameState {
    /// Rewinds the frame state to a previously captured state.
    pub fn rewind_to(&mut self, state: &[u8]) {
        self.data.clear();
        self.data.extend_from_slice(state);
    }
}

/// Attempts to capture a snapshot for the current frame.
///
/// Strategy:
/// 1. If `current` fits within `MAX_SNAPSHOT_SIZE`, return `Full`.
/// 2. Otherwise compute a diff against `previous`. If the diff fits, return `DiffOnly`.
/// 3. Otherwise return `Disabled` — the frame is too large to snapshot.
pub fn capture_snapshot(current: &[u8], previous: Option<&[u8]>) -> SnapshotCaptureResult {
    if current.len() <= MAX_SNAPSHOT_SIZE {
        return SnapshotCaptureResult::Full(current.to_vec());
    }

    // Attempt diff-only capture
    if let Some(prev) = previous {
        let diff = compute_diff(prev, current);
        if diff.len() <= MAX_SNAPSHOT_SIZE {
            return SnapshotCaptureResult::DiffOnly(diff);
        }
    }

    SnapshotCaptureResult::Disabled
}

/// Computes a simple byte-level diff between two state buffers.
/// Returns only the changed regions as a serialized diff.
///
/// Format: repeated [offset: u64][length: u64][changed bytes]
fn compute_diff(old: &[u8], new: &[u8]) -> Vec<u8> {
    let mut diff = Vec::new();
    let max_len = old.len().max(new.len());
    let mut i = 0;

    while i < max_len {
        // Find start of changed region
        let old_byte = old.get(i).copied().unwrap_or(0);
        let new_byte = new.get(i).copied().unwrap_or(0);

        if old_byte != new_byte {
            let start = i;
            // Extend through consecutive changes
            while i < max_len {
                let ob = old.get(i).copied().unwrap_or(0);
                let nb = new.get(i).copied().unwrap_or(0);
                if ob == nb {
                    break;
                }
                i += 1;
            }
            let length = i - start;
            // Write offset
            diff.extend_from_slice(&(start as u64).to_le_bytes());
            // Write length
            diff.extend_from_slice(&(length as u64).to_le_bytes());
            // Write changed bytes from new
            for j in start..i {
                diff.push(new.get(j).copied().unwrap_or(0));
            }
        } else {
            i += 1;
        }
    }

    diff
}

/// Returns a human-readable status string for a capture result.
pub fn capture_status_message(result: &SnapshotCaptureResult) -> &'static str {
    match result {
        SnapshotCaptureResult::Full(_) => "Snapshot captured",
        SnapshotCaptureResult::DiffOnly(_) => "Snapshot oversized; diff captured instead",
        SnapshotCaptureResult::Disabled => "Frame too large to capture",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_small_snapshot_is_full() {
        let data = vec![0u8; 1024];
        match capture_snapshot(&data, None) {
            SnapshotCaptureResult::Full(d) => assert_eq!(d.len(), 1024),
            other => panic!("expected Full, got {:?}", other),
        }
    }

    #[test]
    fn test_oversized_with_small_diff_is_diff_only() {
        let prev = vec![0u8; MAX_SNAPSHOT_SIZE + 100];
        let mut current = prev.clone();
        // Change only a few bytes
        current[0] = 0xFF;
        current[1] = 0xFE;

        match capture_snapshot(&current, Some(&prev)) {
            SnapshotCaptureResult::DiffOnly(d) => {
                assert!(d.len() < MAX_SNAPSHOT_SIZE);
            }
            other => panic!("expected DiffOnly, got {:?}", other),
        }
    }

    #[test]
    fn test_oversized_with_large_diff_is_disabled() {
        let prev = vec![0u8; MAX_SNAPSHOT_SIZE + 100];
        // Completely different data — diff will be as large as full snapshot
        let current = vec![0xFFu8; MAX_SNAPSHOT_SIZE + 100];

        match capture_snapshot(&current, Some(&prev)) {
            SnapshotCaptureResult::Disabled => {}
            other => panic!("expected Disabled, got {:?}", other),
        }
    }

    #[test]
    fn test_oversized_no_previous_is_disabled() {
        let data = vec![0u8; MAX_SNAPSHOT_SIZE + 100];
        match capture_snapshot(&data, None) {
            SnapshotCaptureResult::Disabled => {}
            other => panic!("expected Disabled, got {:?}", other),
        }
    }

    #[test]
    fn test_capture_status_message() {
        assert_eq!(
            capture_status_message(&SnapshotCaptureResult::Full(vec![])),
            "Snapshot captured"
        );
        assert_eq!(
            capture_status_message(&SnapshotCaptureResult::DiffOnly(vec![])),
            "Snapshot oversized; diff captured instead"
        );
        assert_eq!(
            capture_status_message(&SnapshotCaptureResult::Disabled),
            "Frame too large to capture"
        );
    }

    #[test]
    fn test_ttl_valid_unexpired() {
        let key_hash = [0xabu8; 32];
        // live_until > replay_ledger
        assert!(check_ttl_expiry(1000, 500, &key_hash).is_ok());
        // live_until == replay_ledger
        assert!(check_ttl_expiry(500, 500, &key_hash).is_ok());
    }

    #[test]
    fn test_ttl_expired_archived_entry() {
        let key_hash = [0xcdu8; 32];
        let res = check_ttl_expiry(499, 500, &key_hash);
        assert!(res.is_err());
        match res.unwrap_err() {
            TtlExpiryError::ArchivedEntry {
                key_hash: hash_str,
                live_until_ledger,
                replay_ledger,
            } => {
                assert_eq!(hash_str, hex::encode(key_hash));
                assert_eq!(live_until_ledger, 499);
                assert_eq!(replay_ledger, 500);
            }
        }
    }

    #[test]
    fn test_validate_replay_ttl_map() {
        use std::collections::HashMap;
        let mut map = HashMap::new();
        let key1 = vec![1u8; 32];
        let key2 = vec![2u8; 32];

        map.insert(key1.clone(), 100);
        map.insert(key2.clone(), 200);

        // Sequence 90 -> all unexpired
        assert!(validate_replay_ttl_map(&map, 90).is_ok());

        // Sequence 150 -> key1 expired
        assert!(validate_replay_ttl_map(&map, 150).is_err());
    }
}

/// Represents the computed difference between two ledger snapshots,
/// with keys encoded as lowercase hex strings.
#[derive(Debug, Clone)]
pub struct StateDiff {
    /// Keys present in `after` but absent from `before` (newly inserted entries).
    pub new_keys: Vec<String>,
    /// Keys present in both snapshots but whose serialized XDR entries differ.
    pub modified_keys: Vec<String>,
    /// Keys present in `before` but absent from `after` (deleted entries).
    pub deleted_keys: Vec<String>,
}

/// Computes the diff between two ledger snapshots, returning keys as hex strings.
///
/// Delegates to [`crate::snapshot::diff_snapshots`] for the raw comparison,
/// then hex-encodes every key for human-readable output.
pub fn diff_snapshots(before: &LedgerSnapshot, after: &LedgerSnapshot) -> StateDiff {
    let raw = snapshot::diff_snapshots(before, after);
    StateDiff {
        new_keys: raw.inserted.iter().map(hex::encode).collect(),
        modified_keys: raw.modified.iter().map(hex::encode).collect(),
        deleted_keys: raw.deleted.iter().map(hex::encode).collect(),
    }
}

/// Error raised when a contract instance, code, or storage entry has expired
/// (archived) relative to the target replay ledger sequence.
#[derive(Debug, thiserror::Error, Clone, PartialEq, Eq)]
pub enum TtlExpiryError {
    #[error(
        "contract entry with key hash {key_hash} has expired: live_until_ledger {live_until_ledger} < replay_ledger {replay_ledger}"
    )]
    ArchivedEntry {
        key_hash: String,
        live_until_ledger: u32,
        replay_ledger: u32,
    },
}

/// Validates whether a contract instance, code, or storage entry is expired
/// based on its `live_until_ledger` and the target replay ledger sequence.
///
/// If `live_until_ledger < replay_ledger`, returns `TtlExpiryError::ArchivedEntry`.
pub fn check_ttl_expiry(
    live_until_ledger: u32,
    replay_ledger: u32,
    key_hash: &[u8],
) -> Result<(), TtlExpiryError> {
    if live_until_ledger < replay_ledger {
        return Err(TtlExpiryError::ArchivedEntry {
            key_hash: hex::encode(key_hash),
            live_until_ledger,
            replay_ledger,
        });
    }
    Ok(())
}

/// Validates an entry's TTL against a replay ledger sequence.
pub fn validate_entry_ttl(
    key_hash: &[u8],
    live_until_ledger_seq: u32,
    replay_ledger_seq: u32,
) -> Result<(), TtlExpiryError> {
    check_ttl_expiry(live_until_ledger_seq, replay_ledger_seq, key_hash)
}

/// Inspects a collection of TTL records and validates them against the replay ledger sequence.
/// Returns the first `TtlExpiryError` encountered if any contract entry has been archived.
pub fn validate_replay_ttl_map(
    ttl_map: &std::collections::HashMap<Vec<u8>, u32>,
    replay_ledger_seq: u32,
) -> Result<(), TtlExpiryError> {
    for (key_hash, &live_until) in ttl_map {
        check_ttl_expiry(live_until, replay_ledger_seq, key_hash)?;
    }
    Ok(())
}
