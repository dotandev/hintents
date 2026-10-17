// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! WASM execution engine trap interception and arithmetic error handling.
//!
//! Provides utilities to intercept runtime arithmetic traps (specifically integer divide-by-zero
//! and arithmetic overflows) emitted by the WASM execution engine, translating them into standard
//! Soroban contract failure errors ([`HostError`]) with [`ScErrorType::WasmVm`] and
//! [`ScErrorCode::ArithDomain`] to prevent fatal host crashes.

use soroban_env_host::{
    xdr::{ScErrorCode, ScErrorType},
    Error as EnvError, HostError,
};
use thiserror::Error;

/// Structured arithmetic trap errors emitted during WASM contract execution.
#[derive(Debug, Clone, PartialEq, Eq, Error)]
pub enum ArithmeticTrapError {
    /// Integer division or modulo by zero.
    #[error("integer divide by zero")]
    DivisionByZero,

    /// Integer overflow / underflow during arithmetic operation.
    #[error("integer arithmetic overflow")]
    IntegerOverflow,

    /// Invalid conversion to integer (e.g. out of range float-to-int).
    #[error("invalid conversion to integer")]
    InvalidConversion,
}

impl From<ArithmeticTrapError> for HostError {
    fn from(err: ArithmeticTrapError) -> Self {
        let code = match err {
            ArithmeticTrapError::DivisionByZero | ArithmeticTrapError::IntegerOverflow => {
                ScErrorCode::ArithDomain
            }
            ArithmeticTrapError::InvalidConversion => ScErrorCode::InvalidInput,
        };
        HostError::from(EnvError::from_type_and_code(ScErrorType::WasmVm, code))
    }
}

/// Determines whether a panic or error message represents a WASM divide-by-zero trap.
#[must_use]
pub fn is_divide_by_zero_trap(msg: &str) -> bool {
    let lower = msg.to_ascii_lowercase();
    lower.contains("division by zero")
        || lower.contains("divide by zero")
        || lower.contains("integer division by zero")
        || lower.contains("division by 0")
        || lower.contains("divide by 0")
}

/// Identifies an arithmetic trap from an error string if applicable.
#[must_use]
pub fn identify_arithmetic_trap(msg: &str) -> Option<ArithmeticTrapError> {
    let lower = msg.to_ascii_lowercase();
    if is_divide_by_zero_trap(&lower) {
        Some(ArithmeticTrapError::DivisionByZero)
    } else if lower.contains("integer overflow") || lower.contains("arithmetic overflow") {
        Some(ArithmeticTrapError::IntegerOverflow)
    } else if lower.contains("invalid conversion to int") {
        Some(ArithmeticTrapError::InvalidConversion)
    } else {
        None
    }
}

/// Intercepts an arithmetic panic or error emitted during execution and converts it
/// into a standard HostError.
#[allow(dead_code)]
pub fn catch_arithmetic_trap<T, F>(f: F) -> Result<T, HostError>
where
    F: FnOnce() -> Result<T, HostError>,
{
    match std::panic::catch_unwind(std::panic::AssertUnwindSafe(f)) {
        Ok(result) => match result {
            Ok(val) => Ok(val),
            Err(host_err) => {
                let err_str = format!("{host_err:?}");
                if let Some(trap) = identify_arithmetic_trap(&err_str) {
                    Err(trap.into())
                } else {
                    Err(host_err)
                }
            }
        },
        Err(panic_payload) => {
            let msg = if let Some(s) = panic_payload.downcast_ref::<String>() {
                s.as_str()
            } else if let Some(s) = panic_payload.downcast_ref::<&'static str>() {
                *s
            } else {
                "unknown panic"
            };

            if let Some(trap) = identify_arithmetic_trap(msg) {
                Err(trap.into())
            } else {
                std::panic::resume_unwind(panic_payload);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_is_divide_by_zero_trap() {
        assert!(is_divide_by_zero_trap("wasm trap: integer division by zero"));
        assert!(is_divide_by_zero_trap("fatal error: division by zero"));
        assert!(is_divide_by_zero_trap("attempted to divide by zero"));
        assert!(is_divide_by_zero_trap("cannot divide by 0"));
        assert!(!is_divide_by_zero_trap("out of bounds memory access"));
    }

    #[test]
    fn test_identify_arithmetic_trap() {
        assert_eq!(
            identify_arithmetic_trap("trap: integer division by zero"),
            Some(ArithmeticTrapError::DivisionByZero)
        );
        assert_eq!(
            identify_arithmetic_trap("error: integer overflow detected"),
            Some(ArithmeticTrapError::IntegerOverflow)
        );
        assert_eq!(
            identify_arithmetic_trap("invalid conversion to int"),
            Some(ArithmeticTrapError::InvalidConversion)
        );
        assert_eq!(identify_arithmetic_trap("unreachable code"), None);
    }

    #[test]
    fn test_arithmetic_trap_to_host_error_mapping() {
        let host_err: HostError = ArithmeticTrapError::DivisionByZero.into();
        let expected = EnvError::from_type_and_code(ScErrorType::WasmVm, ScErrorCode::ArithDomain);
        assert_eq!(host_err.error, expected);

        let overflow_err: HostError = ArithmeticTrapError::IntegerOverflow.into();
        assert_eq!(overflow_err.error, expected);
    }

    #[test]
    fn test_catch_arithmetic_trap_from_panic() {
        let caught = catch_arithmetic_trap::<(), _>(|| {
            panic!("wasm trap: integer division by zero");
        });
        assert!(caught.is_err());
        let err = caught.unwrap_err();
        let expected = EnvError::from_type_and_code(ScErrorType::WasmVm, ScErrorCode::ArithDomain);
        assert_eq!(err.error, expected);
    }

    #[test]
    fn test_catch_arithmetic_trap_ok_passthrough() {
        let result = catch_arithmetic_trap(|| Ok(42));
        assert_eq!(result.unwrap(), 42);
    }
}
