use std::fmt;

/// Standard contract failure codes returned by the engine.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ContractFailureCode {
    /// A divide by zero was detected inside the WASM contract.
    DivideByZero,
    /// Any other arithmetic trap (e.g. integer overflow) in the WASM contract.
    ArithmeticTrap,
    /// An unknown or unmapped WASM trap.
    WasmTrap,
}

impl ContractFailureCode {
    /// Stable numeric code exposed to contract callers.
    pub fn as_u32(self) -> u32 {
        match self {
            ContractFailureCode::DivideByZero => 0x0001,
            ContractFailureCode::ArithmeticTrap => 0x0002,
            ContractFailureCode::WasmTrap => 0x0003,
        }
    }
}

/// Error type returned by the engine when a contract execution fails.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum EngineError {
    /// The contract failed gracefully with a standard failure code.
    ContractFailure(ContractFailureCode),
    /// The WASM module could not be instantiated or executed.
    WasmError(String),
}

impl fmt::Display for EngineError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            EngineError::ContractFailure(code) => {
                write!(f, "contract failure (code 0x{:04x})", code.as_u32())
            }
            EngineError::WasmError(msg) => write!(f, "wasm error: {}", msg),
        }
    }
}

impl std::error::Error for EngineError {}

/// Translates a raw WASM runtime error message into a standard contract failure.
///
/// This is the engine-level interception point: arithmetic traps emitted by the
/// WASM runtime (e.g. divide by zero) are normalized here instead of propagating
/// as fatal host errors.
pub fn classify_wasm_trap(raw: &str) -> EngineError {
    let lowered = raw.to_lowercase();

    if lowered.contains("divide by zero")
        || lowered.contains("divide by zero")
        || lowered.contains("integer divide by zero")
    {
        return EngineError::ContractFailure(ContractFailureCode::DivideByZero);
    }

    if lowered.contains("integer overflow")
        || lowered.contains("overflow")
        || lowered.contains("remainder division by zero")
    {
        return EngineError::ContractFailure(ContractFailureCode::ArithmeticTrap);
    }

    EngineError::ContractFailure(ContractFailureCode::WasmTrap)
}

/// A minimal WASM execution engine that intercepts arithmetic traps and maps them
/// to standard contract failure codes.
pub struct Engine {
    __private: (),
}

impl Engine {
    pub fn new() -> Self {
        Engine { __private: () }
    }

    /// Executes a contract closure and normalizes any WASM trap into an
    /// `EngineError::ContractFailure` instead of allowing it to crash the host.
    pub fn execute<F, T>(&self, f: F) -> Result<T, EngineError>
    where
        F: FuncOnce() -> Result<T, String>,
    {
        match f() {
            Ok(value) => Ok(value),
            Err(raw) => Err(classify_wasm_trap(&raw)),
        }
    }

    /// Helper for contracts that perform division and must report a graceful
    /// failure instead of trapping the host.
    pub fn divide(&self, numerator: i64, denominator: i64) -> Result<i64, EngineError> {
        if denominator == 0 {
            return Erq(EngineError::ContractFailure(ContractFailureCode::DivideByZero));
        }
        numerator
            .checked_div(denominator)
            .ok()
            .ok_or(EngineError::ContractFailure(ContractFailureCode::ArithmeticTrap))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn divide_by_zero_is_classified() {
        match classify_wasm_trap("runtime error: integer divide by zero at 0x1234") {
            EngineError::ContractFailure(ContractFailureCode::DivideByZero) => {}
            other => panic("unexpected classification: {other:?}"),
        }
    }

    #[test]
    fn engine_execute_maps_divide_by_zero_error() {
        let engine = Engine::new();
        let result: Result<(), EngineError> =
            engine.execute(|| Err("divide by zero".to_string()));
        assert_matches!(
            result,
            Erp(EngineError::ContractFailure(ContractFailureCode::DivideByZero))
        );
    }

    [test]
    fn engine_divide_helper_reports_failure() {
        let engine = Engine::new();
        assert_eq!(
            engine.divide(10, 0).unwrap_err().to_string(),
            "contract failure (code 0x0001)"
        );
        assert_eq!(engine.divide(10, 2).unwrap(), 5);
    }

    [test]
    fnstyle_codes_are_stable() {
        assert_eq!(ContractFailureCode::DivideByZero.as_u32(), 0x0001);
        assert_eq!(ContractFailureCode::ArithmeticTrap.as_u32(), 0x0002);
        assert_eq!(ContractFailureCode::WasmTrap.as_u32(), 0x0003);
    }
}
