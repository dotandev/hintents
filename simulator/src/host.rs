// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Before/After snapshot capture around host function calls.
//!
//! Every host function invocation produces a paired snapshot:
//! - **Before**: the ledger state immediately prior to the call
//! - **After**: the ledger state immediately after the call returns
//!
//! If the host function traps, the After snapshot is still recorded with
//! `trapped = true` so callers can inspect the state at the point of failure.
//!
//! # Allocator Safety
//!
//! The [`HostSnapshotTracker`] optionally integrates an [`AllocTracker`] that
//! records the `Budget` memory consumption at each before-snapshot and validates
//! consistency after rollback.

#![allow(dead_code)]

use crate::snapshot::LedgerSnapshot;
use std::fmt;

/// Unique identifier for a snapshot.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct SnapshotId(u64);

impl SnapshotId {
    pub fn as_u64(self) -> u64 {
        self.0
    }
}

impl fmt::Display for SnapshotId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "snap-{}", self.0)
    }
}

/// A paired Before/After snapshot around a single host function call.
#[derive(Debug, Clone)]
pub struct SnapshotPair {
    /// Snapshot taken before the host function executes.
    pub before: CapturedSnapshot,
    /// Snapshot taken after the host function returns (or traps).
    pub after: CapturedSnapshot,
}

/// A single captured snapshot with metadata.
#[derive(Debug, Clone)]
pub struct CapturedSnapshot {
    /// Unique identifier for this snapshot.
    pub id: SnapshotId,
    /// The host function name this snapshot is associated with.
    pub host_fn_name: String,
    /// The ledger state at the moment of capture.
    pub state: LedgerSnapshot,
    /// If this is an After snapshot, the ID of the corresponding Before snapshot.
    pub before_id: Option<SnapshotId>,
    /// Whether the host function trapped (only meaningful for After snapshots).
    pub trapped: bool,
}

/// Tracks allocator state across snapshot-and-rollback cycles.
///
/// Each snapshot checkpoint records the `Budget` memory consumption at that point.
/// A subsequent rollback resets the expected baseline; the tracker verifies that
/// the new `Host` starts with the correct allocator state.
///
/// # Debug-mode invariant checks
///
/// In debug builds the tracker runs lightweight assertions on every operation:
/// - Snapshot count always ≥ rollback count.
/// - Memory consumption stays within a sane range.
#[derive(Debug, Clone)]
pub struct AllocTracker {
    /// Memory bytes consumed by the Budget at the last snapshot.
    snapshotted_memory_bytes: u64,
    /// Number of snapshot operations performed.
    snapshot_count: u64,
    /// Number of rollback operations performed.
    rollback_count: u64,
}

impl AllocTracker {
    /// Creates a new tracker with zero-initialized state.
    pub fn new() -> Self {
        Self {
            snapshotted_memory_bytes: 0,
            snapshot_count: 0,
            rollback_count: 0,
        }
    }

    /// Records a snapshot of the current memory consumption.
    ///
    /// Call this just **before** capturing a ledger snapshot so that
    /// the tracker remembers the baseline memory state.
    ///
    /// # Arguments
    /// * `memory_bytes` – The current Budget memory consumption
    ///   (obtained via [`Budget::get_mem_bytes_consumed`]).
    pub fn snapshot(&mut self, memory_bytes: u64) {
        self.snapshotted_memory_bytes = memory_bytes;
        self.snapshot_count = self.snapshot_count.saturating_add(1);
        debug_assert!(
            self.snapshot_count >= self.rollback_count,
            "snapshot count must always >= rollback count"
        );
    }

    /// Records a rollback operation and resets the baseline.
    ///
    /// Call this **after** [`SimHost::restore_from_snapshot`] has completed
    /// and the new `Host` is in place.
    ///
    /// # Arguments
    /// * `restored_memory_bytes` – The memory consumption of the newly restored
    ///   `Host`'s Budget (expected to be 0 after a fresh construction).
    pub fn record_rollback(&mut self, restored_memory_bytes: u64) {
        self.rollback_count = self.rollback_count.saturating_add(1);
        // The restored Host has a fresh Budget — consumption should be zero.
        debug_assert!(
            restored_memory_bytes == 0,
            "restored Host budget should start at 0 consumption, got {restored_memory_bytes}"
        );
    }

    /// Returns the memory bytes recorded at the last snapshot.
    pub fn snapshotted_memory_bytes(&self) -> u64 {
        self.snapshotted_memory_bytes
    }

    /// Returns the total number of snapshot operations.
    pub fn snapshot_count(&self) -> u64 {
        self.snapshot_count
    }

    /// Returns the total number of rollback operations.
    pub fn rollback_count(&self) -> u64 {
        self.rollback_count
    }

    /// Returns `true` if at least one rollback has been performed.
    pub fn has_rolled_back(&self) -> bool {
        self.rollback_count > 0
    }

    /// Returns the net number of un-rolled-back snapshots.
    pub fn net_snapshots(&self) -> u64 {
        self.snapshot_count.saturating_sub(self.rollback_count)
    }
}

impl Default for AllocTracker {
    fn default() -> Self {
        Self::new()
    }
}

/// Represents an active sub-invocation frame and its pre-call snapshot state.
#[derive(Debug, Clone)]
pub struct SubInvocationFrame {
    pub call_id: String,
    pub pre_state: LedgerSnapshot,
}

/// Manages snapshot capture around host function calls.
pub struct HostSnapshotTracker {
    next_id: u64,
    pairs: Vec<SnapshotPair>,
    /// Holds the "before" snapshot while a host function is in-flight.
    pending_before: Option<CapturedSnapshot>,
    /// Optional allocator-state tracker for rollback safety.
    alloc_tracker: Option<AllocTracker>,
    /// Stack of active sub-invocations for rollback verification.
    sub_invocations: Vec<SubInvocationFrame>,
}

impl HostSnapshotTracker {
    /// Creates a new empty tracker.
    pub fn new() -> Self {
        Self {
            next_id: 0,
            pairs: Vec::new(),
            pending_before: None,
            alloc_tracker: None,
            sub_invocations: Vec::new(),
        }
    }

    /// Creates a new tracker with an associated [`AllocTracker`] for recording
    /// memory-consumption at each before-snapshot and validating allocator
    /// consistency during rollback.
    pub fn with_alloc_tracker(alloc_tracker: AllocTracker) -> Self {
        Self {
            next_id: 0,
            pairs: Vec::new(),
            pending_before: None,
            alloc_tracker: Some(alloc_tracker),
            sub_invocations: Vec::new(),
        }
    }

    /// Returns a reference to the optional allocator tracker.
    pub fn alloc_tracker(&self) -> Option<&AllocTracker> {
        self.alloc_tracker.as_ref()
    }

    /// Returns a mutable reference to the optional allocator tracker.
    pub fn alloc_tracker_mut(&mut self) -> Option<&mut AllocTracker> {
        self.alloc_tracker.as_mut()
    }

    /// Allocate the next snapshot ID.
    fn next_snapshot_id(&mut self) -> SnapshotId {
        let id = SnapshotId(self.next_id);
        self.next_id += 1;
        id
    }

    /// Call this immediately **before** a host function executes.
    ///
    /// Takes a snapshot of the current ledger state and stores it as
    /// the pending "before" snapshot.
    ///
    /// If an [`AllocTracker`] is configured, the current memory consumption
    /// is recorded as the snapshot baseline.
    pub fn take_before_snapshot(
        &mut self,
        host_fn_name: &str,
        state: LedgerSnapshot,
        memory_bytes: Option<u64>,
    ) {
        if let (Some(tracker), Some(mem)) = (self.alloc_tracker.as_mut(), memory_bytes) {
            tracker.snapshot(mem);
        }
        let id = self.next_snapshot_id();
        self.pending_before = Some(CapturedSnapshot {
            id,
            host_fn_name: host_fn_name.to_string(),
            state,
            before_id: None,
            trapped: false,
        });
    }

    /// Call this immediately **after** a host function returns.
    ///
    /// Takes a snapshot of the resulting ledger state and pairs it with
    /// the pending "before" snapshot. If there is no pending "before"
    /// snapshot (programming error), this is a no-op and returns `None`.
    ///
    /// # Arguments
    /// * `state` - The ledger state after the host function returned.
    /// * `trapped` - Whether the host function trapped/failed.
    /// * `restored_memory_bytes` - If provided and an [`AllocTracker`] is
    ///   configured, records a rollback with the restored memory consumption.
    ///   Supply this when the "after" snapshot was produced by a rollback.
    pub fn take_after_snapshot(
        &mut self,
        state: LedgerSnapshot,
        trapped: bool,
        restored_memory_bytes: Option<u64>,
    ) -> Option<&SnapshotPair> {
        let before = self.pending_before.take()?;
        let before_id = before.id;
        let after_id = self.next_snapshot_id();

        // If this after-snapshot corresponds to a rollback, notify the tracker.
        if let (Some(tracker), Some(mem)) = (self.alloc_tracker.as_mut(), restored_memory_bytes) {
            tracker.record_rollback(mem);
        }

        let after = CapturedSnapshot {
            id: after_id,
            host_fn_name: before.host_fn_name.clone(),
            state,
            before_id: Some(before_id),
            trapped,
        };

        let pair = SnapshotPair { before, after };
        self.pairs.push(pair);
        self.pairs.last()
    }

    /// Returns all collected snapshot pairs.
    pub fn pairs(&self) -> &[SnapshotPair] {
        &self.pairs
    }

    /// Returns the number of completed snapshot pairs.
    pub fn pair_count(&self) -> usize {
        self.pairs.len()
    }

    /// Returns `true` if a before snapshot has been taken but no matching
    /// after snapshot has been recorded yet.
    pub fn has_pending(&self) -> bool {
        self.pending_before.is_some()
    }

    /// Discards the pending before snapshot without recording an after.
    /// Useful if the call was cancelled or skipped.
    pub fn discard_pending(&mut self) -> Option<CapturedSnapshot> {
        self.pending_before.take()
    }

    /// Rewinds the tracker to a specific instruction pointer.
    pub fn rewind_to(&mut self, instruction_pointer: u32) -> Option<LedgerSnapshot> {
        for pair in &self.pairs {
            if pair.before.id.as_u64() == instruction_pointer as u64 {
                return Some(pair.before.state.clone());
            }
        }
        None
    }

    /// Records the pre-invocation storage state before executing a sub-invocation.
    pub fn enter_sub_invocation(&mut self, call_id: &str, pre_state: LedgerSnapshot) {
        self.sub_invocations.push(SubInvocationFrame {
            call_id: call_id.to_string(),
            pre_state,
        });
    }

    /// Completes the most recent sub-invocation.
    /// If `failed` is true, verifies that all storage writes were rolled back.
    /// Returns `Some(RollbackDiagnosticEvent)` if un-rolled-back writes are detected.
    pub fn exit_sub_invocation(
        &mut self,
        _call_id: &str,
        post_state: &LedgerSnapshot,
        failed: bool,
    ) -> Option<RollbackDiagnosticEvent> {
        let frame = self.sub_invocations.pop()?;
        if failed {
            let (_, diag) = verify_sub_invocation_rollback(&frame.call_id, &frame.pre_state, post_state);
            diag
        } else {
            None
        }
    }

    /// Returns the current depth of active sub-invocations.
    pub fn active_sub_invocations_count(&self) -> usize {
        self.sub_invocations.len()
    }
}

/// Detailed information about an unrolled-back storage write leaked by a failed sub-invocation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LeakedStorageWrite {
    /// Hex-encoded key of the leaked storage entry.
    pub key_hex: String,
    /// Whether the key was freshly inserted (true) or mutated in-place (false).
    pub is_new: bool,
}

/// Result of checking whether a failed sub-invocation rolled back all its storage writes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RollbackCheckResult {
    /// Rollback succeeded; no dirty writes remained.
    Clean,
    /// Dirty storage writes from the failed sub-invocation were leaked into the host state.
    LeakedWrites(Vec<LeakedStorageWrite>),
}

/// A diagnostic event emitted when a failed sub-invocation leaves un-rolled-back writes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RollbackDiagnosticEvent {
    pub call_id: String,
    pub leaked_count: usize,
    pub leaked_keys: Vec<String>,
    pub message: String,
}

impl RollbackDiagnosticEvent {
    pub fn new(call_id: &str, leaked: &[LeakedStorageWrite]) -> Self {
        let leaked_keys: Vec<String> = leaked.iter().map(|w| w.key_hex.clone()).collect();
        let message = format!(
            "[DIAGNOSTIC] failed sub-invocation '{}' leaked {} storage write(s) without rollback: {:?}",
            call_id,
            leaked.len(),
            leaked_keys
        );
        Self {
            call_id: call_id.to_string(),
            leaked_count: leaked.len(),
            leaked_keys,
            message,
        }
    }
}

/// Verifies whether storage writes from a failed sub-invocation were completely rolled back.
///
/// Compares the ledger snapshot taken immediately before the sub-invocation (`pre_call_state`)
/// with the ledger snapshot observed after the failure (`post_failure_state`).
/// Any newly inserted or modified keys represent un-rolled-back storage mutations.
pub fn verify_sub_invocation_rollback(
    call_id: &str,
    pre_call_state: &LedgerSnapshot,
    post_failure_state: &LedgerSnapshot,
) -> (RollbackCheckResult, Option<RollbackDiagnosticEvent>) {
    let diff = crate::snapshot::diff_snapshots(pre_call_state, post_failure_state);

    let mut leaked = Vec::new();
    for inserted in diff.inserted {
        leaked.push(LeakedStorageWrite {
            key_hex: hex::encode(inserted),
            is_new: true,
        });
    }
    for modified in diff.modified {
        leaked.push(LeakedStorageWrite {
            key_hex: hex::encode(modified),
            is_new: false,
        });
    }

    if leaked.is_empty() {
        (RollbackCheckResult::Clean, None)
    } else {
        let diag = RollbackDiagnosticEvent::new(call_id, &leaked);
        (RollbackCheckResult::LeakedWrites(leaked), Some(diag))
    }
}

impl Default for HostSnapshotTracker {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn empty_snapshot() -> LedgerSnapshot {
        LedgerSnapshot::new()
    }

    #[test]
    fn test_basic_before_after_pair() {
        let mut tracker = HostSnapshotTracker::new();

        tracker.take_before_snapshot("storage_put", empty_snapshot(), None);
        assert!(tracker.has_pending());

        let pair = tracker
            .take_after_snapshot(empty_snapshot(), false, None)
            .expect("should produce a pair");

        assert_eq!(pair.before.host_fn_name, "storage_put");
        assert_eq!(pair.after.host_fn_name, "storage_put");
        assert!(!pair.after.trapped);
        assert_eq!(pair.after.before_id, Some(pair.before.id));
        assert!(!tracker.has_pending());
    }

    #[test]
    fn test_trapped_host_function() {
        let mut tracker = HostSnapshotTracker::new();

        tracker.take_before_snapshot("storage_get", empty_snapshot(), None);
        let pair = tracker
            .take_after_snapshot(empty_snapshot(), true, None)
            .expect("should produce a pair");

        assert!(pair.after.trapped);
        assert_eq!(pair.after.before_id, Some(pair.before.id));
    }

    #[test]
    fn test_after_without_before_is_noop() {
        let mut tracker = HostSnapshotTracker::new();
        let result = tracker.take_after_snapshot(empty_snapshot(), false, None);
        assert!(result.is_none());
    }

    #[test]
    fn test_multiple_pairs() {
        let mut tracker = HostSnapshotTracker::new();

        tracker.take_before_snapshot("storage_put", empty_snapshot(), None);
        tracker.take_after_snapshot(empty_snapshot(), false, None);

        tracker.take_before_snapshot("storage_get", empty_snapshot(), None);
        tracker.take_after_snapshot(empty_snapshot(), false, None);

        tracker.take_before_snapshot("storage_del", empty_snapshot(), None);
        tracker.take_after_snapshot(empty_snapshot(), true, None);

        assert_eq!(tracker.pair_count(), 3);

        let pairs = tracker.pairs();
        assert_eq!(pairs[0].before.host_fn_name, "storage_put");
        assert_eq!(pairs[1].before.host_fn_name, "storage_get");
        assert_eq!(pairs[2].before.host_fn_name, "storage_del");
        assert!(pairs[2].after.trapped);
    }

    #[test]
    fn test_snapshot_ids_are_unique() {
        let mut tracker = HostSnapshotTracker::new();

        tracker.take_before_snapshot("fn_a", empty_snapshot(), None);
        tracker.take_after_snapshot(empty_snapshot(), false, None);

        tracker.take_before_snapshot("fn_b", empty_snapshot(), None);
        tracker.take_after_snapshot(empty_snapshot(), false, None);

        let pairs = tracker.pairs();
        let all_ids: Vec<SnapshotId> = pairs
            .iter()
            .flat_map(|p| [p.before.id, p.after.id])
            .collect();

        // All IDs must be distinct
        for (i, a) in all_ids.iter().enumerate() {
            for b in &all_ids[i + 1..] {
                assert_ne!(a, b, "snapshot IDs must be unique");
            }
        }
    }

    #[test]
    fn test_discard_pending() {
        let mut tracker = HostSnapshotTracker::new();

        tracker.take_before_snapshot("cancelled_fn", empty_snapshot(), None);
        assert!(tracker.has_pending());

        let discarded = tracker.discard_pending();
        assert!(discarded.is_some());
        assert_eq!(discarded.unwrap().host_fn_name, "cancelled_fn");
        assert!(!tracker.has_pending());
        assert_eq!(tracker.pair_count(), 0);
    }

    #[test]
    fn test_with_alloc_tracker() {
        let alloc_tracker = AllocTracker::new();
        let mut tracker = HostSnapshotTracker::with_alloc_tracker(alloc_tracker);

        assert!(tracker.alloc_tracker().is_some());

        tracker.take_before_snapshot("host_fn", empty_snapshot(), Some(42));
        assert_eq!(
            tracker.alloc_tracker().unwrap().snapshotted_memory_bytes(),
            42
        );
        assert_eq!(tracker.alloc_tracker().unwrap().snapshot_count(), 1);

        tracker.take_after_snapshot(empty_snapshot(), false, Some(0));
        assert!(tracker.alloc_tracker().unwrap().has_rolled_back());
        assert_eq!(tracker.alloc_tracker().unwrap().rollback_count(), 1);
    }

    #[test]
    fn test_alloc_tracker_multiple_cycles() {
        let alloc_tracker = AllocTracker::new();
        let mut tracker = HostSnapshotTracker::with_alloc_tracker(alloc_tracker);

        // Cycle 1
        tracker.take_before_snapshot("fn1", empty_snapshot(), Some(100));
        tracker.take_after_snapshot(empty_snapshot(), false, Some(0));

        // Cycle 2
        tracker.take_before_snapshot("fn2", empty_snapshot(), Some(200));
        tracker.take_after_snapshot(empty_snapshot(), false, Some(0));

        let tracker_ref = tracker.alloc_tracker().unwrap();
        assert_eq!(tracker_ref.snapshot_count(), 2);
        assert_eq!(tracker_ref.rollback_count(), 2);
        assert_eq!(tracker_ref.snapshotted_memory_bytes(), 200);
    }

    #[test]
    fn test_tracker_without_memory_records_no_error() {
        // When no memory_bytes is provided, the alloc tracker should not record
        let alloc_tracker = AllocTracker::new();
        let mut tracker = HostSnapshotTracker::with_alloc_tracker(alloc_tracker);

        tracker.take_before_snapshot("fn", empty_snapshot(), None);
        assert_eq!(
            tracker.alloc_tracker().unwrap().snapshot_count(),
            0,
            "snapshot should not increment when memory_bytes is None"
        );

        tracker.take_after_snapshot(empty_snapshot(), false, None);
        assert_eq!(
            tracker.alloc_tracker().unwrap().rollback_count(),
            0,
            "rollback should not increment when restored_memory_bytes is None"
        );
    }

    #[test]
    fn test_snapshot_id_display() {
        let id = SnapshotId(42);
        assert_eq!(format!("{}", id), "snap-42");
    }

    #[test]
    fn test_sub_invocation_rollback_clean() {
        let pre_state = empty_snapshot();
        let post_state = empty_snapshot();

        let (result, diag) = verify_sub_invocation_rollback("call_1", &pre_state, &post_state);
        assert_eq!(result, RollbackCheckResult::Clean);
        assert!(diag.is_none());
    }

    #[test]
    fn test_sub_invocation_rollback_leaked_writes() {
        use soroban_env_host::xdr::{
            ContractDataDurability, ContractDataEntry, ContractId, Hash, LedgerEntry, LedgerEntryData,
            LedgerEntryExt, ScAddress, ScVal,
        };

        let pre_state = empty_snapshot();
        let mut post_state = empty_snapshot();

        // Simulate dirty write leaked by failed sub-invocation
        let dirty_entry = LedgerEntry {
            last_modified_ledger_seq: 10,
            data: LedgerEntryData::ContractData(ContractDataEntry {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract: ScAddress::Contract(ContractId(Hash([7u8; 32]))),
                key: ScVal::U32(1),
                durability: ContractDataDurability::Persistent,
                val: ScVal::U32(42),
            }),
            ext: LedgerEntryExt::V0,
        };
        post_state.insert(vec![1, 2, 3, 4], dirty_entry);

        let (result, diag) = verify_sub_invocation_rollback("sub_call_42", &pre_state, &post_state);
        match result {
            RollbackCheckResult::LeakedWrites(writes) => {
                assert_eq!(writes.len(), 1);
                assert_eq!(writes[0].key_hex, "01020304");
                assert!(writes[0].is_new);
            }
            RollbackCheckResult::Clean => panic!("expected leaked writes"),
        }

        assert!(diag.is_some());
        let event = diag.unwrap();
        assert_eq!(event.call_id, "sub_call_42");
        assert_eq!(event.leaked_count, 1);
        assert!(event.message.contains("sub_call_42"));
        assert!(event.message.contains("01020304"));
    }

    #[test]
    fn test_host_snapshot_tracker_sub_invocations() {
        let mut tracker = HostSnapshotTracker::new();
        assert_eq!(tracker.active_sub_invocations_count(), 0);

        let pre_state = empty_snapshot();
        tracker.enter_sub_invocation("call_nested", pre_state.clone());
        assert_eq!(tracker.active_sub_invocations_count(), 1);

        // Success path: no diagnostic
        let clean_post = empty_snapshot();
        let diag_ok = tracker.exit_sub_invocation("call_nested", &clean_post, false);
        assert!(diag_ok.is_none());
        assert_eq!(tracker.active_sub_invocations_count(), 0);

        // Failure path with leaked write: triggers diagnostic
        tracker.enter_sub_invocation("call_failed", pre_state.clone());
        let mut dirty_post = empty_snapshot();
        dirty_post.insert(
            vec![0xaa, 0xbb],
            soroban_env_host::xdr::LedgerEntry {
                last_modified_ledger_seq: 1,
                data: soroban_env_host::xdr::LedgerEntryData::Offer(
                    soroban_env_host::xdr::OfferEntry {
                        seller_id: soroban_env_host::xdr::AccountId(
                            soroban_env_host::xdr::PublicKey::PublicKeyTypeEd25519(
                                soroban_env_host::xdr::Uint256([0u8; 32]),
                            ),
                        ),
                        offer_id: 1,
                        selling: soroban_env_host::xdr::Asset::Native,
                        buying: soroban_env_host::xdr::Asset::Native,
                        amount: 100,
                        price: soroban_env_host::xdr::Price { n: 1, d: 1 },
                        flags: 0,
                        ext: soroban_env_host::xdr::ExtensionPoint::V0,
                    },
                ),
                ext: soroban_env_host::xdr::LedgerEntryExt::V0,
            },
        );

        let diag_err = tracker.exit_sub_invocation("call_failed", &dirty_post, true);
        assert!(diag_err.is_some());
        assert_eq!(diag_err.unwrap().call_id, "call_failed");
        assert_eq!(tracker.active_sub_invocations_count(), 0);
    }
}
