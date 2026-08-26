// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Proactive WASM data-section bounds checking.
//!
//! A malicious actor can craft a WASM module whose declared data section
//! claims an enormous amount of initialiser bytes.  Naively handing such a
//! module to a parser or runtime allocates memory proportional to the
//! declared size *before* any execution budget kicks in, exhausting the
//! simulator process and potentially the host OS.
//!
//! [`validate_data_section`] walks only the `DataSection` payload of the
//! WASM binary — it does not instantiate the module — and returns an error
//! if either:
//!
//! * the total initialiser byte count exceeds [`crate::memory::MAX_DATA_SECTION_SIZE`], or
//! * the number of individual data segments exceeds [`crate::memory::MAX_DATA_SEGMENT_COUNT`].
//!
//! Call this function on raw WASM bytes **before** passing them to any
//! higher-level parser, runtime, or source-map analyser.

use crate::memory::{MAX_DATA_SECTION_SIZE, MAX_DATA_SEGMENT_COUNT};
use wasmparser::{DataKind, Parser, Payload};

/// Errors that can be returned by [`validate_data_section`].
#[derive(Debug, PartialEq, Eq)]
pub enum DataSectionError {
    /// The WASM binary could not be parsed.
    ParseError(String),
    /// The number of data segments exceeds [`MAX_DATA_SEGMENT_COUNT`].
    TooManySegments {
        /// Actual segment count found in the module.
        count: usize,
        /// Configured segment limit.
        limit: usize,
    },
    /// The combined initialiser data size exceeds [`MAX_DATA_SECTION_SIZE`].
    TotalSizeTooLarge {
        /// Total bytes found in all data segments.
        total: usize,
        /// Configured byte limit.
        limit: usize,
    },
}

impl std::fmt::Display for DataSectionError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            DataSectionError::ParseError(msg) => {
                write!(f, "failed to parse WASM data section: {msg}")
            }
            DataSectionError::TooManySegments { count, limit } => write!(
                f,
                "WASM data section contains {count} segments (limit {limit}): \
                 possible memory exhaustion attack"
            ),
            DataSectionError::TotalSizeTooLarge { total, limit } => write!(
                f,
                "WASM data section total size is {total} bytes (limit {limit}): \
                 possible memory exhaustion attack"
            ),
        }
    }
}

impl std::error::Error for DataSectionError {}

/// Validate the data section of a WASM module *without* instantiating it.
///
/// Iterates over every `DataSection` payload in `wasm_bytes` and checks:
///
/// 1. That the segment count does not exceed [`MAX_DATA_SEGMENT_COUNT`].
/// 2. That the combined initialiser byte count does not exceed [`MAX_DATA_SECTION_SIZE`].
///
/// Both checks happen incrementally — the function short-circuits on the
/// first violation so we never accumulate more state than necessary.
///
/// # Errors
///
/// Returns [`DataSectionError`] when a limit is exceeded or when the bytes
/// cannot be parsed as a valid WASM module.
pub fn validate_data_section(wasm_bytes: &[u8]) -> Result<(), DataSectionError> {
    let mut segment_count: usize = 0;
    let mut total_size: usize = 0;

    for payload in Parser::new(0).parse_all(wasm_bytes) {
        let payload = payload.map_err(|e| DataSectionError::ParseError(e.to_string()))?;

        if let Payload::DataSection(reader) = payload {
            for data in reader {
                let data = data.map_err(|e| DataSectionError::ParseError(e.to_string()))?;

                // Count only active and passive segments; both consume memory on
                // instantiation.  Declared segments (DataKind::Passive with no
                // init expr) are included because they still back the data count
                // import and can be bulk-copied at runtime.
                let seg_len = match &data.kind {
                    DataKind::Active { .. } | DataKind::Passive => data.data.len(),
                };

                segment_count = segment_count.saturating_add(1);
                if segment_count > MAX_DATA_SEGMENT_COUNT {
                    return Err(DataSectionError::TooManySegments {
                        count: segment_count,
                        limit: MAX_DATA_SEGMENT_COUNT,
                    });
                }

                total_size = total_size.saturating_add(seg_len);
                if total_size > MAX_DATA_SECTION_SIZE {
                    return Err(DataSectionError::TotalSizeTooLarge {
                        total: total_size,
                        limit: MAX_DATA_SECTION_SIZE,
                    });
                }
            }
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    // ------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------

    /// Minimal valid WASM module with no data section.
    fn empty_module() -> Vec<u8> {
        wat::parse_str("(module)").expect("wat parse failed")
    }

    /// WASM module with one data segment of the given byte count.
    fn module_with_data(byte_count: usize) -> Vec<u8> {
        // Build a module that has a memory and a single active data segment.
        // The segment is filled with zeros.
        let data_str = "\\00".repeat(byte_count);
        let wat = format!(
            r#"(module
                (memory 1)
                (data (i32.const 0) "{data_str}")
            )"#
        );
        wat::parse_str(&wat).expect("wat parse failed")
    }

    /// WASM module with `n` data segments each containing one byte.
    fn module_with_n_segments(n: usize) -> Vec<u8> {
        // Each segment writes a zero byte at offset `i`.
        let mut segments = String::new();
        for i in 0..n {
            segments.push_str(&format!(r#"(data (i32.const {i}) "\00")"#));
            segments.push('\n');
        }
        let wat = format!(
            r#"(module
                (memory 1)
                {segments}
            )"#
        );
        wat::parse_str(&wat).expect("wat parse failed")
    }

    // ------------------------------------------------------------------
    // Positive cases
    // ------------------------------------------------------------------

    #[test]
    fn test_valid_empty_module() {
        assert!(validate_data_section(&empty_module()).is_ok());
    }

    #[test]
    fn test_valid_small_data_segment() {
        // 1 KiB of data — well within limits
        let wasm = module_with_data(1024);
        assert!(validate_data_section(&wasm).is_ok());
    }

    #[test]
    fn test_valid_data_at_exact_size_limit() {
        let wasm = module_with_data(MAX_DATA_SECTION_SIZE);
        assert!(validate_data_section(&wasm).is_ok());
    }

    #[test]
    fn test_valid_segments_at_exact_count_limit() {
        let wasm = module_with_n_segments(MAX_DATA_SEGMENT_COUNT);
        assert!(validate_data_section(&wasm).is_ok());
    }

    // ------------------------------------------------------------------
    // Negative cases — size
    // ------------------------------------------------------------------

    #[test]
    fn test_rejects_data_section_one_byte_over_limit() {
        let wasm = module_with_data(MAX_DATA_SECTION_SIZE + 1);
        let err = validate_data_section(&wasm).unwrap_err();
        assert!(
            matches!(err, DataSectionError::TotalSizeTooLarge { .. }),
            "expected TotalSizeTooLarge, got: {err}"
        );
    }

    #[test]
    fn test_rejects_data_section_far_over_limit() {
        // 2× limit
        let wasm = module_with_data(MAX_DATA_SECTION_SIZE * 2);
        let err = validate_data_section(&wasm).unwrap_err();
        assert!(matches!(err, DataSectionError::TotalSizeTooLarge { .. }));
    }

    #[test]
    fn test_error_message_contains_totals_for_size() {
        let over = MAX_DATA_SECTION_SIZE + 1;
        let wasm = module_with_data(over);
        let err = validate_data_section(&wasm).unwrap_err();
        let msg = err.to_string();
        assert!(
            msg.contains("bytes"),
            "error message should mention byte count: {msg}"
        );
        assert!(
            msg.contains("memory exhaustion"),
            "error message should call out memory exhaustion: {msg}"
        );
    }

    // ------------------------------------------------------------------
    // Negative cases — segment count
    // ------------------------------------------------------------------

    #[test]
    fn test_rejects_too_many_segments() {
        let wasm = module_with_n_segments(MAX_DATA_SEGMENT_COUNT + 1);
        let err = validate_data_section(&wasm).unwrap_err();
        assert!(
            matches!(err, DataSectionError::TooManySegments { .. }),
            "expected TooManySegments, got: {err}"
        );
    }

    #[test]
    fn test_error_message_contains_count_for_segments() {
        let wasm = module_with_n_segments(MAX_DATA_SEGMENT_COUNT + 1);
        let err = validate_data_section(&wasm).unwrap_err();
        let msg = err.to_string();
        assert!(
            msg.contains("segments"),
            "error message should mention segment count: {msg}"
        );
        assert!(
            msg.contains("memory exhaustion"),
            "error message should call out memory exhaustion: {msg}"
        );
    }

    // ------------------------------------------------------------------
    // Edge case — invalid WASM bytes
    // ------------------------------------------------------------------

    #[test]
    fn test_rejects_invalid_wasm_bytes() {
        let junk = b"this is not wasm at all";
        let err = validate_data_section(junk).unwrap_err();
        assert!(
            matches!(err, DataSectionError::ParseError(_)),
            "expected ParseError, got: {err}"
        );
    }
}
