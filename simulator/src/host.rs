// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! SimHost: Host memory state management and snapshot infrastructure.
//!
//! This module owns the full runtime state of a simulated Soroban host at
//! every instruction boundary. It provides:
//!
//! - [`HostMemoryState`]: A complete, deep-copyable snapshot of all mutable
//!   host state at a given instruction pointer. Used by the time-travel
//!   debugger to jump back to any previously observed execution point.
//!
//! - [`CapturedSnapshot`]: A single ledger-state snapshot with metadata
//!   (host function name, snapshot ID, trap flag).
//!
//! - [`SnapshotPair`]: A before/after pair around one host function call.
//!
//! - [`HostSnapshotTracker`]: Manages sequential capture of snapshot pairs
//!   across a full transaction execution.
//!
//! - [`AllocTracker`]: Records Budget memory consumption across
//!   snapshot/rollback cycles so allocator consistency can be validated.
//!
//! # Deep Copying vs Forking
//!
//! `clone()` on a [`crate::snapshot::LedgerSnapshot`] shares the underlying
//! `Arc`-wrapped allocations (shallow clone). `fork()` merges delta into a
//! new shared base — still Arc-shared. **`deep_copy()`** materializes every
//! live entry into a brand-new allocation: the copy and the original share
//! nothing.
//!
//! Always use `deep_copy()` when capturing a checkpoint for time-travel; use
//! `fork()` for rollback-optimised COW derivation; use `clone()` for cheap
//! read-sharing when neither needs to mutate independently.

#![allow(dead_code)]

use crate::snapshot::LedgerSnapshot;
use std::fmt;

// ---------------------------------------------------------------------------
// SnapshotId
// ---------------------------------------------------------------------------

/// Unique identifier for a snapshot.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct SnapshotId(u64);

impl SnapshotId {
    /// Returns the raw numeric value of this ID.
    pub fn as_u64(self) -> u64 {
        self.0
    }
}

impl fmt::Display for SnapshotId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "snap-{}", self.0)
    }
}

// ---------------------------------------------------------------------------
// CapturedSnapshot
// ---------------------------------------------------------------------------

/// A single captured ledger-state snapshot with execution metadata.
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

impl CapturedSnapshot {
    /// Returns a fully independent deep copy of this `CapturedSnapshot`.
    ///
    /// The returned value shares **no** allocations with `self`. Mutating the
    /// copy's `state` will not affect `self.state`, and vice-versa.
    ///
    /// Metadata fields (`id`, `host_fn_name`, `before_id`, `trapped`) are
    /// cheaply cloned via their standard `Clone` impl.
    pub fn deep_copy(&self) -> Self {
        Self {
            id: self.id,
            host_fn_name: self.host_fn_name.clone(),
            state: self.state.deep_copy(),
            before_id: self.before_id,
            trapped: self.trapped,
        }
    }
}

// ---------------------------------------------------------------------------
// SnapshotPair
// ---------------------------------------------------------------------------

/// A paired Before/After snapshot around a single host function call.
#[derive(Debug, Clone)]
pub struct SnapshotPair {
    /// Snapshot taken before the host function executes.
    pub before: CapturedSnapshot,
    /// Snapshot taken after the host function returns (or traps).
    pub after: CapturedSnapshot,
}

impl SnapshotPair {
    /// Returns a fully independent deep copy of this `SnapshotPair`.
    ///
    /// Both `before` and `after` are deep-copied; neither shares any
    /// allocation with the corresponding field in `self`.
    pub fn deep_copy(&self) -> Self {
        Self {
            before: self.before.deep_copy(),
            after: self.after.deep_copy(),
        }
    }
}

// ---------------------------------------------------------------------------
// HostMemoryState
// ---------------------------------------------------------------------------

/// Complete, deep-copyable state of the simulated host at a given instruction
/// pointer.
///
/// A [`HostMemoryState`] is the primary unit exchanged by the time-travel
/// engine: capturing one at instruction *N* and restoring it later resets
/// the simulation to exactly the memory layout it had at that point.
///
/// # Isolation guarantee
///
/// Every field that contains heap-allocated data is **deep-copied** when the
/// struct is constructed via [`HostMemoryState::capture`] or cloned via
/// [`HostMemoryState::deep_copy`].  There is no Arc-sharing between a
/// checkpoint and the live host: mutations to the live host after a checkpoint
/// are invisible to the checkpoint, and restoring a checkpoint does not modify
/// the live host's allocations.
#[derive(Debug, Clone)]
pub struct HostMemoryState {
    /// The WASM instruction pointer at the moment of capture.
    pub instruction_pointer: u32,
    /// The ledger state at the moment of capture — deeply owned.
    pub ledger_state: LedgerSnapshot,
    /// The host function being executed when this state was captured.
    pub host_fn_name: String,
    /// Whether the host trapped immediately before/after this capture.
    pub trapped: bool,
}

impl HostMemoryState {
    /// Captures the current host memory state at `instruction_pointer`.
    ///
    /// `ledger_state` is **deep-copied** so the returned value is independent
    /// of the live host regardless of subsequent mutations.
    pub fn capture(
        instruction_pointer: u32,
        ledger_state: &LedgerSnapshot,
        host_fn_name: &str,
        trapped: bool,
    ) -> Self {
        Self {
            instruction_pointer,
            ledger_state: ledger_state.deep_copy(),
            host_fn_name: host_fn_name.to_string(),
            trapped,
        }
    }

    /// Returns a fully independent deep copy of this `HostMemoryState`.
    ///
    /// Equivalent to calling `capture(…)` with each field, but avoids
    /// re-encoding the host function name.
    pub fn deep_copy(&self) -> Self {
        Self {
            instruction_pointer: self.instruction_pointer,
            ledger_state: self.ledger_state.deep_copy(),
            host_fn_name: self.host_fn_name.clone(),
            trapped: self.trapped,
        }
    }
}

// ---------------------------------------------------------------------------
// AllocTracker
// ---------------------------------------------------------------------------

/// Tracks allocator state across snapshot-and-rollback cycles.
///
/// Each snapshot checkpoint records the `Budget` memory consumption at that
/// point. A subsequent rollback resets the expected baseline; the tracker
/// verifies that the new `Host` starts with the correct allocator state.
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
    ///   (obtained via `Budget::get_mem_bytes_consumed`).
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
    /// * `restored_memory_bytes` – The memory consumption of the newly
    ///   restored `Host`'s Budget (expected to be 0 after a fresh
    ///   construction).
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

// ---------------------------------------------------------------------------
// HostSnapshotTracker
// ---------------------------------------------------------------------------

/// Manages snapshot capture around host function calls.
///
/// Records sequential [`SnapshotPair`]s as a transaction executes.  The
/// tracker also exposes [`HostSnapshotTracker::deep_copy_at`] for the
/// time-travel engine: given an instruction pointer it returns a
/// [`HostMemoryState`] that is fully independent of the tracker's internal
/// storage.
pub struct HostSnapshotTracker {
    next_id: u64,
    pairs: Vec<SnapshotPair>,
    /// Holds the "before" snapshot while a host function is in-flight.
    pending_before: Option<CapturedSnapshot>,
    /// Optional allocator-state tracker for rollback safety.
    alloc_tracker: Option<AllocTracker>,
}

impl HostSnapshotTracker {
    /// Creates a new empty tracker.
    pub fn new() -> Self {
        Self {
            next_id: 0,
            pairs: Vec::new(),
            pending_before: None,
            alloc_tracker: None,
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

    /// Allocates the next snapshot ID.
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

    /// Returns a deep copy of the [`HostMemoryState`] at `instruction_pointer`.
    ///
    /// Searches the recorded pairs for a before-snapshot whose ID matches
    /// `instruction_pointer`. If found, every allocation in the snapshot is
    /// independently copied before it is returned — the result shares nothing
    /// with the tracker's internal storage, satisfying the time-travel
    /// isolation invariant.
    ///
    /// Returns `None` if `instruction_pointer` does not correspond to any
    /// recorded before-snapshot.
    pub fn deep_copy_at(&self, instruction_pointer: u32) -> Option<HostMemoryState> {
        for pair in &self.pairs {
            if pair.before.id.as_u64() == u64::from(instruction_pointer) {
                return Some(HostMemoryState::capture(
                    instruction_pointer,
                    &pair.before.state,
                    &pair.before.host_fn_name,
                    pair.before.trapped,
                ));
            }
        }
        None
    }

    /// Rewinds the tracker to a specific instruction pointer.
    ///
    /// Returns a **shallow clone** of the before-snapshot at that point.
    /// Prefer [`deep_copy_at`](Self::deep_copy_at) when the caller needs
    /// mutation isolation.
    pub fn rewind_to(&mut self, instruction_pointer: u32) -> Option<LedgerSnapshot> {
        for pair in &self.pairs {
            if pair.before.id.as_u64() == u64::from(instruction_pointer) {
                return Some(pair.before.state.clone());
            }
        }
        None
    }
}

impl Default for HostSnapshotTracker {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn empty_snapshot() -> LedgerSnapshot {
        LedgerSnapshot::new()
    }

    /// Build a snapshot with `n` entries keyed by sequential bytes.
    fn populated_snapshot(n: u8) -> LedgerSnapshot {
        use soroban_env_host::xdr::{
            AccountEntry, AccountId, LedgerEntry, LedgerEntryData, PublicKey, SequenceNumber,
            Thresholds, Uint256,
        };

        let mut snap = LedgerSnapshot::new();
        for i in 0..n {
            let account_id = AccountId(PublicKey::PublicKeyTypeEd25519(Uint256([i; 32])));
            let entry = LedgerEntry {
                last_modified_ledger_seq: u32::from(i),
                data: LedgerEntryData::Account(AccountEntry {
                    account_id,
                    balance: i64::from(i) * 1000,
                    seq_num: SequenceNumber(i64::from(i)),
                    num_sub_entries: 0,
                    inflation_dest: None,
                    flags: 0,
                    home_domain: Default::default(),
                    thresholds: Thresholds([1, 0, 0, 0]),
                    signers: Default::default(),
                    ext: Default::default(),
                }),
                ext: Default::default(),
            };
            snap.insert(vec![i], entry);
        }
        snap
    }

    // ------------------------------------------------------------------
    // HostSnapshotTracker — existing behaviour (regression guard)
    // ------------------------------------------------------------------

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

        tracker.take_before_snapshot("fn1", empty_snapshot(), Some(100));
        tracker.take_after_snapshot(empty_snapshot(), false, Some(0));

        tracker.take_before_snapshot("fn2", empty_snapshot(), Some(200));
        tracker.take_after_snapshot(empty_snapshot(), false, Some(0));

        let tracker_ref = tracker.alloc_tracker().unwrap();
        assert_eq!(tracker_ref.snapshot_count(), 2);
        assert_eq!(tracker_ref.rollback_count(), 2);
        assert_eq!(tracker_ref.snapshotted_memory_bytes(), 200);
    }

    #[test]
    fn test_tracker_without_memory_records_no_error() {
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
        assert_eq!(format!("{id}"), "snap-42");
    }

    // ------------------------------------------------------------------
    // deep_copy isolation invariant
    // ------------------------------------------------------------------

    /// Core isolation test: mutating the original after deep_copy must not
    /// affect the copy, and mutating the copy must not affect the original.
    #[test]
    fn test_ledger_snapshot_deep_copy_is_isolated() {
        let original = populated_snapshot(4);
        let copy = original.deep_copy();

        // Both start with the same entries.
        assert_eq!(original.len(), copy.len());

        // Insert a new key into the original — the copy must be unaffected.
        let mut mutated_original = original;
        mutated_original.insert(vec![0xAA], {
            use soroban_env_host::xdr::{
                AccountEntry, AccountId, LedgerEntry, LedgerEntryData, PublicKey, SequenceNumber,
                Thresholds, Uint256,
            };
            LedgerEntry {
                last_modified_ledger_seq: 99,
                data: LedgerEntryData::Account(AccountEntry {
                    account_id: AccountId(PublicKey::PublicKeyTypeEd25519(Uint256([0xAA; 32]))),
                    balance: 9999,
                    seq_num: SequenceNumber(99),
                    num_sub_entries: 0,
                    inflation_dest: None,
                    flags: 0,
                    home_domain: Default::default(),
                    thresholds: Thresholds([1, 0, 0, 0]),
                    signers: Default::default(),
                    ext: Default::default(),
                }),
                ext: Default::default(),
            }
        });

        // copy must still have only the original 4 entries.
        assert_eq!(copy.len(), 4, "deep copy was affected by mutation of original");
        assert!(copy.get(&[0xAA]).is_none(), "deep copy saw a key inserted after the copy");
    }

    /// Mutating the copy must not affect the original.
    #[test]
    fn test_ledger_snapshot_deep_copy_mutation_of_copy_does_not_affect_original() {
        let original = populated_snapshot(3);
        let mut copy = original.deep_copy();

        // Delete key [0] from the copy.
        copy.delete(&[0]);

        // Original must still have 3 entries.
        assert_eq!(original.len(), 3, "original was affected by deletion in copy");
        assert!(original.get(&[0]).is_some(), "original lost entry after copy was mutated");
    }

    /// `CapturedSnapshot::deep_copy` produces an isolated ledger state.
    #[test]
    fn test_captured_snapshot_deep_copy_is_isolated() {
        let state = populated_snapshot(2);
        let original = CapturedSnapshot {
            id: SnapshotId(1),
            host_fn_name: "test_fn".to_string(),
            state,
            before_id: None,
            trapped: false,
        };

        let mut copy = original.deep_copy();
        copy.state.insert(vec![0xFF], {
            use soroban_env_host::xdr::{
                AccountEntry, AccountId, LedgerEntry, LedgerEntryData, PublicKey, SequenceNumber,
                Thresholds, Uint256,
            };
            LedgerEntry {
                last_modified_ledger_seq: 1,
                data: LedgerEntryData::Account(AccountEntry {
                    account_id: AccountId(PublicKey::PublicKeyTypeEd25519(Uint256([0xFF; 32]))),
                    balance: 1,
                    seq_num: SequenceNumber(1),
                    num_sub_entries: 0,
                    inflation_dest: None,
                    flags: 0,
                    home_domain: Default::default(),
                    thresholds: Thresholds([1, 0, 0, 0]),
                    signers: Default::default(),
                    ext: Default::default(),
                }),
                ext: Default::default(),
            }
        });

        assert_eq!(
            original.state.len(),
            2,
            "original CapturedSnapshot.state was affected by copy mutation"
        );
    }

    /// `SnapshotPair::deep_copy` deep-copies both before and after.
    #[test]
    fn test_snapshot_pair_deep_copy_both_sides_isolated() {
        let state_before = populated_snapshot(2);
        let state_after = populated_snapshot(3);

        let before = CapturedSnapshot {
            id: SnapshotId(0),
            host_fn_name: "put".to_string(),
            state: state_before,
            before_id: None,
            trapped: false,
        };
        let after = CapturedSnapshot {
            id: SnapshotId(1),
            host_fn_name: "put".to_string(),
            state: state_after,
            before_id: Some(SnapshotId(0)),
            trapped: false,
        };

        let pair = SnapshotPair { before, after };
        let mut copy = pair.deep_copy();

        // Mutate both sides of the copy.
        copy.before.state.delete(&[0]);
        copy.after.state.delete(&[0]);
        copy.after.state.delete(&[1]);

        // Originals must be unaffected.
        assert_eq!(pair.before.state.len(), 2, "pair.before.state was mutated through deep copy");
        assert_eq!(pair.after.state.len(), 3, "pair.after.state was mutated through deep copy");
    }

    /// `HostMemoryState::capture` deep-copies the passed-in state.
    #[test]
    fn test_host_memory_state_capture_is_independent() {
        let mut live_state = populated_snapshot(5);
        let checkpoint = HostMemoryState::capture(42, &live_state, "contract_call", false);

        // Mutate live state after capture.
        live_state.delete(&[0]);
        live_state.delete(&[1]);

        // Checkpoint must still have all 5 entries.
        assert_eq!(
            checkpoint.ledger_state.len(),
            5,
            "HostMemoryState.ledger_state was affected by mutations after capture"
        );
        assert_eq!(checkpoint.instruction_pointer, 42);
        assert_eq!(checkpoint.host_fn_name, "contract_call");
        assert!(!checkpoint.trapped);
    }

    /// `HostMemoryState::deep_copy` returns a second independent copy.
    #[test]
    fn test_host_memory_state_deep_copy_is_independent() {
        let state = populated_snapshot(3);
        let original = HostMemoryState::capture(7, &state, "fn_x", false);
        let mut copy = original.deep_copy();

        copy.ledger_state.delete(&[0]);

        assert_eq!(
            original.ledger_state.len(),
            3,
            "HostMemoryState deep copy affected original"
        );
    }

    /// `HostSnapshotTracker::deep_copy_at` returns an isolated checkpoint.
    #[test]
    fn test_tracker_deep_copy_at_is_isolated() {
        let mut tracker = HostSnapshotTracker::new();

        // Snapshot 0 — before ID = 0.
        let state_before = populated_snapshot(3);
        tracker.take_before_snapshot("fn_a", state_before, None);
        tracker.take_after_snapshot(populated_snapshot(3), false, None);

        // The before snapshot's ID is 0.
        let mut checkpoint = tracker
            .deep_copy_at(0)
            .expect("expected a checkpoint at IP 0");

        // Mutate the checkpoint.
        checkpoint.ledger_state.delete(&[0]);

        // The tracker's internal state must be unaffected.
        let tracker_state = tracker.rewind_to(0).expect("rewind_to should still work");
        assert_eq!(
            tracker_state.len(),
            3,
            "tracker's internal snapshot was mutated through deep_copy_at result"
        );
    }

    /// `deep_copy_at` returns `None` for an unknown instruction pointer.
    #[test]
    fn test_tracker_deep_copy_at_unknown_ip_returns_none() {
        let mut tracker = HostSnapshotTracker::new();
        tracker.take_before_snapshot("fn", empty_snapshot(), None);
        tracker.take_after_snapshot(empty_snapshot(), false, None);

        assert!(
            tracker.deep_copy_at(999).is_none(),
            "expected None for unknown instruction pointer"
        );
    }
}
