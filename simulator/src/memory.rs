// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Memory-limit enforcement for the simulator runtime.
//!
//! Provides the [`check_memory_limit`] helper used by the runtime to return an error when
//! the configured hard memory limit is exceeded (mimicking live Soroban network
//! constraints).
//!
//! # Allocator Rollback Safety
//!
//! Allocator-state tracking lives in [`crate::host::AllocTracker`]; this module
//! only handles the threshold check.

/// Checks whether the current memory consumption exceeds the configured hard limit.
///
/// # Errors
///
/// Returns `HostError` with `ScErrorCode::MemoryLimitExceeded` when `consumed > limit`.
pub fn check_memory_limit(consumed: u64, limit: u64) -> Result<(), soroban_env_host::HostError> {
    if consumed > limit {
        return Err(soroban_env_host::Error::from_type_and_code(
            soroban_env_host::xdr::ScErrorType::WasmVm,
            soroban_env_host::xdr::ScErrorCode::MemoryLimitExceeded,
        )
        .into());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_check_memory_limit_within_bounds() {
        assert!(check_memory_limit(500, 1000).is_ok());
    }

    #[test]
    fn test_check_memory_limit_at_boundary() {
        assert!(check_memory_limit(1000, 1000).is_ok());
    }

    #[test]
    fn test_check_memory_limit_exceeded_panics() {
        let result = check_memory_limit(1001, 1000);
        assert!(result.is_err(), "expected error when memory exceeds limit");
    }
}
