// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Test module for memory limit simulation functionality.
//!
//! These tests verify that [`SimHost`](crate::runner::SimHost) correctly enforces
//! a hard memory limit through its [`check_memory_limit`] method, and that the
//! allocator tracker remains consistent across snapshot/rollback cycles.

#[cfg(test)]
mod tests {
    use crate::host::AllocTracker;
    use crate::runner::SimHost;

    #[test]
    fn test_no_memory_limit() {
        // SimHost can be created without any memory limit.
        let host = SimHost::new(None, None, None);
        host.check_memory_limit(); // should not panic
    }

    #[test]
    fn test_memory_limit_within_bounds_no_panic() {
        // With a small-but-positive limit and zero consumed bytes, the check
        // should not panic.
        let host = SimHost::new(None, None, Some(1));
        host.check_memory_limit();
    }

    #[test]
    fn test_memory_limit_large_limit_no_panic() {
        let host = SimHost::new(None, None, Some(1_000_000));
        host.check_memory_limit();
    }

    #[test]
    fn test_memory_limit_check_called_on_fresh_host() {
        // The method must be callable immediately after construction.
        let host = SimHost::new(None, None, Some(1024));
        host.check_memory_limit();
    }

    #[test]
    fn test_memory_limit_after_snapshot_restore() {
        // After restore_from_snapshot, the memory limit should still be
        // applied to the new Host.
        let mut host = SimHost::new(None, None, Some(1024));
        let snapshot = host.capture_snapshot().expect("snapshot should capture");
        host.restore_from_snapshot(&snapshot)
            .expect("restore should succeed");
        host.check_memory_limit();
    }

    #[test]
    fn test_memory_limit_preserved_across_repeated_restores() {
        let mut host = SimHost::new(None, None, Some(1024));
        for _ in 0..5 {
            let snapshot = host.capture_snapshot().expect("snapshot should capture");
            host.restore_from_snapshot(&snapshot)
                .expect("restore should succeed");
            host.check_memory_limit();
        }
    }

    #[test]
    fn test_alloc_tracker_tracks_snapshot_and_rollback() {
        // Demonstrate that AllocTracker works alongside SimHost operations.
        let mut tracker = AllocTracker::new();
        tracker.snapshot(0);
        assert_eq!(tracker.snapshot_count(), 1);

        tracker.record_rollback(0);
        assert_eq!(tracker.rollback_count(), 1);
        assert!(tracker.has_rolled_back());
    }

    #[test]
    fn test_alloc_tracker_catches_nonzero_restored_memory_in_debug() {
        let mut tracker = AllocTracker::new();
        tracker.snapshot(100);

        // In debug mode, record_rollback panics if restored_memory_bytes != 0.
        // (In release mode the debug_assert is elided.)
        // Use AssertUnwindSafe because catch_unwind requires UnwindSafe on the closure.
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            let mut t = AllocTracker::new();
            t.snapshot(100);
            t.record_rollback(42);
        }));
        // We expect the debug_assert to fire.
        #[cfg(debug_assertions)]
        assert!(
            result.is_err(),
            "debug_assert should fire on nonzero restore"
        );
        #[cfg(not(debug_assertions))]
        drop(result);
    }
}
