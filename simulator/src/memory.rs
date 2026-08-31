// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Memory-limit enforcement for the simulator runtime.
//!
//! Provides the [`check_memory_limit`] helper used by the runtime to panic when
//! the configured hard memory limit is exceeded (mimicking live Soroban network
//! constraints).
//!
//! # Allocator Rollback Safety
//!
//! Allocator-state tracking lives in [`crate::host::AllocTracker`]; this module
//! only handles the threshold check.

/// Checks whether the current memory consumption exceeds the configured hard limit.
///
/// # Panics
///
/// Panics with a diagnostic message when `consumed > limit`.
pub fn check_memory_limit(consumed: u64, limit: u64) {
    if consumed > limit {
        panic!("ERR_MEMORY_LIMIT_EXCEEDED: consumed {consumed} bytes, limit {limit} bytes");
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_check_memory_limit_within_bounds() {
        // Should not panic
        check_memory_limit(500, 1000);
    }

    #[test]
    fn test_check_memory_limit_at_boundary() {
        // Should not panic — exactly at limit
        check_memory_limit(1000, 1000);
    }

    #[test]
    fn test_check_memory_limit_exceeded_panics() {
        let result = std::panic::catch_unwind(|| check_memory_limit(1001, 1000));
        assert!(result.is_err(), "expected panic when memory exceeds limit");
    }
}
