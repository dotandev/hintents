// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Before/After snapshot capture and async execution around host function calls.
//!
//! Every host function invocation produces a paired snapshot:
//! - **Before**: the ledger state immediately prior to the call
//! - **After**: the ledger state immediately after the call returns
//!
//! If the host function traps, the After snapshot is still recorded with
//! `trapped = true` so callers can inspect the state at the point of failure.
//!
//! Asynchronous execution helpers and mock latency configurations are provided
//! to better mock network latency and external contract calls.

#![allow(dead_code)]

use crate::snapshot::LedgerSnapshot;
use std::collections::HashMap;
use std::fmt;
use std::future::Future;
use std::time::Duration;

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

/// Mock configuration for asynchronous host function execution (e.g. network latency and failures).
#[derive(Debug, Clone, Default)]
pub struct AsyncHostConfig {
    /// Default latency applied to async host function calls if no specific latency is set.
    pub default_latency: Option<Duration>,
    /// Per-function latency overrides for simulating network latency on external contract or network calls.
    pub function_latencies: HashMap<String, Duration>,
    /// Per-function failure overrides to simulate network / remote host call errors.
    pub simulated_errors: HashMap<String, String>,
}

impl AsyncHostConfig {
    /// Creates a new empty async host configuration.
    pub fn new() -> Self {
        Self::default()
    }

    /// Sets default latency for all async host calls.
    pub fn with_default_latency(mut self, latency: Duration) -> Self {
        self.default_latency = Some(latency);
        self
    }

    /// Sets latency for a specific host function name.
    pub fn with_function_latency(mut self, host_fn_name: impl Into<String>, latency: Duration) -> Self {
        self.function_latencies.insert(host_fn_name.into(), latency);
        self
    }

    /// Sets simulated error for a specific host function name.
    pub fn with_simulated_error(
        mut self,
        host_fn_name: impl Into<String>,
        error_msg: impl Into<String>,
    ) -> Self {
        self.simulated_errors.insert(host_fn_name.into(), error_msg.into());
        self
    }

    /// Returns the latency configured for the given host function name, if any.
    pub fn get_latency(&self, host_fn_name: &str) -> Option<Duration> {
        self.function_latencies
            .get(host_fn_name)
            .copied()
            .or(self.default_latency)
    }

    /// Returns the simulated error configured for the given host function name, if any.
    pub fn get_simulated_error(&self, host_fn_name: &str) -> Option<&str> {
        self.simulated_errors.get(host_fn_name).map(String::as_str)
    }
}

/// Result of an async host function invocation.
#[derive(Debug)]
pub struct AsyncHostCallResult<T, E> {
    /// The return value or error from the function.
    pub result: Result<T, E>,
    /// The captured snapshot pair (before & after).
    pub snapshot_pair: Option<SnapshotPair>,
}

/// Manages snapshot capture around host function calls.
pub struct HostSnapshotTracker {
    next_id: u64,
    pairs: Vec<SnapshotPair>,
    /// Holds the "before" snapshot while a host function is in-flight.
    pending_before: Option<CapturedSnapshot>,
    /// Configuration for async network mocking and latency.
    async_config: AsyncHostConfig,
}

impl HostSnapshotTracker {
    /// Creates a new empty tracker.
    pub fn new() -> Self {
        Self {
            next_id: 0,
            pairs: Vec::new(),
            pending_before: None,
            async_config: AsyncHostConfig::default(),
        }
    }

    /// Creates a new tracker with the specified async configuration.
    pub fn with_async_config(async_config: AsyncHostConfig) -> Self {
        Self {
            next_id: 0,
            pairs: Vec::new(),
            pending_before: None,
            async_config,
        }
    }

    /// Returns a reference to the tracker's async configuration.
    pub fn async_config(&self) -> &AsyncHostConfig {
        &self.async_config
    }

    /// Returns a mutable reference to the tracker's async configuration.
    pub fn async_config_mut(&mut self) -> &mut AsyncHostConfig {
        &mut self.async_config
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

    /// Executes an asynchronous host function call with snapshot tracking and optional latency simulation.
    ///
    /// This method:
    /// 1. Takes a "before" snapshot of the ledger state.
    /// 2. Simulates network latency if configured in `AsyncHostConfig`.
    /// 3. Awaits the provided async host function future.
    /// 4. Takes an "after" snapshot using `after_state_fn` (or the initial state if failed and no state provided),
    ///    marking `trapped = true` if the future resolved to an `Err`.
    ///
    /// # Arguments
    /// * `host_fn_name` - Name of the host function being called.
    /// * `before_state` - Ledger snapshot before calling the function.
    /// * `host_fn` - The asynchronous closure returning a future.
    /// * `after_state_fn` - Closure to extract the resulting `LedgerSnapshot` after completion.
    pub async fn execute_async<F, Fut, T, E, S>(
        &mut self,
        host_fn_name: &str,
        before_state: LedgerSnapshot,
        host_fn: F,
        after_state_fn: S,
    ) -> AsyncHostCallResult<T, E>
    where
        F: FnOnce() -> Fut,
        Fut: Future<Output = Result<T, E>>,
        S: FnOnce(&Result<T, E>) -> (LedgerSnapshot, bool),
    {
        self.take_before_snapshot(host_fn_name, before_state);

        if let Some(latency) = self.async_config.get_latency(host_fn_name) {
            tokio::time::sleep(latency).await;
        }

        let result = host_fn().await;
        let (after_state, trapped) = after_state_fn(&result);
        let pair = self.take_after_snapshot(after_state, trapped).cloned();

        AsyncHostCallResult {
            result,
            snapshot_pair: pair,
        }
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

    #[tokio::test]
    async fn test_async_host_execution_success() {
        let mut tracker = HostSnapshotTracker::new();

        let call_res = tracker
            .execute_async(
                "call_contract_remote",
                empty_snapshot(),
                || async {
                    // Simulate async work (e.g. network call)
                    tokio::time::sleep(Duration::from_millis(10)).await;
                    Ok::<u32, &'static str>(42)
                },
                |res| (empty_snapshot(), res.is_err()),
            )
            .await;

        assert_eq!(call_res.result, Ok(42));
        assert!(call_res.snapshot_pair.is_some());
        let pair = call_res.snapshot_pair.unwrap();
        assert_eq!(pair.before.host_fn_name, "call_contract_remote");
        assert_eq!(pair.after.host_fn_name, "call_contract_remote");
        assert!(!pair.after.trapped);
        assert_eq!(tracker.pair_count(), 1);
        assert!(!tracker.has_pending());
    }

    #[tokio::test]
    async fn test_async_host_execution_trap() {
        let mut tracker = HostSnapshotTracker::new();

        let call_res = tracker
            .execute_async(
                "failing_remote_call",
                empty_snapshot(),
                || async {
                    tokio::time::sleep(Duration::from_millis(5)).await;
                    Err::<u32, &'static str>("network failure")
                },
                |res| (empty_snapshot(), res.is_err()),
            )
            .await;

        assert_eq!(call_res.result, Err("network failure"));
        assert!(call_res.snapshot_pair.is_some());
        let pair = call_res.snapshot_pair.unwrap();
        assert_eq!(pair.before.host_fn_name, "failing_remote_call");
        assert!(pair.after.trapped);
        assert_eq!(tracker.pair_count(), 1);
    }

    #[tokio::test]
    async fn test_async_host_latency_mocking() {
        let config = AsyncHostConfig::new()
            .with_default_latency(Duration::from_millis(5))
            .with_function_latency("slow_external_contract", Duration::from_millis(30))
            .with_simulated_error("unreachable_contract", "connection timeout");

        let mut tracker = HostSnapshotTracker::with_async_config(config);

        assert_eq!(
            tracker.async_config().get_latency("slow_external_contract"),
            Some(Duration::from_millis(30))
        );
        assert_eq!(
            tracker.async_config().get_latency("normal_call"),
            Some(Duration::from_millis(5))
        );
        assert_eq!(
            tracker.async_config().get_simulated_error("unreachable_contract"),
            Some("connection timeout")
        );

        let start = std::time::Instant::now();
        let call_res = tracker
            .execute_async(
                "slow_external_contract",
                empty_snapshot(),
                || async { Ok::<&str, &str>("ok") },
                |res| (empty_snapshot(), res.is_err()),
            )
            .await;

        assert!(start.elapsed() >= Duration::from_millis(25));
        assert_eq!(call_res.result, Ok("ok"));
        assert_eq!(tracker.pair_count(), 1);
    }
}

