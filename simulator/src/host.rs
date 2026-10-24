// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Before/After snapshot capture and state rollback verification around host function calls.
//!
//! Every host function invocation produces a paired snapshot:
//! - **Before**: the ledger state immediately prior to the call
//! - **After**: the ledger state immediately after the call returns
//!
//! If the host function traps, the After snapshot is still recorded with
//! `trapped = true` so callers can inspect the state at the point of failure.
//!
//! # Sub-Invocation Rollback Verification
//!
//! When a nested contract call (sub-invocation) fails and the caller catches the
//! failure (e.g. via `try_call`), Soroban requires that all child storage writes
//! are rolled back. This module snapshots the storage write set before each
//! sub-invocation and diffs it after a failure. If any writes (insertions,
//! modifications, or deletions) from the failed call remain, an actionable
//! [`DiagnosticEvent`] and structured [`SubInvocationRollbackDiagnostic`] are emitted.
//!
//! # Allocator Safety
//!
//! The [`HostSnapshotTracker`] optionally integrates an [`AllocTracker`] that
//! records the `Budget` memory consumption at each before-snapshot and validates
//! consistency after rollback.

#![allow(dead_code)]

use crate::snapshot::LedgerSnapshot;
use crate::types::DiagnosticEvent;
use soroban_env_host::xdr::{Limits, WriteXdr};
use soroban_env_host::EnvBase;
use std::collections::BTreeMap;
use std::fmt;

/// Unique identifier for a snapshot.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, serde::Serialize, serde::Deserialize)]
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

/// Represents the captured storage write set at a point in execution.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StorageWriteSet {
    /// Map of ledger key bytes to serialized XDR entry bytes.
    pub entries: BTreeMap<Vec<u8>, Option<Vec<u8>>>,
}

impl StorageWriteSet {
    /// Creates a new empty storage write set.
    pub fn new() -> Self {
        Self {
            entries: BTreeMap::new(),
        }
    }

    /// Captures the storage write set from a [`LedgerSnapshot`].
    pub fn from_snapshot(snapshot: &LedgerSnapshot) -> Self {
        let mut entries = BTreeMap::new();
        for (key, entry) in snapshot.iter() {
            let entry_bytes = entry.to_xdr(Limits::none()).ok();
            entries.insert(key, entry_bytes);
        }
        Self { entries }
    }

    /// Computes the diff between this baseline write set and an `after` write set.
    ///
    /// Identifies all keys that were inserted, modified, or deleted in `after`
    /// relative to `self`.
    pub fn diff(&self, after: &Self) -> StorageWriteSetDiff {
        let mut inserted = Vec::new();
        let mut modified = Vec::new();
        let mut deleted = Vec::new();

        for (key, after_val) in &after.entries {
            match self.entries.get(key) {
                None => inserted.push(key.clone()),
                Some(before_val) if before_val != after_val => modified.push(key.clone()),
                _ => {}
            }
        }

        for key in self.entries.keys() {
            if !after.entries.contains_key(key) {
                deleted.push(key.clone());
            }
        }

        inserted.sort_unstable();
        modified.sort_unstable();
        deleted.sort_unstable();

        StorageWriteSetDiff {
            inserted,
            modified,
            deleted,
        }
    }
}

impl Default for StorageWriteSet {
    fn default() -> Self {
        Self::new()
    }
}

/// Represents differences between two storage write sets.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize)]
pub struct StorageWriteSetDiff {
    /// Keys inserted (present in after, absent from before).
    pub inserted: Vec<Vec<u8>>,
    /// Keys modified (present in both, but serialized contents differ).
    pub modified: Vec<Vec<u8>>,
    /// Keys deleted (present in before, absent from after).
    pub deleted: Vec<Vec<u8>>,
}

impl StorageWriteSetDiff {
    /// Returns `true` if there are no difference entries.
    pub fn is_empty(&self) -> bool {
        self.inserted.is_empty() && self.modified.is_empty() && self.deleted.is_empty()
    }

    /// Returns the total count of changed keys.
    pub fn total_changes(&self) -> usize {
        self.inserted.len() + self.modified.len() + self.deleted.len()
    }

    /// Returns inserted keys formatted as hex strings.
    pub fn inserted_keys_hex(&self) -> Vec<String> {
        self.inserted.iter().map(hex::encode).collect()
    }

    /// Returns modified keys formatted as hex strings.
    pub fn modified_keys_hex(&self) -> Vec<String> {
        self.modified.iter().map(hex::encode).collect()
    }

    /// Returns deleted keys formatted as hex strings.
    pub fn deleted_keys_hex(&self) -> Vec<String> {
        self.deleted.iter().map(hex::encode).collect()
    }
}

/// Structured diagnostic information emitted when a failed sub-invocation leaves
/// un-rolled-back storage writes.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize)]
pub struct SubInvocationRollbackDiagnostic {
    /// Target contract identifier if known.
    pub contract_id: Option<String>,
    /// Name of the host function or method invoked.
    pub host_fn_name: String,
    /// Call stack depth where the sub-invocation took place.
    pub call_depth: u32,
    /// Snapshot identifier associated with this sub-invocation.
    pub sub_invocation_id: SnapshotId,
    /// Storage writes that remained un-rolled-back.
    pub diff: StorageWriteSetDiff,
    /// Detailed diagnostic message.
    pub message: String,
}

impl SubInvocationRollbackDiagnostic {
    /// Converts this structured diagnostic into a [`DiagnosticEvent`] suitable
    /// for inclusion in the simulation event trace.
    pub fn to_diagnostic_event(&self) -> DiagnosticEvent {
        let mut topics = vec![
            "storage_rollback_failed".to_string(),
            self.host_fn_name.clone(),
        ];
        if let Some(ref cid) = self.contract_id {
            topics.push(cid.clone());
        }

        DiagnosticEvent {
            event_type: "diagnostic".to_string(),
            contract_id: self.contract_id.clone(),
            topics,
            data: self.message.clone(),
            in_successful_contract_call: false,
            snapshot_id: Some(self.sub_invocation_id.to_string()),
            snapshot_metadata: None,
            wasm_instruction: None,
            cpu: None,
            mem: None,
        }
    }

    /// Emits this diagnostic event into a Soroban [`Host`](soroban_env_host::Host) diagnostic log if available.
    pub fn emit_to_host(
        &self,
        host: &soroban_env_host::Host,
    ) -> Result<(), soroban_env_host::HostError> {
        host.log_from_slice(&self.message, &[]).map(|_| ())
    }
}

/// Represents an in-flight sub-invocation call frame on the stack.
#[derive(Debug, Clone)]
pub struct SubInvocationFrame {
    /// Snapshot ID assigned to this sub-invocation.
    pub id: SnapshotId,
    /// Name of the function being invoked.
    pub host_fn_name: String,
    /// Contract identifier if applicable.
    pub contract_id: Option<String>,
    /// Call stack depth (1-based).
    pub call_depth: u32,
    /// Baseline ledger snapshot captured immediately before the call.
    pub before_state: LedgerSnapshot,
    /// Storage write set captured immediately before the call.
    pub write_set_before: StorageWriteSet,
    /// Memory consumption recorded at before-snapshot time.
    pub memory_bytes: Option<u64>,
}

/// Result of exiting a sub-invocation frame.
#[derive(Debug)]
pub enum SubInvocationResult {
    /// Sub-invocation succeeded normally.
    Success { pair: SnapshotPair },
    /// Sub-invocation failed, and its storage writes were cleanly rolled back.
    RolledBackCleanly { pair: SnapshotPair },
    /// Sub-invocation failed, and un-rolled-back storage writes were detected!
    RollbackFailed {
        pair: SnapshotPair,
        diagnostic: SubInvocationRollbackDiagnostic,
        diagnostic_event: Box<DiagnosticEvent>,
    },
}

impl SubInvocationResult {
    /// Returns `true` if the sub-invocation completed successfully.
    pub fn is_success(&self) -> bool {
        matches!(self, Self::Success { .. })
    }

    /// Returns `true` if the sub-invocation failed but rolled back cleanly.
    pub fn is_clean_rollback(&self) -> bool {
        matches!(self, Self::RolledBackCleanly { .. })
    }

    /// Returns `true` if storage rollback verification failed.
    pub fn is_rollback_failed(&self) -> bool {
        matches!(self, Self::RollbackFailed { .. })
    }

    /// Returns a reference to the rollback diagnostic if verification failed.
    pub fn diagnostic(&self) -> Option<&SubInvocationRollbackDiagnostic> {
        match self {
            Self::RollbackFailed { diagnostic, .. } => Some(diagnostic),
            _ => None,
        }
    }

    /// Returns a reference to the snapshot pair recorded for this invocation.
    pub fn pair(&self) -> &SnapshotPair {
        match self {
            Self::Success { pair }
            | Self::RolledBackCleanly { pair }
            | Self::RollbackFailed { pair, .. } => pair,
        }
    }
}

/// Computes differences between storage write sets of two snapshots.
pub fn diff_storage_write_sets(
    before: &LedgerSnapshot,
    after: &LedgerSnapshot,
) -> StorageWriteSetDiff {
    let ws_before = StorageWriteSet::from_snapshot(before);
    let ws_after = StorageWriteSet::from_snapshot(after);
    ws_before.diff(&ws_after)
}

/// Verifies whether storage state was properly rolled back after a failed sub-invocation.
///
/// Diffs `after_failure` against `before`. If any storage writes remain
/// (insertions, modifications, or deletions), constructs and returns a [`SubInvocationRollbackDiagnostic`].
pub fn verify_sub_invocation_rollback(
    host_fn_name: &str,
    contract_id: Option<&str>,
    call_depth: u32,
    snapshot_id: SnapshotId,
    before: &LedgerSnapshot,
    after_failure: &LedgerSnapshot,
) -> Option<SubInvocationRollbackDiagnostic> {
    let diff = diff_storage_write_sets(before, after_failure);
    if diff.is_empty() {
        return None;
    }

    let message = format!(
        "Storage rollback verification failed: sub-invocation '{host_fn_name}' at depth {call_depth} failed but left {} un-rolled-back storage write(s) [inserted: {}, modified: {}, deleted: {}]",
        diff.total_changes(),
        diff.inserted.len(),
        diff.modified.len(),
        diff.deleted.len(),
    );

    Some(SubInvocationRollbackDiagnostic {
        contract_id: contract_id.map(ToString::to_string),
        host_fn_name: host_fn_name.to_string(),
        call_depth,
        sub_invocation_id: snapshot_id,
        diff,
        message,
    })
}

/// Convenience function to create a [`DiagnosticEvent`] from a rollback diagnostic.
pub fn create_rollback_diagnostic_event(
    diagnostic: &SubInvocationRollbackDiagnostic,
) -> DiagnosticEvent {
    diagnostic.to_diagnostic_event()
}

/// Manages snapshot capture and rollback verification around host function calls
/// and nested sub-invocations.
pub struct HostSnapshotTracker {
    next_id: u64,
    pairs: Vec<SnapshotPair>,
    /// Holds the "before" snapshot while a host function is in-flight.
    pending_before: Option<CapturedSnapshot>,
    /// Optional allocator-state tracker for rollback safety.
    alloc_tracker: Option<AllocTracker>,
    /// Active sub-invocation call stack.
    sub_invocation_stack: Vec<SubInvocationFrame>,
    /// Accumulated diagnostic events (including rollback failure diagnostics).
    diagnostic_events: Vec<DiagnosticEvent>,
    /// Structured rollback diagnostics emitted on failed sub-invocations.
    rollback_diagnostics: Vec<SubInvocationRollbackDiagnostic>,
}

impl HostSnapshotTracker {
    /// Creates a new empty tracker.
    pub fn new() -> Self {
        Self {
            next_id: 0,
            pairs: Vec::new(),
            pending_before: None,
            alloc_tracker: None,
            sub_invocation_stack: Vec::new(),
            diagnostic_events: Vec::new(),
            rollback_diagnostics: Vec::new(),
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
            sub_invocation_stack: Vec::new(),
            diagnostic_events: Vec::new(),
            rollback_diagnostics: Vec::new(),
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
    /// If `trapped` is `true`, checks whether any storage writes remained
    /// from the trapped execution. If un-rolled-back writes are detected,
    /// a diagnostic event is emitted into the tracker.
    ///
    /// # Arguments
    /// * `state` - The ledger state after the host function returned.
    /// * `trapped` - Whether the host function trapped/failed.
    /// * `restored_memory_bytes` - If provided and an [`AllocTracker`] is
    ///   configured, records a rollback with the restored memory consumption.
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

        // When a call trapped, verify whether storage writes were rolled back.
        if trapped {
            if let Some(diag) = verify_sub_invocation_rollback(
                &before.host_fn_name,
                None,
                1,
                before_id,
                &before.state,
                &state,
            ) {
                let event = diag.to_diagnostic_event();
                self.diagnostic_events.push(event);
                self.rollback_diagnostics.push(diag);
            }
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

    /// Pushes a new sub-invocation frame onto the call stack.
    ///
    /// Captures the storage write set immediately before the child call executes.
    ///
    /// # Arguments
    /// * `host_fn_name` - The contract function or method being invoked.
    /// * `contract_id` - Optional target contract ID.
    /// * `state` - The ledger snapshot immediately prior to the call.
    /// * `memory_bytes` - Optional memory consumption baseline.
    pub fn enter_sub_invocation(
        &mut self,
        host_fn_name: &str,
        contract_id: Option<&str>,
        state: &LedgerSnapshot,
        memory_bytes: Option<u64>,
    ) -> SnapshotId {
        if let (Some(tracker), Some(mem)) = (self.alloc_tracker.as_mut(), memory_bytes) {
            tracker.snapshot(mem);
        }

        let id = self.next_snapshot_id();
        let call_depth = (self.sub_invocation_stack.len() as u32) + 1;
        let write_set_before = StorageWriteSet::from_snapshot(state);

        self.sub_invocation_stack.push(SubInvocationFrame {
            id,
            host_fn_name: host_fn_name.to_string(),
            contract_id: contract_id.map(ToString::to_string),
            call_depth,
            before_state: state.clone(),
            write_set_before,
            memory_bytes,
        });

        id
    }

    /// Pops the active sub-invocation frame and verifies state rollback on failure.
    ///
    /// If `failed` is `true` (e.g. child trapped or failed, caught by `try_call`),
    /// diffs `current_state` against the pre-invocation write set. If any writes
    /// from the failed call remain, emits a [`DiagnosticEvent`] and returns
    /// [`SubInvocationResult::RollbackFailed`].
    ///
    /// # Arguments
    /// * `current_state` - The current ledger snapshot upon returning from the call.
    /// * `failed` - Whether the child sub-invocation failed.
    /// * `restored_memory_bytes` - Optional restored memory consumption for allocator tracking.
    pub fn exit_sub_invocation(
        &mut self,
        current_state: &LedgerSnapshot,
        failed: bool,
        restored_memory_bytes: Option<u64>,
    ) -> Option<SubInvocationResult> {
        let frame = self.sub_invocation_stack.pop()?;
        let after_id = self.next_snapshot_id();

        if failed {
            if let (Some(tracker), Some(mem)) = (self.alloc_tracker.as_mut(), restored_memory_bytes)
            {
                tracker.record_rollback(mem);
            }
        }

        let before_snap = CapturedSnapshot {
            id: frame.id,
            host_fn_name: frame.host_fn_name.clone(),
            state: frame.before_state.clone(),
            before_id: None,
            trapped: false,
        };

        let after_snap = CapturedSnapshot {
            id: after_id,
            host_fn_name: frame.host_fn_name.clone(),
            state: current_state.clone(),
            before_id: Some(frame.id),
            trapped: failed,
        };

        let pair = SnapshotPair {
            before: before_snap,
            after: after_snap,
        };
        self.pairs.push(pair.clone());

        if failed {
            let current_write_set = StorageWriteSet::from_snapshot(current_state);
            let diff = frame.write_set_before.diff(&current_write_set);

            if !diff.is_empty() {
                let message = format!(
                    "Storage rollback verification failed: sub-invocation '{}' at depth {} failed but left {} un-rolled-back storage write(s) [inserted: {}, modified: {}, deleted: {}]",
                    frame.host_fn_name,
                    frame.call_depth,
                    diff.total_changes(),
                    diff.inserted.len(),
                    diff.modified.len(),
                    diff.deleted.len(),
                );

                let diagnostic = SubInvocationRollbackDiagnostic {
                    contract_id: frame.contract_id,
                    host_fn_name: frame.host_fn_name,
                    call_depth: frame.call_depth,
                    sub_invocation_id: frame.id,
                    diff,
                    message,
                };

                let diagnostic_event = diagnostic.to_diagnostic_event();
                self.diagnostic_events
                    .push(diagnostic.to_diagnostic_event());
                self.rollback_diagnostics.push(diagnostic.clone());

                return Some(SubInvocationResult::RollbackFailed {
                    pair,
                    diagnostic,
                    diagnostic_event: Box::new(diagnostic_event),
                });
            }

            Some(SubInvocationResult::RolledBackCleanly { pair })
        } else {
            Some(SubInvocationResult::Success { pair })
        }
    }

    /// Verifies storage rollback between two snapshots directly and emits a diagnostic if dirty.
    pub fn verify_storage_rollback(
        &mut self,
        host_fn_name: &str,
        contract_id: Option<&str>,
        before: &LedgerSnapshot,
        after_failure: &LedgerSnapshot,
    ) -> Option<&SubInvocationRollbackDiagnostic> {
        let snapshot_id = self.next_snapshot_id();
        let call_depth = (self.sub_invocation_stack.len() as u32).max(1);

        if let Some(diag) = verify_sub_invocation_rollback(
            host_fn_name,
            contract_id,
            call_depth,
            snapshot_id,
            before,
            after_failure,
        ) {
            let event = diag.to_diagnostic_event();
            self.diagnostic_events.push(event);
            self.rollback_diagnostics.push(diag);
            return self.rollback_diagnostics.last();
        }

        None
    }

    /// Returns current call stack depth of active sub-invocations.
    pub fn sub_invocation_depth(&self) -> u32 {
        self.sub_invocation_stack.len() as u32
    }

    /// Returns `true` if there are active sub-invocations currently on the stack.
    pub fn has_active_sub_invocations(&self) -> bool {
        !self.sub_invocation_stack.is_empty()
    }

    /// Returns a slice of the active sub-invocation frames.
    pub fn active_sub_invocations(&self) -> &[SubInvocationFrame] {
        &self.sub_invocation_stack
    }

    /// Returns all diagnostic events recorded by this tracker.
    pub fn diagnostic_events(&self) -> &[DiagnosticEvent] {
        &self.diagnostic_events
    }

    /// Takes all recorded diagnostic events out of the tracker.
    pub fn take_diagnostic_events(&mut self) -> Vec<DiagnosticEvent> {
        std::mem::take(&mut self.diagnostic_events)
    }

    /// Returns all structured rollback diagnostics recorded by this tracker.
    pub fn rollback_diagnostics(&self) -> &[SubInvocationRollbackDiagnostic] {
        &self.rollback_diagnostics
    }

    /// Returns `true` if any sub-invocation has failed rollback verification.
    pub fn has_unrolled_writes(&self) -> bool {
        !self.rollback_diagnostics.is_empty()
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
    use soroban_env_host::xdr::{
        AccountEntry, AccountId, LedgerEntry, LedgerEntryData, PublicKey, SequenceNumber,
        Thresholds, Uint256,
    };

    fn empty_snapshot() -> LedgerSnapshot {
        LedgerSnapshot::new()
    }

    fn make_test_entry(balance: i64) -> LedgerEntry {
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
        assert_eq!(format!("{id}"), "snap-42");
    }

    #[test]
    fn test_sub_invocation_clean_rollback() {
        let mut tracker = HostSnapshotTracker::new();
        let mut initial_state = LedgerSnapshot::new();
        initial_state.insert(KEY_A.to_vec(), make_test_entry(100));

        // Enter sub-invocation
        let snap_id = tracker.enter_sub_invocation(
            "child_contract_call",
            Some("CA123"),
            &initial_state,
            None,
        );
        assert_eq!(snap_id.as_u64(), 0);
        assert_eq!(tracker.sub_invocation_depth(), 1);

        // Child execution failed (e.g. trapped or caught by try_call),
        // and host properly rolled back storage to initial_state.
        let rolled_back_state = initial_state.clone();
        let result = tracker
            .exit_sub_invocation(&rolled_back_state, true, None)
            .expect("should return result");

        assert!(result.is_clean_rollback());
        assert!(!result.is_rollback_failed());
        assert!(!result.is_success());
        assert_eq!(tracker.sub_invocation_depth(), 0);
        assert!(tracker.diagnostic_events().is_empty());
        assert!(tracker.rollback_diagnostics().is_empty());
        assert!(!tracker.has_unrolled_writes());
    }

    #[test]
    fn test_sub_invocation_failed_rollback_emits_diagnostic() {
        let mut tracker = HostSnapshotTracker::new();
        let mut initial_state = LedgerSnapshot::new();
        initial_state.insert(KEY_A.to_vec(), make_test_entry(100));

        // Enter sub-invocation
        tracker.enter_sub_invocation(
            "child_contract_try_call",
            Some("CC_CHILD"),
            &initial_state,
            None,
        );

        // Child made changes: modified KEY_A, inserted KEY_B, and deleted KEY_C (if it existed)
        let mut dirty_state = initial_state.clone();
        dirty_state.insert(KEY_A.to_vec(), make_test_entry(999)); // modified
        dirty_state.insert(KEY_B.to_vec(), make_test_entry(200)); // inserted

        // Child call failed, BUT storage was NOT rolled back!
        let result = tracker
            .exit_sub_invocation(&dirty_state, true, None)
            .expect("should return result");

        assert!(result.is_rollback_failed());
        assert!(tracker.has_unrolled_writes());

        let diag = result.diagnostic().expect("diagnostic should be present");
        assert_eq!(diag.host_fn_name, "child_contract_try_call");
        assert_eq!(diag.contract_id.as_deref(), Some("CC_CHILD"));
        assert_eq!(diag.call_depth, 1);
        assert_eq!(diag.diff.total_changes(), 2);
        assert_eq!(diag.diff.inserted.len(), 1);
        assert_eq!(diag.diff.modified.len(), 1);
        assert_eq!(diag.diff.deleted.len(), 0);

        // Verify diagnostic event emitted into tracker
        assert_eq!(tracker.diagnostic_events().len(), 1);
        let event = &tracker.diagnostic_events()[0];
        assert_eq!(event.event_type, "diagnostic");
        assert_eq!(event.contract_id.as_deref(), Some("CC_CHILD"));
        assert!(event
            .topics
            .contains(&"storage_rollback_failed".to_string()));
        assert!(!event.in_successful_contract_call);
        assert!(event.data.contains("Storage rollback verification failed"));
    }

    #[test]
    fn test_nested_sub_invocations() {
        let mut tracker = HostSnapshotTracker::new();
        let initial_state = LedgerSnapshot::new();

        // Level 1: Parent call
        tracker.enter_sub_invocation("parent_call", Some("PARENT"), &initial_state, None);
        assert_eq!(tracker.sub_invocation_depth(), 1);

        // Parent writes KEY_A
        let mut parent_state = initial_state.clone();
        parent_state.insert(KEY_A.to_vec(), make_test_entry(50));

        // Level 2: Nested child try_call
        tracker.enter_sub_invocation("child_try_call", Some("CHILD"), &parent_state, None);
        assert_eq!(tracker.sub_invocation_depth(), 2);

        // Child writes KEY_B and fails, but KEY_B remains
        let mut child_dirty_state = parent_state.clone();
        child_dirty_state.insert(KEY_B.to_vec(), make_test_entry(100));

        // Exit child (failed rollback)
        let child_res = tracker
            .exit_sub_invocation(&child_dirty_state, true, None)
            .expect("should exit child");
        assert!(child_res.is_rollback_failed());
        assert_eq!(tracker.sub_invocation_depth(), 1);
        assert_eq!(tracker.rollback_diagnostics().len(), 1);
        assert_eq!(tracker.rollback_diagnostics()[0].call_depth, 2);

        // Parent catches the failure and successfully rolls back / overwrites state
        let parent_final_state = parent_state.clone();
        let parent_res = tracker
            .exit_sub_invocation(&parent_final_state, false, None)
            .expect("should exit parent");
        assert!(parent_res.is_success());
        assert_eq!(tracker.sub_invocation_depth(), 0);
    }

    #[test]
    fn test_sub_invocation_success_no_diagnostic() {
        let mut tracker = HostSnapshotTracker::new();
        let mut state = LedgerSnapshot::new();

        tracker.enter_sub_invocation("successful_call", None, &state, None);
        state.insert(KEY_A.to_vec(), make_test_entry(100));

        let res = tracker
            .exit_sub_invocation(&state, false, None)
            .expect("should succeed");
        assert!(res.is_success());
        assert!(tracker.diagnostic_events().is_empty());
        assert!(tracker.rollback_diagnostics().is_empty());
    }

    #[test]
    fn test_storage_write_set_diff_operations() {
        let mut snap1 = LedgerSnapshot::new();
        snap1.insert(KEY_A.to_vec(), make_test_entry(10));
        snap1.insert(KEY_B.to_vec(), make_test_entry(20));

        let mut snap2 = LedgerSnapshot::new();
        snap2.insert(KEY_A.to_vec(), make_test_entry(99)); // modified
        snap2.insert(KEY_C.to_vec(), make_test_entry(30)); // inserted
                                                           // KEY_B deleted

        let diff = diff_storage_write_sets(&snap1, &snap2);
        assert_eq!(diff.inserted.len(), 1);
        assert_eq!(diff.modified.len(), 1);
        assert_eq!(diff.deleted.len(), 1);
        assert_eq!(diff.total_changes(), 3);
        assert_eq!(diff.inserted_keys_hex(), vec![hex::encode(KEY_C)]);
        assert_eq!(diff.modified_keys_hex(), vec![hex::encode(KEY_A)]);
        assert_eq!(diff.deleted_keys_hex(), vec![hex::encode(KEY_B)]);
    }

    #[test]
    fn test_standalone_verify_sub_invocation_rollback() {
        let mut before = LedgerSnapshot::new();
        before.insert(KEY_A.to_vec(), make_test_entry(10));

        let clean_after = before.clone();
        let diag_clean = verify_sub_invocation_rollback(
            "clean_fn",
            Some("C1"),
            1,
            SnapshotId(0),
            &before,
            &clean_after,
        );
        assert!(diag_clean.is_none());

        let mut dirty_after = before.clone();
        dirty_after.insert(KEY_B.to_vec(), make_test_entry(20));
        let diag_dirty = verify_sub_invocation_rollback(
            "dirty_fn",
            Some("C1"),
            1,
            SnapshotId(1),
            &before,
            &dirty_after,
        )
        .expect("dirty rollback must yield diagnostic");

        assert_eq!(diag_dirty.diff.inserted.len(), 1);
        assert!(diag_dirty
            .message
            .contains("Storage rollback verification failed"));
        let event = diag_dirty.to_diagnostic_event();
        assert_eq!(event.event_type, "diagnostic");
        assert!(!event.in_successful_contract_call);
    }
}
