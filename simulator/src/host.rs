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
use soroban_env_host::xdr::{Limits, SorobanAuthorizationEntry, SorobanAuthorizedInvocation};
use std::fmt;

/// Maximum supported depth of a Soroban authorization invocation tree.
///
/// The root invocation is level 1, so the deepest accepted tree is a root with
/// sub-invocations nested three levels below it
/// (root -> child -> grandchild -> great-grandchild). A root with no
/// sub-invocations has depth 1.
pub const MAX_AUTH_DEPTH: usize = 4;

/// Maximum nesting depth accepted when decoding untrusted XDR.
///
/// XDR decoding recurses once per nested type, so decoding with
/// [`Limits::none`] lets a crafted, deeply nested authorization tree overflow
/// the stack before any validation can run. 500 matches the depth limit the
/// Soroban host itself applies to XDR reads and writes.
pub const MAX_XDR_DECODE_DEPTH: u32 = 500;

/// XDR decode limits for untrusted input: depth-bounded to prevent stack
/// overflow, with no length cap so large but legitimate envelopes still parse.
#[must_use]
pub fn untrusted_xdr_limits() -> Limits {
    Limits::depth(MAX_XDR_DECODE_DEPTH)
}

/// Returned when an authorization tree is nested deeper than the supported
/// maximum.
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
#[error("authorization tree depth exceeds the maximum of {max_depth} levels")]
pub struct AuthDepthExceeded {
    /// The depth limit that was exceeded.
    pub max_depth: usize,
}

/// Verifies that an authorization invocation tree is at most `max_depth`
/// levels deep, counting the root as level 1.
///
/// The traversal is iterative (explicit stack) so the verifier itself cannot
/// overflow the call stack regardless of input shape, and it stops at the first
/// node that exceeds `max_depth`, so a hostile tree is rejected after visiting
/// at most `max_depth + 1` levels along any path.
///
/// # Errors
///
/// Returns [`AuthDepthExceeded`] if any invocation sits deeper than
/// `max_depth`.
pub fn verify_auth_tree_depth(
    root: &SorobanAuthorizedInvocation,
    max_depth: usize,
) -> Result<(), AuthDepthExceeded> {
    let mut stack: Vec<(&SorobanAuthorizedInvocation, usize)> = vec![(root, 1)];
    while let Some((node, depth)) = stack.pop() {
        if depth > max_depth {
            return Err(AuthDepthExceeded { max_depth });
        }
        for child in node.sub_invocations.iter() {
            stack.push((child, depth + 1));
        }
    }
    Ok(())
}

/// Verifies every authorization entry attached to an `InvokeHostFunction`
/// operation against [`MAX_AUTH_DEPTH`].
///
/// # Errors
///
/// Returns [`AuthDepthExceeded`] if any entry's invocation tree is too deep.
pub fn verify_contract_auth(auth: &[SorobanAuthorizationEntry]) -> Result<(), AuthDepthExceeded> {
    auth.iter()
        .try_for_each(|entry| verify_auth_tree_depth(&entry.root_invocation, MAX_AUTH_DEPTH))
}

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

/// Manages snapshot capture around host function calls.
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
}

impl Default for HostSnapshotTracker {
    fn default() -> Self {
        Self::new()
    }
}

/// Builders for authorization trees, shared by unit tests in this crate.
#[cfg(test)]
pub(crate) mod test_support {
    use soroban_env_host::xdr::{
        ContractId, Hash, InvokeContractArgs, ScAddress, ScSymbol, SorobanAuthorizationEntry,
        SorobanAuthorizedFunction, SorobanAuthorizedInvocation, SorobanCredentials, VecM,
    };

    pub(crate) fn invocation(
        children: Vec<SorobanAuthorizedInvocation>,
    ) -> SorobanAuthorizedInvocation {
        SorobanAuthorizedInvocation {
            function: SorobanAuthorizedFunction::ContractFn(InvokeContractArgs {
                contract_address: ScAddress::Contract(ContractId(Hash([7u8; 32]))),
                function_name: ScSymbol("f".try_into().unwrap()),
                args: VecM::default(),
            }),
            sub_invocations: children.try_into().unwrap(),
        }
    }

    /// A linear chain with `depth` levels (root is level 1). Built bottom-up
    /// with a loop, so constructing even a huge chain does not recurse.
    pub(crate) fn chain(depth: usize) -> SorobanAuthorizedInvocation {
        assert!(depth >= 1);
        let mut node = invocation(vec![]);
        for _ in 1..depth {
            node = invocation(vec![node]);
        }
        node
    }

    pub(crate) fn entry(root: SorobanAuthorizedInvocation) -> SorobanAuthorizationEntry {
        SorobanAuthorizationEntry {
            credentials: SorobanCredentials::SourceAccount,
            root_invocation: root,
        }
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

    // ---- Authorization tree depth limit ----------------------------------

    use super::test_support::{chain, entry, invocation};
    use soroban_env_host::xdr::{ReadXdr, WriteXdr};

    #[test]
    fn test_auth_depth_up_to_max_is_accepted() {
        for depth in 1..=MAX_AUTH_DEPTH {
            let e = entry(chain(depth));
            assert_eq!(verify_contract_auth(&[e]), Ok(()), "depth {depth}");
        }
    }

    #[test]
    fn test_auth_depth_max_plus_one_is_rejected() {
        let e = entry(chain(MAX_AUTH_DEPTH + 1));
        assert_eq!(
            verify_contract_auth(&[e]),
            Err(AuthDepthExceeded {
                max_depth: MAX_AUTH_DEPTH
            })
        );
    }

    #[test]
    fn test_auth_depth_counts_root_as_level_one() {
        // A lone root is depth 1; each sub-invocation adds one level.
        assert_eq!(verify_auth_tree_depth(&chain(1), 1), Ok(()));
        assert!(verify_auth_tree_depth(&chain(2), 1).is_err());
    }

    #[test]
    fn test_auth_depth_checks_every_branch_and_every_entry() {
        // Shallow siblings must not mask one deep branch.
        let mixed = invocation(vec![chain(1), chain(MAX_AUTH_DEPTH), chain(2)]);
        assert!(verify_auth_tree_depth(&mixed, MAX_AUTH_DEPTH).is_err());

        // A single bad entry among valid ones fails the whole set.
        let entries = vec![
            entry(chain(1)),
            entry(chain(MAX_AUTH_DEPTH)),
            entry(chain(MAX_AUTH_DEPTH + 1)),
        ];
        assert!(verify_contract_auth(&entries).is_err());
        assert_eq!(verify_contract_auth(&entries[..2]), Ok(()));
        assert_eq!(verify_contract_auth(&[]), Ok(()));
    }

    #[test]
    fn test_auth_wide_shallow_tree_is_accepted() {
        let wide = invocation((0..1_000).map(|_| chain(MAX_AUTH_DEPTH - 1)).collect());
        assert_eq!(verify_auth_tree_depth(&wide, MAX_AUTH_DEPTH), Ok(()));
    }

    #[test]
    fn test_auth_extremely_deep_tree_does_not_overflow_stack() {
        // 200k levels would overflow the stack of any recursive traversal.
        let deep = entry(chain(200_000));
        let result = verify_contract_auth(std::slice::from_ref(&deep));
        // Dropping a tree this deep is itself recursive, so leak it instead.
        std::mem::forget(deep);
        assert_eq!(
            result,
            Err(AuthDepthExceeded {
                max_depth: MAX_AUTH_DEPTH
            })
        );
    }

    #[test]
    fn test_auth_depth_exceeded_message_names_the_limit() {
        let err = AuthDepthExceeded { max_depth: 4 };
        assert_eq!(
            err.to_string(),
            "authorization tree depth exceeds the maximum of 4 levels"
        );
    }

    /// Raw XDR for a linear invocation chain of `depth` levels, assembled
    /// without any recursive encoder.
    fn chain_xdr(depth: usize) -> Vec<u8> {
        let leaf = invocation(vec![]).to_xdr(Limits::none()).unwrap();
        // Everything except the trailing 4-byte (empty) sub_invocations length.
        let function = &leaf[..leaf.len() - 4];
        let mut out = Vec::with_capacity(depth * (function.len() + 4));
        for _ in 1..depth {
            out.extend_from_slice(function);
            out.extend_from_slice(&1u32.to_be_bytes()); // one sub-invocation
        }
        out.extend_from_slice(&leaf);
        out
    }

    #[test]
    fn test_untrusted_xdr_decode_rejects_pathological_depth_without_overflow() {
        let bytes = chain_xdr(100_000);
        assert!(
            SorobanAuthorizedInvocation::from_xdr(&bytes, untrusted_xdr_limits()).is_err(),
            "decode must fail cleanly instead of recursing 100k levels"
        );
    }

    #[test]
    fn test_untrusted_xdr_decode_accepts_valid_tree_and_roundtrips() {
        let bytes = chain_xdr(MAX_AUTH_DEPTH);
        let decoded =
            SorobanAuthorizedInvocation::from_xdr(&bytes, untrusted_xdr_limits()).unwrap();
        assert_eq!(verify_auth_tree_depth(&decoded, MAX_AUTH_DEPTH), Ok(()));
        assert_eq!(decoded, chain(MAX_AUTH_DEPTH));
    }
}
