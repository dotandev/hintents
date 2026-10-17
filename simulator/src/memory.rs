// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Memory-limit enforcement and WASM memory bounds mapping for the simulator runtime.
//!
//! Provides boundary validation, trap-to-HostError mapping for out-of-bounds linear memory
//! accesses, and [`check_memory_limit`] thresholds used by the runtime (mimicking live
//! Soroban network constraints).

use soroban_env_host::{
    xdr::{ScErrorCode, ScErrorType},
    Error as EnvError, HostError,
};

/// Structured representation of memory boundary and allocation errors during simulation.
#[derive(Debug, thiserror::Error, Clone, PartialEq, Eq)]
pub enum MemoryBoundsError {
    /// Attempted memory access exceeded available linear memory bounds.
    #[error("WASM out-of-bounds memory access: offset {offset} + len {len} > memory size {memory_size}")]
    OutOfBounds {
        offset: u64,
        len: u64,
        memory_size: u64,
    },
    /// Memory consumption exceeded the configured simulator limit.
    #[error("Memory limit exceeded: consumed {consumed} bytes, limit {limit} bytes")]
    LimitExceeded {
        consumed: u64,
        limit: u64,
    },
}

impl From<MemoryBoundsError> for HostError {
    fn from(err: MemoryBoundsError) -> Self {
        match err {
            MemoryBoundsError::OutOfBounds { .. } => {
                // Map out-of-bounds memory trap to Soroban WasmVm IndexBounds error
                EnvError::from_type_and_code(ScErrorType::WasmVm, ScErrorCode::IndexBounds).into()
            }
            MemoryBoundsError::LimitExceeded { .. } => {
                // Map resource exhaustion to Budget Exceeded error
                EnvError::from_type_and_code(ScErrorType::Budget, ScErrorCode::ExceededLimit).into()
            }
        }
    }
}

/// Validates whether an access slice `[offset, offset + len)` is safely within `memory_size`.
#[allow(dead_code)]
pub fn validate_memory_access(
    offset: u64,
    len: u64,
    memory_size: u64,
) -> Result<(), MemoryBoundsError> {
    let end = offset
        .checked_add(len)
        .ok_or(MemoryBoundsError::OutOfBounds {
            offset,
            len,
            memory_size,
        })?;

    if end > memory_size {
        return Err(MemoryBoundsError::OutOfBounds {
            offset,
            len,
            memory_size,
        });
    }

    Ok(())
}

/// Inspects an engine trap or panic message and maps out-of-bounds traps into a structured [`MemoryBoundsError`].
///
/// Returns `Some(MemoryBoundsError)` if the trap represents an out-of-bounds memory access
/// or allocation limit, or `None` if the error is unrelated.
pub fn map_memory_trap(message: &str) -> Option<MemoryBoundsError> {
    let lower = message.to_ascii_lowercase();
    if lower.contains("out of bounds memory access")
        || lower.contains("memory access out of bounds")
        || lower.contains("out of bounds")
        || lower.contains("wasm memory trap")
    {
        Some(MemoryBoundsError::OutOfBounds {
            offset: 0,
            len: 0,
            memory_size: 0,
        })
    } else if lower.contains("memory_limit_exceeded") || lower.contains("memory limit exceeded") {
        Some(MemoryBoundsError::LimitExceeded {
            consumed: 0,
            limit: 0,
        })
    } else {
        None
    }
}

/// Helper that converts any trapped memory panic or error into a typed [`HostError`].
pub fn catch_memory_trap_to_host_error(message: &str) -> Option<HostError> {
    map_memory_trap(message).map(HostError::from)
}

/// Result-returning version of memory limit checking.
pub fn check_memory_limit_result(consumed: u64, limit: u64) -> Result<(), MemoryBoundsError> {
    if consumed > limit {
        Err(MemoryBoundsError::LimitExceeded { consumed, limit })
    } else {
        Ok(())
    }
}

/// Checks whether the current memory consumption exceeds the configured hard limit.
///
/// # Panics
///
/// Panics with a diagnostic message when `consumed > limit`.
pub fn check_memory_limit(consumed: u64, limit: u64) {
    if check_memory_limit_result(consumed, limit).is_err() {
        panic!("ERR_MEMORY_LIMIT_EXCEEDED: consumed {consumed} bytes, limit {limit} bytes");
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_check_memory_limit_within_bounds() {
        assert!(check_memory_limit_result(500, 1000).is_ok());
        check_memory_limit(500, 1000);
    }

    #[test]
    fn test_check_memory_limit_at_boundary() {
        assert!(check_memory_limit_result(1000, 1000).is_ok());
        check_memory_limit(1000, 1000);
    }

    #[test]
    fn test_check_memory_limit_exceeded_panics() {
        let result = std::panic::catch_unwind(|| check_memory_limit(1001, 1000));
        assert!(result.is_err(), "expected panic when memory exceeds limit");
    }

    #[test]
    fn test_validate_memory_access_success() {
        assert!(validate_memory_access(0, 64, 1024).is_ok());
        assert!(validate_memory_access(1000, 24, 1024).is_ok());
    }

    #[test]
    fn test_validate_memory_access_out_of_bounds() {
        let err = validate_memory_access(1000, 25, 1024).unwrap_err();
        assert_eq!(
            err,
            MemoryBoundsError::OutOfBounds {
                offset: 1000,
                len: 25,
                memory_size: 1024
            }
        );
    }

    #[test]
    fn test_validate_memory_access_overflow() {
        let err = validate_memory_access(u64::MAX, 1, 1024).unwrap_err();
        assert_eq!(
            err,
            MemoryBoundsError::OutOfBounds {
                offset: u64::MAX,
                len: 1,
                memory_size: 1024
            }
        );
    }

    #[test]
    fn test_map_memory_trap_recognition() {
        let err1 = map_memory_trap("wasm trap: out of bounds memory access").unwrap();
        assert!(matches!(err1, MemoryBoundsError::OutOfBounds { .. }));

        let err2 = map_memory_trap("panic: ERR_MEMORY_LIMIT_EXCEEDED: consumed 200").unwrap();
        assert!(matches!(err2, MemoryBoundsError::LimitExceeded { .. }));

        assert!(map_memory_trap("unrelated arithmetic error").is_none());
    }

    #[test]
    fn test_memory_bounds_error_to_host_error() {
        let oob = MemoryBoundsError::OutOfBounds {
            offset: 10,
            len: 5,
            memory_size: 12,
        };
        let host_err: HostError = oob.into();
        assert_eq!(
            host_err.error,
            EnvError::from_type_and_code(ScErrorType::WasmVm, ScErrorCode::IndexBounds)
        );

        let limit = MemoryBoundsError::LimitExceeded {
            consumed: 200,
            limit: 100,
        };
        let host_err2: HostError = limit.into();
        assert_eq!(
            host_err2.error,
            EnvError::from_type_and_code(ScErrorType::Budget, ScErrorCode::ExceededLimit)
        );
    }

    #[test]
    fn test_catch_memory_trap_to_host_error() {
        let host_err = catch_memory_trap_to_host_error("Memory access out of bounds").unwrap();
        assert_eq!(
            host_err.error,
            EnvError::from_type_and_code(ScErrorType::WasmVm, ScErrorCode::IndexBounds)
        );

        assert!(catch_memory_trap_to_host_error("some other error").is_none());
    }
}
