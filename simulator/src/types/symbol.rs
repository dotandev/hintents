// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Strict UTF-8 validation for Soroban `Symbol` values.
//!
//! A Soroban `Symbol` is a length-bounded byte string (at most
//! [`MAX_SYMBOL_LEN`] bytes, the XDR `SCSYMBOL_LIMIT`) that the protocol
//! requires to be valid UTF-8. The XDR decoder, however, only enforces the
//! length bound: `stellar-xdr` stores the payload as raw bytes inside a
//! `StringM<32>` and validates neither UTF-8 nor the `[a-zA-Z0-9_]` character
//! repertoire. A contract, a corrupt ledger entry, or a host event can
//! therefore carry a `Symbol` whose bytes are not valid UTF-8.
//!
//! Code that blindly converts those bytes with `str::from_utf8(bytes).unwrap()`
//! (or the host's unchecked `SymbolStr` conversion) can panic or invoke
//! undefined behaviour when the string is formatted. This module provides a
//! single, reusable validation boundary that must be applied whenever `Symbol`
//! bytes are decoded from WASM memory — host events, invocation results, ledger
//! entries — before they are turned into strings.

use std::fmt;

/// Maximum number of bytes a Soroban `Symbol` may contain.
///
/// Mirrors the XDR `SCSYMBOL_LIMIT` constant.
pub const MAX_SYMBOL_LEN: usize = 32;

/// Placeholder rendered when a `Symbol` payload is not a valid UTF-8 string.
pub const INVALID_SYMBOL_PLACEHOLDER: &str = "<invalid symbol>";

/// Reason a raw byte slice is not a valid Soroban `Symbol`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SymbolError {
    /// The payload is longer than [`MAX_SYMBOL_LEN`] bytes.
    TooLong {
        /// Actual length of the offending payload, in bytes.
        len: usize,
    },
    /// The payload is not valid UTF-8.
    InvalidUtf8 {
        /// Index of the first byte that does not begin a valid UTF-8 sequence.
        valid_up_to: usize,
        /// Length in bytes of the invalid sequence, if it is not a truncated
        /// tail at the end of the payload.
        error_len: Option<usize>,
    },
}

impl fmt::Display for SymbolError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::TooLong { len } => write!(
                f,
                "symbol is {len} bytes long, exceeding the {MAX_SYMBOL_LEN}-byte limit"
            ),
            Self::InvalidUtf8 {
                valid_up_to,
                error_len,
            } => match error_len {
                Some(len) => write!(
                    f,
                    "symbol is not valid UTF-8: invalid {len}-byte sequence at offset {valid_up_to}"
                ),
                None => write!(
                    f,
                    "symbol is not valid UTF-8: truncated sequence at offset {valid_up_to}"
                ),
            },
        }
    }
}

impl std::error::Error for SymbolError {}

/// Validates a raw Soroban `Symbol` payload and borrows it as a `&str`.
///
/// The payload is rejected if it is longer than [`MAX_SYMBOL_LEN`] bytes or if
/// its contents are not valid UTF-8.
///
/// # Errors
///
/// Returns [`SymbolError::TooLong`] when `bytes` exceeds the length limit, and
/// [`SymbolError::InvalidUtf8`] when the payload is not valid UTF-8.
pub fn validate_symbol(bytes: &[u8]) -> Result<&str, SymbolError> {
    if bytes.len() > MAX_SYMBOL_LEN {
        return Err(SymbolError::TooLong { len: bytes.len() });
    }
    std::str::from_utf8(bytes).map_err(|err| SymbolError::InvalidUtf8 {
        valid_up_to: err.valid_up_to(),
        error_len: err.error_len(),
    })
}

/// Returns `true` if `bytes` is a valid Soroban `Symbol` payload.
#[must_use]
pub fn is_valid_symbol(bytes: &[u8]) -> bool {
    validate_symbol(bytes).is_ok()
}

/// Validates a raw `Symbol` payload and copies it into an owned `String`.
///
/// # Errors
///
/// Propagates any [`SymbolError`] reported by [`validate_symbol`].
pub fn symbol_from_bytes(bytes: &[u8]) -> Result<String, SymbolError> {
    validate_symbol(bytes).map(str::to_owned)
}

/// Renders raw `Symbol` bytes for display without ever panicking.
///
/// Valid UTF-8 is returned unchanged; malformed payloads are replaced with
/// [`INVALID_SYMBOL_PLACEHOLDER`] so a single corrupt symbol can never take
/// down the formatter.
#[must_use]
pub fn symbol_to_string(bytes: &[u8]) -> String {
    validate_symbol(bytes)
        .map(str::to_owned)
        .unwrap_or_else(|_| INVALID_SYMBOL_PLACEHOLDER.to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_valid_ascii_symbol() {
        assert_eq!(validate_symbol(b"balance"), Ok("balance"));
        assert!(is_valid_symbol(b"transfer_from"));
        assert_eq!(symbol_from_bytes(b"mint").unwrap(), "mint");
    }

    #[test]
    fn accepts_empty_symbol() {
        assert_eq!(validate_symbol(b""), Ok(""));
        assert!(is_valid_symbol(b""));
    }

    #[test]
    fn accepts_valid_unicode_symbol() {
        let bytes = "café".as_bytes();
        assert_eq!(validate_symbol(bytes), Ok("café"));
        assert_eq!(symbol_from_bytes(bytes).unwrap(), "café");
    }

    #[test]
    fn accepts_symbol_at_exact_length_limit() {
        let bytes = vec![b'a'; MAX_SYMBOL_LEN];
        assert!(is_valid_symbol(&bytes));
    }

    #[test]
    fn rejects_symbol_exceeding_length_limit() {
        let bytes = vec![b'a'; MAX_SYMBOL_LEN + 1];
        assert_eq!(
            validate_symbol(&bytes),
            Err(SymbolError::TooLong {
                len: MAX_SYMBOL_LEN + 1
            })
        );
    }

    #[test]
    fn length_is_checked_before_utf8() {
        // 33 bytes of invalid UTF-8 must be reported as too long, not invalid.
        let bytes = vec![0xFF; MAX_SYMBOL_LEN + 1];
        assert_eq!(
            validate_symbol(&bytes),
            Err(SymbolError::TooLong {
                len: MAX_SYMBOL_LEN + 1
            })
        );
    }

    #[test]
    fn rejects_invalid_utf8() {
        let bytes = [b'f', 0x80, b'o'];
        assert_eq!(
            validate_symbol(&bytes),
            Err(SymbolError::InvalidUtf8 {
                valid_up_to: 1,
                error_len: Some(1),
            })
        );
        assert!(!is_valid_symbol(&bytes));
    }

    #[test]
    fn rejects_truncated_multibyte_sequence() {
        let bytes = [0xE2, 0x82];
        assert_eq!(
            validate_symbol(&bytes),
            Err(SymbolError::InvalidUtf8 {
                valid_up_to: 0,
                error_len: None,
            })
        );
    }

    #[test]
    fn rejects_overlong_encoding() {
        // 0xC0 is never a valid UTF-8 leading byte.
        let bytes = [0xC0, 0xAF];
        assert!(matches!(
            validate_symbol(&bytes),
            Err(SymbolError::InvalidUtf8 { valid_up_to: 0, .. })
        ));
    }

    #[test]
    fn rejects_utf8_encoded_surrogate() {
        // CESU-8 / UTF-8 encoded surrogate half must be rejected.
        let bytes = [0xED, 0xA0, 0x80];
        assert!(matches!(
            validate_symbol(&bytes),
            Err(SymbolError::InvalidUtf8 { .. })
        ));
    }

    #[test]
    fn symbol_to_string_uses_placeholder_for_invalid_bytes() {
        assert_eq!(symbol_to_string(&[0xFF, 0xFE]), INVALID_SYMBOL_PLACEHOLDER);
        assert_eq!(
            symbol_to_string(&[0xFF; MAX_SYMBOL_LEN + 1]),
            INVALID_SYMBOL_PLACEHOLDER
        );
    }

    #[test]
    fn symbol_to_string_preserves_valid_bytes() {
        assert_eq!(symbol_to_string(b"fn_call"), "fn_call");
        assert_eq!(symbol_to_string("café".as_bytes()), "café");
    }

    #[test]
    fn error_display_messages_are_descriptive() {
        let too_long = SymbolError::TooLong { len: 40 }.to_string();
        assert!(too_long.contains("40"));
        assert!(too_long.contains("32"));

        let invalid = SymbolError::InvalidUtf8 {
            valid_up_to: 3,
            error_len: Some(1),
        }
        .to_string();
        assert!(invalid.contains("UTF-8"));
        assert!(invalid.contains('3'));

        let truncated = SymbolError::InvalidUtf8 {
            valid_up_to: 1,
            error_len: None,
        }
        .to_string();
        assert!(truncated.contains("truncated"));
    }
}
