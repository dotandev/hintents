// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Memory-limit enforcement for the simulator runtime.
//!
//! Provides:
//!
//! * [`check_memory_limit`] — panics when the Soroban budget reports more
//!   memory consumed than the configured hard limit (mimicking live network
//!   constraints).
//! * [`MAX_DATA_SECTION_SIZE`] / [`MAX_DATA_SEGMENT_COUNT`] — compile-time
//!   caps used by [`crate::decoder`] to reject crafted WASM modules *before*
//!   any allocation takes place.
//! * [`check_data_section`] — convenience wrapper that converts a
//!   [`crate::decoder::DataSectionError`] into a `Result<(), String>`.
//!
//! # Allocator Rollback Safety
//!
//! Allocator-state tracking lives in [`crate::host::AllocTracker`]; this
//! module only handles the threshold checks.

use crate::decoder::{validate_data_section, DataSectionError};

/// Maximum combined byte size of all data-section initialisers in a single
/// WASM module.  Soroban's network limit for the entire contract binary is
/// 64 KiB; 128 KiB for data gives headroom for test fixtures while still
/// bounding adversarial inputs well below any OOM threshold.
pub const MAX_DATA_SECTION_SIZE: usize = 128 * 1024; // 128 KiB

/// Maximum number of individual data segments permitted in a WASM module.
/// Each segment incurs bookkeeping overhead regardless of its byte count,
/// so we cap segment count independently of total size.
pub const MAX_DATA_SEGMENT_COUNT: usize = 512;

/// Validate the data section of `wasm_bytes` against the hard limits defined
/// in this module.
///
/// This is a thin wrapper around [`validate_data_section`] that converts the
/// strongly-typed [`DataSectionError`] into a `Result<(), String>` suitable
/// for inline use in the simulation pipeline.
///
/// # Errors
///
/// Returns `Err(message)` when the data section exceeds [`MAX_DATA_SECTION_SIZE`],
/// [`MAX_DATA_SEGMENT_COUNT`], or when `wasm_bytes` cannot be parsed.
pub fn check_data_section(wasm_bytes: &[u8]) -> Result<(), String> {
    validate_data_section(wasm_bytes).map_err(|e: DataSectionError| e.to_string())
}

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

    #[test]
    fn test_check_data_section_valid_module() {
        let wasm = wat::parse_str("(module)").expect("wat parse failed");
        assert!(check_data_section(&wasm).is_ok());
    }

    #[test]
    fn test_check_data_section_oversized_returns_err_string() {
        // Build a module whose single data segment is 1 byte over the limit.
        let data_str = "\\00".repeat(MAX_DATA_SECTION_SIZE + 1);
        let wat_src = format!(
            r#"(module (memory 1) (data (i32.const 0) "{data_str}"))"#
        );
        let wasm = wat::parse_str(&wat_src).expect("wat parse failed");
        let err = check_data_section(&wasm).unwrap_err();
        assert!(
            err.contains("memory exhaustion"),
            "error string should mention memory exhaustion: {err}"
        );
    }

    #[test]
    fn test_max_data_section_size_is_positive() {
        assert!(MAX_DATA_SECTION_SIZE > 0);
    }

    #[test]
    fn test_max_data_segment_count_is_positive() {
        assert!(MAX_DATA_SEGMENT_COUNT > 0);
    }
}
