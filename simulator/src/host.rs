// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Before/After snapshot capture around host function calls, with integrated
//! cross-contract security boundary enforcement.
//!
//! Every host function invocation produces a paired snapshot:
//! - **Before**: the ledger state immediately prior to the call
//! - **After**: the ledger state immediately after the call returns
//!
//! If the host function traps, the After snapshot is still recorded with
//! `trapped = true` so callers can inspect the state at the point of failure.
//!
//! # Cross-contract security boundary enforcement
//!
//! When a contract calls another contract the tracker maintains a
//! [`ContractCallSandbox`] that enforces the same security invariants as the
//! real Soroban host:
//!
//! 1. **Call-depth limit** – [`HostSnapshotTracker::enter_cross_contract_call`]
//!    pushes a [`CallFrame`] via [`ContractCallSandbox::push_frame`] and returns
//!    `Err(SandboxError::CallDepthExceeded)` when the depth would exceed
//!    [`MAX_CALL_DEPTH`].
//! 2. **Auth isolation** – Every new cross-contract frame is given a **fresh**,
//!    empty [`AuthScope`].  The caller's authorizations are never visible to the
//!    callee.
//! 3. **Storage namespace scoping** – The tracker exposes
//!    [`HostSnapshotTracker::check_storage_access`] so the simulation loop can
//!    validate each ledger read/write against the current frame's contract ID.
//! 4. **Error propagation** – A trap inside a callee is recorded via
//!    [`HostSnapshotTracker::record_callee_trap`], which produces the same
//!    propagating `SandboxError::CalleeTrap` the real host would return.
//! 5. **Budget sharing** – Budget metrics are recorded per-snapshot and
//!    annotated with the call depth so the diagnostic layer can attribute
//!    resource consumption to individual contracts.
//!
//! # Allocator Safety
//!
//! The [`HostSnapshotTracker`] optionally integrates an [`AllocTracker`] that
//! records the `Budget` memory consumption at each before-snapshot and validates
//! consistency after rollback.

#![allow(dead_code)]

use crate::sandbox::{
    AuthScope, CallFrame, ContractCallSandbox, SandboxDiagnostic, SandboxError,
};
use crate::snapshot::LedgerSnapshot;
use std::fmt;

// Re-export the sandbox constant so callers don't need to import the sandbox
// module directly when they only need the depth limit value.
#[allow(unused_imports)]
pub use crate::sandbox::MAX_CALL_DEPTH;

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
    /// The cross-contract call-stack depth at the time of capture.
    ///
    /// A depth of 0 means the snapshot was captured outside any cross-contract
    /// call (top-level setup phase).  A depth of 1 is the root invocation, and
    /// so on up to [`MAX_CALL_DEPTH`].
    pub call_depth: usize,
    /// The contract ID active at the time of capture (hex-encoded), if known.
    ///
    /// `None` when the snapshot was taken outside an active call frame, e.g.
    /// during the simulator's ledger-loading phase.
    pub active_contract_id: Option<String>,
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

/// Manages snapshot capture around host function calls, with integrated
/// cross-contract security boundary enforcement via [`ContractCallSandbox`].
///
/// # Security boundary integration
///
/// ```text
///  Simulation loop                      HostSnapshotTracker
///  ──────────────────                   ────────────────────
///
///  // Contract A invokes contract B
///  tracker.enter_cross_contract_call(   push CallFrame{B, depth=2}
///    "b_contract", "transfer"           (fails with CallDepthExceeded if depth>10)
///  )?;
///
///  // Optionally grant auth to B's frame
///  tracker.grant_auth_to_current("sig:xyz");
///
///  // B reads a ledger key
///  tracker.check_storage_access("key_hex")?;   // fails if key belongs to A
///
///  // Before/after snapshot around B's host call
///  tracker.take_before_snapshot("b_call", state, None);
///  // … invoke …
///  tracker.take_after_snapshot(state, false, None);
///
///  // B traps
///  tracker.record_callee_trap("b_contract", "overflow");
///
///  // B's frame exits
///  tracker.exit_cross_contract_call()?;   // pops CallFrame{B}
/// ```
pub struct HostSnapshotTracker {
    next_id: u64,
    pairs: Vec<SnapshotPair>,
    /// Holds the "before" snapshot while a host function is in-flight.
    pending_before: Option<CapturedSnapshot>,
    /// Optional allocator-state tracker for rollback safety.
    alloc_tracker: Option<AllocTracker>,
    /// Cross-contract call security sandbox.
    ///
    /// Always present — constructed via `new()` or `with_alloc_tracker()`.
    sandbox: ContractCallSandbox,
}

impl HostSnapshotTracker {
    /// Creates a new empty tracker with a fresh security sandbox.
    pub fn new() -> Self {
        Self {
            next_id: 0,
            pairs: Vec::new(),
            pending_before: None,
            alloc_tracker: None,
            sandbox: ContractCallSandbox::new(),
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
            sandbox: ContractCallSandbox::new(),
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

    // ── Sandbox delegation ────────────────────────────────────────────────────

    /// Returns a reference to the embedded [`ContractCallSandbox`].
    ///
    /// Use this when you need read access to the sandbox state (depth, violations,
    /// diagnostics) but do not need to cross a call boundary.
    pub fn sandbox(&self) -> &ContractCallSandbox {
        &self.sandbox
    }

    /// Returns a mutable reference to the embedded [`ContractCallSandbox`].
    pub fn sandbox_mut(&mut self) -> &mut ContractCallSandbox {
        &mut self.sandbox
    }

    /// Registers the ledger keys a contract is allowed to access.
    ///
    /// Delegates to [`ContractCallSandbox::register_contract_keys`].  Call
    /// this for every contract whose footprint is known before simulation
    /// begins so that [`check_storage_access`] can enforce namespace isolation.
    ///
    /// # Arguments
    /// * `contract_id` – Hex-encoded contract ID.
    /// * `keys` – Iterator over hex-encoded raw XDR of allowed `LedgerKey`s.
    pub fn register_contract_keys(
        &mut self,
        contract_id: impl Into<String>,
        keys: impl IntoIterator<Item = impl Into<String>>,
    ) {
        self.sandbox.register_contract_keys(contract_id, keys);
    }

    /// Validates that the currently executing contract is allowed to access
    /// the ledger key identified by `key_hex`.
    ///
    /// Mirrors the real Soroban host's enforcing-footprint check.  If the
    /// access would cross a contract storage namespace boundary the violation
    /// is recorded in the sandbox and `Err(SandboxError::StorageAccessViolation)`
    /// is returned.
    ///
    /// # Arguments
    /// * `key_hex` – Hex-encoded raw XDR of the `LedgerKey` being accessed.
    pub fn check_storage_access(&mut self, key_hex: &str) -> Result<(), SandboxError> {
        self.sandbox.check_storage_access(key_hex)
    }

    /// Enters a cross-contract call by pushing a new call frame onto the sandbox.
    ///
    /// This is the security-boundary gate for one contract invoking another.
    /// The new frame always starts with a **fresh, empty** [`AuthScope`] so the
    /// callee cannot observe the caller's authorizations — exactly what the real
    /// Soroban host does in `call_n_internal`.
    ///
    /// # Arguments
    /// * `contract_id` – Hex-encoded ID of the contract being called.
    /// * `function_name` – Name of the function being invoked.
    ///
    /// # Errors
    /// Returns `Err(SandboxError::CallDepthExceeded)` if pushing the frame
    /// would take the call depth beyond [`MAX_CALL_DEPTH`].
    pub fn enter_cross_contract_call(
        &mut self,
        contract_id: impl Into<String>,
        function_name: impl Into<String>,
    ) -> Result<(), SandboxError> {
        let depth = self.sandbox.depth() + 1; // prospective depth
        let contract_id = contract_id.into();
        let caller = self
            .sandbox
            .current_frame()
            .map(|f| f.contract_id.clone());
        let auth_scope = match caller {
            Some(ref cid) => AuthScope::with_caller(cid),
            None => AuthScope::new(),
        };
        let frame = CallFrame::new(contract_id, function_name, depth, auth_scope);
        self.sandbox.push_frame(frame)
    }

    /// Exits the current cross-contract call by popping the innermost frame.
    ///
    /// Should be called after the callee returns (successfully or after a
    /// trapped error has been recorded via [`record_callee_trap`]).
    ///
    /// # Errors
    /// Returns `Err(SandboxError::CallStackUnderflow)` if the stack is already
    /// empty.
    pub fn exit_cross_contract_call(&mut self) -> Result<CallFrame, SandboxError> {
        self.sandbox.pop_frame()
    }

    /// Records a callee trap and returns the propagating error.
    ///
    /// Mirrors the real Soroban host's behaviour: a trap inside any call frame
    /// propagates upward, failing the entire transaction.  Call this before
    /// calling [`exit_cross_contract_call`] so the violation is attributed to
    /// the correct frame.
    ///
    /// # Arguments
    /// * `contract_id` – The contract that trapped.
    /// * `reason` – Human-readable reason (e.g. from `HostError` formatting).
    pub fn record_callee_trap(
        &mut self,
        contract_id: impl Into<String>,
        reason: impl Into<String>,
    ) -> SandboxError {
        self.sandbox.record_callee_trap(contract_id, reason)
    }

    /// Grants an authorization key to the currently executing call frame.
    ///
    /// Returns `false` if there is no active frame (the call stack is empty).
    ///
    /// Auth keys are opaque strings.  In a full implementation they would be
    /// serialized `AuthorizedInvocation` XDR blobs; for simulation purposes
    /// human-readable identifiers work equally well.
    pub fn grant_auth_to_current(&mut self, auth_key: impl Into<String>) -> bool {
        self.sandbox.grant_auth_to_current(auth_key)
    }

    /// Attempts to consume an auth key from the current call frame.
    ///
    /// Implements one-shot auth semantics: a key can only be consumed once.
    /// Returns `false` if the key is unavailable or the stack is empty.
    pub fn consume_auth_in_current(&mut self, auth_key: &str) -> bool {
        self.sandbox.consume_auth_in_current(auth_key)
    }

    /// Returns the current cross-contract call depth (0 = idle).
    pub fn call_depth(&self) -> usize {
        self.sandbox.depth()
    }

    /// Returns a structured diagnostic summary of the sandbox state.
    ///
    /// Intended for embedding in `SimulationResponse` diagnostic logs when
    /// a security violation is detected.
    pub fn sandbox_diagnostic(&self) -> SandboxDiagnostic {
        self.sandbox.diagnostic_summary()
    }

    // ── Snapshot management ───────────────────────────────────────────────────

    /// Allocate the next snapshot ID.
    fn next_snapshot_id(&mut self) -> SnapshotId {
        let id = SnapshotId(self.next_id);
        self.next_id += 1;
        id
    }

    /// Call this immediately **before** a host function executes.
    ///
    /// Takes a snapshot of the current ledger state and stores it as
    /// the pending "before" snapshot.  The snapshot is annotated with the
    /// current cross-contract call depth and the active contract ID so the
    /// diagnostic layer can attribute it to the right frame.
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
        let call_depth = self.sandbox.depth();
        let active_contract_id = self
            .sandbox
            .current_frame()
            .map(|f| f.contract_id.clone());
        self.pending_before = Some(CapturedSnapshot {
            id,
            host_fn_name: host_fn_name.to_string(),
            state,
            before_id: None,
            trapped: false,
            call_depth,
            active_contract_id,
        });
    }

    /// Call this immediately **after** a host function returns.
    ///
    /// Takes a snapshot of the resulting ledger state and pairs it with
    /// the pending "before" snapshot. If there is no pending "before"
    /// snapshot (programming error), this is a no-op and returns `None`.
    ///
    /// The after-snapshot inherits the call depth and contract ID from the
    /// matching before-snapshot.
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
        // Inherit depth and contract ID from the before snapshot so the pair
        // is always self-consistent, even if a cross-contract call boundary
        // was crossed between the two capture points.
        let call_depth = before.call_depth;
        let active_contract_id = before.active_contract_id.clone();

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
            call_depth,
            active_contract_id,
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

#[cfg(test)]
mod tests {
    use super::*;

    fn empty_snapshot() -> LedgerSnapshot {
        LedgerSnapshot::new()
    }

    // ── Legacy snapshot tests (preserved unchanged) ──────────────────────────

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

    // ── Sandbox integration tests ────────────────────────────────────────────

    #[test]
    fn enter_and_exit_cross_contract_call() {
        let mut tracker = HostSnapshotTracker::new();
        assert_eq!(tracker.call_depth(), 0);

        tracker
            .enter_cross_contract_call("contract_a", "do_thing")
            .expect("first enter should succeed");
        assert_eq!(tracker.call_depth(), 1);

        tracker
            .exit_cross_contract_call()
            .expect("exit should succeed");
        assert_eq!(tracker.call_depth(), 0);
    }

    #[test]
    fn nested_cross_contract_calls_enforce_depth_limit() {
        let mut tracker = HostSnapshotTracker::new();

        for i in 0..MAX_CALL_DEPTH {
            tracker
                .enter_cross_contract_call(format!("c{i}"), "fn")
                .unwrap_or_else(|_| panic!("frame {i} should succeed"));
        }
        assert_eq!(tracker.call_depth(), MAX_CALL_DEPTH);

        // One more push must fail.
        let err = tracker
            .enter_cross_contract_call("overflow", "fn")
            .unwrap_err();
        assert!(matches!(err, SandboxError::CallDepthExceeded { .. }));
        // Depth must be unchanged.
        assert_eq!(tracker.call_depth(), MAX_CALL_DEPTH);
    }

    #[test]
    fn snapshot_annotated_with_call_depth() {
        let mut tracker = HostSnapshotTracker::new();

        // Enter depth 1.
        tracker
            .enter_cross_contract_call("contract_a", "fn_a")
            .unwrap();
        tracker.take_before_snapshot("call_at_depth_1", empty_snapshot(), None);
        let pair = tracker
            .take_after_snapshot(empty_snapshot(), false, None)
            .unwrap();

        assert_eq!(pair.before.call_depth, 1);
        assert_eq!(pair.after.call_depth, 1);
        assert_eq!(
            pair.before.active_contract_id.as_deref(),
            Some("contract_a")
        );
    }

    #[test]
    fn snapshot_outside_call_frame_has_zero_depth() {
        let mut tracker = HostSnapshotTracker::new();

        // No call frame active.
        tracker.take_before_snapshot("setup_fn", empty_snapshot(), None);
        let pair = tracker
            .take_after_snapshot(empty_snapshot(), false, None)
            .unwrap();

        assert_eq!(pair.before.call_depth, 0);
        assert!(pair.before.active_contract_id.is_none());
    }

    #[test]
    fn auth_isolation_across_frames() {
        let mut tracker = HostSnapshotTracker::new();

        // Root frame with an auth.
        tracker
            .enter_cross_contract_call("root_contract", "entry")
            .unwrap();
        assert!(tracker.grant_auth_to_current("sig:root_sig"));

        // Callee frame: fresh empty auth — cannot see root's sig.
        tracker
            .enter_cross_contract_call("child_contract", "callback")
            .unwrap();
        assert_eq!(
            tracker
                .sandbox()
                .current_frame()
                .unwrap()
                .auth_scope
                .available_auth_count(),
            0,
            "callee must start with an empty auth scope"
        );

        tracker.exit_cross_contract_call().unwrap();

        // Root frame still has its auth.
        assert_eq!(
            tracker
                .sandbox()
                .current_frame()
                .unwrap()
                .auth_scope
                .available_auth_count(),
            1
        );
    }

    #[test]
    fn callee_auth_does_not_propagate_to_caller() {
        let mut tracker = HostSnapshotTracker::new();
        tracker
            .enter_cross_contract_call("contract_a", "fn")
            .unwrap();
        tracker
            .enter_cross_contract_call("contract_b", "fn")
            .unwrap();

        // Grant auth only to the innermost frame (B).
        tracker.grant_auth_to_current("sig:b_only");
        assert!(tracker.consume_auth_in_current("sig:b_only"));

        // Exit B.
        tracker.exit_cross_contract_call().unwrap();

        // A must not have "sig:b_only".
        assert!(
            !tracker.consume_auth_in_current("sig:b_only"),
            "caller must not have access to callee's auth"
        );
    }

    #[test]
    fn one_shot_auth_cannot_be_reused() {
        let mut tracker = HostSnapshotTracker::new();
        tracker
            .enter_cross_contract_call("contract_a", "fn")
            .unwrap();
        tracker.grant_auth_to_current("sig:once");

        assert!(tracker.consume_auth_in_current("sig:once"));
        assert!(
            !tracker.consume_auth_in_current("sig:once"),
            "one-shot auth must not be reusable"
        );
    }

    #[test]
    fn storage_access_violation_is_recorded() {
        let mut tracker = HostSnapshotTracker::new();
        tracker.register_contract_keys("contract_a", ["key_a"]);
        tracker.register_contract_keys("contract_b", ["key_b"]);

        tracker
            .enter_cross_contract_call("contract_a", "attack")
            .unwrap();

        let result = tracker.check_storage_access("key_b");
        assert!(result.is_err());
        assert!(tracker.sandbox().has_violations());
    }

    #[test]
    fn storage_access_within_own_namespace_succeeds() {
        let mut tracker = HostSnapshotTracker::new();
        tracker.register_contract_keys("contract_a", ["key_a"]);
        tracker
            .enter_cross_contract_call("contract_a", "read")
            .unwrap();

        assert!(tracker.check_storage_access("key_a").is_ok());
        assert!(!tracker.sandbox().has_violations());
    }

    #[test]
    fn callee_trap_recorded_in_sandbox() {
        let mut tracker = HostSnapshotTracker::new();
        tracker
            .enter_cross_contract_call("contract_a", "fn")
            .unwrap();
        tracker
            .enter_cross_contract_call("contract_b", "boom")
            .unwrap();

        let err = tracker.record_callee_trap("contract_b", "overflow");
        assert!(matches!(err, SandboxError::CalleeTrap { .. }));
        assert!(tracker.sandbox().has_violations());
    }

    #[test]
    fn sandbox_diagnostic_reflects_live_state() {
        let mut tracker = HostSnapshotTracker::new();
        tracker
            .enter_cross_contract_call("c1", "fn1")
            .unwrap();
        tracker
            .enter_cross_contract_call("c2", "fn2")
            .unwrap();

        let diag = tracker.sandbox_diagnostic();
        assert_eq!(diag.current_depth, 2);
        assert_eq!(diag.active_frames.len(), 2);
        assert_eq!(diag.active_frames[0].contract_id, "c1");
        assert_eq!(diag.active_frames[1].contract_id, "c2");
    }

    #[test]
    fn caller_contract_propagated_to_child_auth_scope() {
        let mut tracker = HostSnapshotTracker::new();
        tracker
            .enter_cross_contract_call("parent_contract", "invoke_child")
            .unwrap();

        // Enter the child — the child's auth scope should know its caller.
        tracker
            .enter_cross_contract_call("child_contract", "callback")
            .unwrap();

        let caller = tracker
            .sandbox()
            .current_frame()
            .unwrap()
            .auth_scope
            .caller_contract();
        assert_eq!(caller, Some("parent_contract"));
    }
}
