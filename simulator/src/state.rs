// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Simulator state utilities: oversized snapshot handling, ledger state diffing,
//! and TTL expiry verification for smart contract replay.
//!
//! When simulation state exceeds a configurable threshold, the snapshot subsection
//! provides graceful degradation: full snapshot -> diff-only -> disabled.
//!
//! The diffing subsection exposes [`diff_snapshots`] for comparing ledger snapshots.
//!
//! The TTL subsection simulates contract instance and code entry archival by comparing
//! entry TTLs (`live_until_ledger_seq`) against the replay ledger sequence number,
//! raising [`StateError::EntryArchived`] (matching the network's `INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED`
//! failure) when an entry has expired.

use sha2::{Digest, Sha256};
use soroban_env_host::xdr::{
    Hash, LedgerEntry, LedgerEntryData, LedgerKey, Limits, ScVal, WriteXdr,
};
use std::collections::HashMap;

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

// ============================================================================
// TTL Expiry Verification for Replay Simulation
// ============================================================================

/// The network error string produced by Stellar Core / Soroban when an archived entry is accessed.
pub const INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED: &str = "INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED";

/// Errors occurring during state manipulation, TTL verification, and ledger entry loading.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum StateError {
    /// Raised when an entry's TTL is lower than the replay ledger sequence,
    /// matching the Stellar network's `INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED` error.
    #[error("{INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED}: {entry_type} entry is archived (key_hash: {key_hash}, live_until_ledger: {live_until_ledger}, replay_sequence: {replay_sequence})")]
    EntryArchived {
        /// Human-readable description of the entry type (e.g. "contract instance", "contract code").
        entry_type: String,
        /// Hex-encoded SHA-256 hash of the target entry's `LedgerKey`.
        key_hash: String,
        /// The ledger sequence up to which the entry was live.
        live_until_ledger: u32,
        /// The ledger sequence being replayed.
        replay_sequence: u32,
    },
    /// Raised when base64 decoding or XDR parsing of a `LedgerKey` fails.
    #[error("Failed to parse LedgerKey XDR: {0}")]
    KeyDecode(String),
    /// Raised when base64 decoding or XDR parsing of a `LedgerEntry` fails.
    #[error("Failed to parse LedgerEntry XDR: {0}")]
    EntryDecode(String),
    /// Raised when injecting an entry into host storage fails.
    #[error("Failed to inject ledger entry: {0}")]
    HostInjection(String),
}

/// Computes the 32-byte SHA-256 hash of a `LedgerKey`'s XDR representation,
/// matching the `key_hash` field used in Soroban `TtlEntry`.
pub fn compute_key_hash(key: &LedgerKey) -> Result<Hash, StateError> {
    let key_xdr = key
        .to_xdr(Limits::none())
        .map_err(|e| StateError::KeyDecode(format!("Failed to serialize key to XDR: {e}")))?;
    let hash: [u8; 32] = Sha256::digest(&key_xdr).into();
    Ok(Hash(hash))
}

/// Returns `true` if the ledger key represents a Soroban smart contract instance.
#[must_use]
pub fn is_contract_instance(key: &LedgerKey) -> bool {
    matches!(
        key,
        LedgerKey::ContractData(cd) if cd.key == ScVal::LedgerKeyContractInstance
    )
}

/// Returns `true` if the ledger key represents a Soroban contract WASM bytecode entry.
#[must_use]
pub fn is_contract_code(key: &LedgerKey) -> bool {
    matches!(key, LedgerKey::ContractCode(_))
}

/// Returns `true` if the ledger key is a contract instance or contract code entry.
#[must_use]
pub fn is_contract_instance_or_code(key: &LedgerKey) -> bool {
    is_contract_instance(key) || is_contract_code(key)
}

/// Checks whether an entry's TTL is still live with respect to the replay ledger sequence.
///
/// In the Stellar network, an entry is live if `live_until_ledger >= replay_sequence`.
/// If `live_until_ledger < replay_sequence`, the entry is archived and cannot be used
/// without prior restoration.
pub fn check_ttl(
    entry_type: &str,
    key_hash: &Hash,
    live_until_ledger: u32,
    replay_sequence: u32,
) -> Result<(), StateError> {
    if live_until_ledger < replay_sequence {
        return Err(StateError::EntryArchived {
            entry_type: entry_type.to_string(),
            key_hash: hex::encode(key_hash.0),
            live_until_ledger,
            replay_sequence,
        });
    }
    Ok(())
}

/// Validates decoded ledger entries against the replay ledger sequence.
///
/// Builds a TTL lookup table from all `TtlEntry` items in `entries`, then verifies that:
/// 1. Every contract instance entry's TTL is `>= replay_sequence`.
/// 2. Every contract code entry's TTL is `>= replay_sequence`.
/// 3. Every persistent contract data entry's TTL is `>= replay_sequence`.
/// 4. Every standalone `TtlEntry` is `>= replay_sequence`.
///
/// Raises [`StateError::EntryArchived`] on the first expired entry encountered.
pub fn validate_ledger_entries_ttl(
    entries: &[(LedgerKey, LedgerEntry)],
    replay_sequence: u32,
) -> Result<(), StateError> {
    if replay_sequence == 0 {
        return Ok(());
    }

    // Step 1: Collect all TTLs from TtlEntry items in the dataset
    let mut ttl_map: HashMap<Hash, u32> = HashMap::new();
    for (key, entry) in entries {
        if let LedgerEntryData::Ttl(ref ttl) = entry.data {
            ttl_map.insert(ttl.key_hash.clone(), ttl.live_until_ledger_seq);
        }
        if let LedgerKey::Ttl(ref ttl_key) = key {
            if let LedgerEntryData::Ttl(ref ttl) = entry.data {
                ttl_map.insert(ttl_key.key_hash.clone(), ttl.live_until_ledger_seq);
            }
        }
    }

    // Step 2: Check contract instance and contract code entries first
    for (key, _entry) in entries {
        if is_contract_instance(key) {
            let key_hash = compute_key_hash(key)?;
            if let Some(&live_until) = ttl_map.get(&key_hash) {
                check_ttl("contract instance", &key_hash, live_until, replay_sequence)?;
            }
        } else if is_contract_code(key) {
            let key_hash = compute_key_hash(key)?;
            if let Some(&live_until) = ttl_map.get(&key_hash) {
                check_ttl("contract code", &key_hash, live_until, replay_sequence)?;
            }
        } else if matches!(key, LedgerKey::ContractData(_)) {
            let key_hash = compute_key_hash(key)?;
            if let Some(&live_until) = ttl_map.get(&key_hash) {
                check_ttl("contract data", &key_hash, live_until, replay_sequence)?;
            }
        }
    }

    // Step 3: Check any standalone or remaining TTL entries
    for (key, entry) in entries {
        if let LedgerEntryData::Ttl(ref ttl) = entry.data {
            check_ttl(
                "contract",
                &ttl.key_hash,
                ttl.live_until_ledger_seq,
                replay_sequence,
            )?;
        } else if let LedgerKey::Ttl(ref ttl_key) = key {
            if let Some(&live_until) = ttl_map.get(&ttl_key.key_hash) {
                check_ttl("contract", &ttl_key.key_hash, live_until, replay_sequence)?;
            }
        }
    }

    Ok(())
}

/// Decodes, validates TTL against replay ledger sequence, and injects ledger entries into `SimHost`.
///
/// When `replay_sequence` is `Some(seq)` (with `seq > 0`), compares each entry's TTL against
/// `seq` and raises [`StateError::EntryArchived`] if any contract instance or contract code entry
/// has expired.
pub fn load_ledger_entries(
    sim_host: &mut crate::runner::SimHost,
    entries: &HashMap<String, String>,
    replay_sequence: Option<u32>,
) -> Result<usize, StateError> {
    // Phase 1: Decode all entries
    let mut decoded = Vec::with_capacity(entries.len());
    for (key_xdr, entry_xdr) in entries {
        let key = crate::snapshot::decode_ledger_key(key_xdr)
            .map_err(|e| StateError::KeyDecode(e.to_string()))?;
        let entry = crate::snapshot::decode_ledger_entry(entry_xdr)
            .map_err(|e| StateError::EntryDecode(e.to_string()))?;
        decoded.push((key, entry));
    }

    // Phase 2: Validate TTLs against replay ledger sequence if specified or configured on host
    let effective_sequence = replay_sequence.or_else(|| sim_host.get_ledger_sequence());
    if let Some(seq) = effective_sequence {
        if seq > 0 {
            validate_ledger_entries_ttl(&decoded, seq)?;
        }
    }

    // Phase 3: Inject entries into host
    let mut loaded_entries = 0usize;
    for (key, entry) in decoded {
        sim_host
            .set_ledger_entry(key, entry)
            .map_err(|e| StateError::HostInjection(format!("{e:?}")))?;
        loaded_entries += 1;
    }

    Ok(loaded_entries)
}

#[cfg(test)]
mod ttl_tests {
    use super::*;
    use base64::engine::general_purpose::STANDARD;
    use base64::Engine;
    use soroban_env_host::xdr::{
        AccountEntry, AccountId, ContractCodeEntry, ContractCodeEntryExt, ContractDataDurability,
        ContractDataEntry, ContractId, ExtensionPoint, LedgerEntryExt, LedgerKeyAccount,
        LedgerKeyContractCode, LedgerKeyContractData, LedgerKeyTtl, PublicKey, ScAddress,
        SequenceNumber, Thresholds, TtlEntry, Uint256,
    };

    fn make_test_instance_key(contract_id_bytes: [u8; 32]) -> LedgerKey {
        LedgerKey::ContractData(LedgerKeyContractData {
            contract: ScAddress::Contract(ContractId(Hash(contract_id_bytes))),
            key: ScVal::LedgerKeyContractInstance,
            durability: ContractDataDurability::Persistent,
        })
    }

    fn make_test_instance_entry(contract_id_bytes: [u8; 32]) -> LedgerEntry {
        LedgerEntry {
            last_modified_ledger_seq: 100,
            data: LedgerEntryData::ContractData(ContractDataEntry {
                ext: ExtensionPoint::V0,
                contract: ScAddress::Contract(ContractId(Hash(contract_id_bytes))),
                key: ScVal::LedgerKeyContractInstance,
                durability: ContractDataDurability::Persistent,
                val: ScVal::U32(1),
            }),
            ext: LedgerEntryExt::V0,
        }
    }

    fn make_test_code_key(wasm_hash: [u8; 32]) -> LedgerKey {
        LedgerKey::ContractCode(LedgerKeyContractCode {
            hash: Hash(wasm_hash),
        })
    }

    fn make_test_code_entry(wasm_hash: [u8; 32]) -> LedgerEntry {
        LedgerEntry {
            last_modified_ledger_seq: 100,
            data: LedgerEntryData::ContractCode(ContractCodeEntry {
                ext: ContractCodeEntryExt::V0,
                hash: Hash(wasm_hash),
                code: vec![0, 0x61, 0x73, 0x6d].try_into().unwrap(),
            }),
            ext: LedgerEntryExt::V0,
        }
    }

    fn make_ttl_pair_for_key(key: &LedgerKey, live_until: u32) -> (LedgerKey, LedgerEntry) {
        let key_hash = compute_key_hash(key).unwrap();
        let ttl_key = LedgerKey::Ttl(LedgerKeyTtl {
            key_hash: key_hash.clone(),
        });
        let ttl_entry = LedgerEntry {
            last_modified_ledger_seq: 100,
            data: LedgerEntryData::Ttl(TtlEntry {
                key_hash,
                live_until_ledger_seq: live_until,
            }),
            ext: LedgerEntryExt::V0,
        };
        (ttl_key, ttl_entry)
    }

    #[test]
    fn test_is_contract_instance() {
        let instance_key = make_test_instance_key([1u8; 32]);
        assert!(is_contract_instance(&instance_key));
        assert!(is_contract_instance_or_code(&instance_key));

        let non_instance_key = LedgerKey::ContractData(LedgerKeyContractData {
            contract: ScAddress::Contract(ContractId(Hash([1u8; 32]))),
            key: ScVal::U32(42),
            durability: ContractDataDurability::Persistent,
        });
        assert!(!is_contract_instance(&non_instance_key));
    }

    #[test]
    fn test_is_contract_code() {
        let code_key = make_test_code_key([2u8; 32]);
        assert!(is_contract_code(&code_key));
        assert!(is_contract_instance_or_code(&code_key));

        let instance_key = make_test_instance_key([2u8; 32]);
        assert!(!is_contract_code(&instance_key));
    }

    #[test]
    fn test_compute_key_hash_matches_sha256() {
        let key = make_test_instance_key([3u8; 32]);
        let hash = compute_key_hash(&key).unwrap();
        let expected_xdr = key.to_xdr(Limits::none()).unwrap();
        let expected_hash: [u8; 32] = Sha256::digest(&expected_xdr).into();
        assert_eq!(hash.0, expected_hash);
    }

    #[test]
    fn test_check_ttl_live_boundary() {
        let hash = Hash([4u8; 32]);
        // live_until == replay_sequence: entry is STILL LIVE
        assert!(check_ttl("contract instance", &hash, 100, 100).is_ok());

        // live_until > replay_sequence: entry is LIVE
        assert!(check_ttl("contract instance", &hash, 101, 100).is_ok());

        // live_until < replay_sequence: entry is ARCHIVED
        let err = check_ttl("contract instance", &hash, 99, 100).unwrap_err();
        match err {
            StateError::EntryArchived {
                entry_type,
                key_hash,
                live_until_ledger,
                replay_sequence,
            } => {
                assert_eq!(entry_type, "contract instance");
                assert_eq!(key_hash, hex::encode(hash.0));
                assert_eq!(live_until_ledger, 99);
                assert_eq!(replay_sequence, 100);
            }
            _ => panic!("Expected EntryArchived error"),
        }
    }

    #[test]
    fn test_contract_instance_expired_fails() {
        let instance_key = make_test_instance_key([5u8; 32]);
        let instance_entry = make_test_instance_entry([5u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&instance_key, 150);

        let entries = vec![(instance_key, instance_entry), (ttl_key, ttl_entry)];

        // Replaying at sequence 200 > 150 (expired)
        let result = validate_ledger_entries_ttl(&entries, 200);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(err
            .to_string()
            .contains(INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED));
        assert!(err
            .to_string()
            .contains("contract instance entry is archived"));
    }

    #[test]
    fn test_contract_instance_live_succeeds() {
        let instance_key = make_test_instance_key([6u8; 32]);
        let instance_entry = make_test_instance_entry([6u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&instance_key, 250);

        let entries = vec![(instance_key, instance_entry), (ttl_key, ttl_entry)];

        // Replaying at sequence 200 <= 250 (live)
        let result = validate_ledger_entries_ttl(&entries, 200);
        assert!(result.is_ok());
    }

    #[test]
    fn test_contract_code_expired_fails() {
        let code_key = make_test_code_key([7u8; 32]);
        let code_entry = make_test_code_entry([7u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&code_key, 120);

        let entries = vec![(code_key, code_entry), (ttl_key, ttl_entry)];

        // Replaying at sequence 150 > 120 (expired)
        let result = validate_ledger_entries_ttl(&entries, 150);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(err
            .to_string()
            .contains(INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED));
        assert!(err.to_string().contains("contract code entry is archived"));
    }

    #[test]
    fn test_contract_code_live_succeeds() {
        let code_key = make_test_code_key([8u8; 32]);
        let code_entry = make_test_code_entry([8u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&code_key, 300);

        let entries = vec![(code_key, code_entry), (ttl_key, ttl_entry)];

        // Replaying at sequence 300 (exact boundary: live)
        let result = validate_ledger_entries_ttl(&entries, 300);
        assert!(result.is_ok());
    }

    #[test]
    fn test_standalone_ttl_entry_expired_fails() {
        let target_key = make_test_instance_key([9u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&target_key, 100);

        let entries = vec![(ttl_key, ttl_entry)];

        let result = validate_ledger_entries_ttl(&entries, 101);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(err
            .to_string()
            .contains(INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED));
    }

    #[test]
    fn test_standalone_ttl_entry_live_succeeds() {
        let target_key = make_test_instance_key([10u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&target_key, 100);

        let entries = vec![(ttl_key, ttl_entry)];

        let result = validate_ledger_entries_ttl(&entries, 100);
        assert!(result.is_ok());
    }

    #[test]
    fn test_contract_without_ttl_entry_does_not_fail() {
        // If a snapshot does not include TTL entries (e.g. legacy/mock snapshots),
        // TTL validation does not reject the transaction.
        let instance_key = make_test_instance_key([11u8; 32]);
        let instance_entry = make_test_instance_entry([11u8; 32]);

        let entries = vec![(instance_key, instance_entry)];
        let result = validate_ledger_entries_ttl(&entries, 500);
        assert!(result.is_ok());
    }

    #[test]
    fn test_zero_replay_sequence_skips_validation() {
        let instance_key = make_test_instance_key([12u8; 32]);
        let instance_entry = make_test_instance_entry([12u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&instance_key, 50);

        let entries = vec![(instance_key, instance_entry), (ttl_key, ttl_entry)];

        // Sequence 0 indicates uninitialized / no check
        let result = validate_ledger_entries_ttl(&entries, 0);
        assert!(result.is_ok());
    }

    #[test]
    fn test_multiple_entries_one_expired_fails() {
        let instance_key = make_test_instance_key([13u8; 32]);
        let instance_entry = make_test_instance_entry([13u8; 32]);
        let (ttl_inst_key, ttl_inst_entry) = make_ttl_pair_for_key(&instance_key, 500); // live

        let code_key = make_test_code_key([14u8; 32]);
        let code_entry = make_test_code_entry([14u8; 32]);
        let (ttl_code_key, ttl_code_entry) = make_ttl_pair_for_key(&code_key, 250); // expired at 300

        let entries = vec![
            (instance_key, instance_entry),
            (ttl_inst_key, ttl_inst_entry),
            (code_key, code_entry),
            (ttl_code_key, ttl_code_entry),
        ];

        let result = validate_ledger_entries_ttl(&entries, 300);
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(err.to_string().contains("contract code entry is archived"));
    }

    #[test]
    fn test_non_soroban_entries_not_checked_by_ttl() {
        let account_id = AccountId(PublicKey::PublicKeyTypeEd25519(Uint256([15u8; 32])));
        let acc_key = LedgerKey::Account(LedgerKeyAccount {
            account_id: account_id.clone(),
        });
        let acc_entry = LedgerEntry {
            last_modified_ledger_seq: 100,
            data: LedgerEntryData::Account(AccountEntry {
                account_id,
                balance: 1000,
                seq_num: SequenceNumber(1),
                num_sub_entries: 0,
                inflation_dest: None,
                flags: 0,
                home_domain: Default::default(),
                thresholds: Thresholds([0; 4]),
                signers: vec![].try_into().unwrap(),
                ext: Default::default(),
            }),
            ext: LedgerEntryExt::V0,
        };

        let entries = vec![(acc_key, acc_entry)];
        let result = validate_ledger_entries_ttl(&entries, 1000);
        assert!(
            result.is_ok(),
            "Account entries should not be subject to TTL expiry"
        );
    }

    #[test]
    fn test_load_ledger_entries_end_to_end_success() {
        let instance_key = make_test_instance_key([16u8; 32]);
        let instance_entry = make_test_instance_entry([16u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&instance_key, 500);

        let mut map = HashMap::new();
        map.insert(
            STANDARD.encode(instance_key.to_xdr(Limits::none()).unwrap()),
            STANDARD.encode(instance_entry.to_xdr(Limits::none()).unwrap()),
        );
        map.insert(
            STANDARD.encode(ttl_key.to_xdr(Limits::none()).unwrap()),
            STANDARD.encode(ttl_entry.to_xdr(Limits::none()).unwrap()),
        );

        let mut host = crate::runner::SimHost::new(crate::runner::HostConfig::default());
        let count =
            load_ledger_entries(&mut host, &map, Some(300)).expect("loading should succeed");
        assert_eq!(count, 2);
    }

    #[test]
    fn test_load_ledger_entries_end_to_end_expired_raises_error() {
        let instance_key = make_test_instance_key([17u8; 32]);
        let instance_entry = make_test_instance_entry([17u8; 32]);
        let (ttl_key, ttl_entry) = make_ttl_pair_for_key(&instance_key, 250); // expired at 300

        let mut map = HashMap::new();
        map.insert(
            STANDARD.encode(instance_key.to_xdr(Limits::none()).unwrap()),
            STANDARD.encode(instance_entry.to_xdr(Limits::none()).unwrap()),
        );
        map.insert(
            STANDARD.encode(ttl_key.to_xdr(Limits::none()).unwrap()),
            STANDARD.encode(ttl_entry.to_xdr(Limits::none()).unwrap()),
        );

        let mut host = crate::runner::SimHost::new(crate::runner::HostConfig::default());
        let result = load_ledger_entries(&mut host, &map, Some(300));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert!(err
            .to_string()
            .contains(INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED));
        assert!(err
            .to_string()
            .contains("contract instance entry is archived"));
    }
}
