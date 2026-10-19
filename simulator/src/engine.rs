use std::fmt;

/// Standard contract failure codes returned by the engine.
///
/// These codes are stable and may be matched on by callers of the
/// simulator. New codes may be added but existing ones must not change
/// meaning.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pubres enum ContractFailureCode {
    /// Generic failure with no more specific classification.
    Generic = 1,
    /// The contract executed an invalid arithmetic operation,
    /// such as dividing by zero.
    ArithmeticTrap = 2,
    /// The contract exhausted its execution budget.
    OutOfGas = 3,
    /// The contract panicked or hit an unrecoverable runtime error.
    RuntimeError = 4,
}

/// Error type produced by the engine when executing a contract.
///
/// This is the boundary between raw WASM runtime errors and the
/// contract-level failure codes that callers can handle gracefully.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum EngineError {
    /// The contract failed with a standard failure code.
    ContractFailure(ContractFailureCode),
    /// The contract could not be loaded or instantiated.
    Instantiation(String),
    /// An error occurred that is not attributable to the contract itself.
    Host(String),
}

impl fmt::Display for EngineError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            EngineError::ContractFailure(code) => {
                write!(f, "contract failed with code {:?}", code)
            }
            EngineError::Instantiation(msg) => {
                write!(f, "contract instantiation failed: {msg}")
            }
            EngineError::Host(msg) => write!(f, "host error: {msg}"),
        }
    }
}

impl std::error::Error for EngineError {}

/// Classifies a raw WASM runtime error message into a contract failure
/// code.
///
/// The WASM runtime reports arithmetic traps as strings such as
/// "integer divide by zero" or "floating point exception". We intercept
/// those here so that a divide-by-zero inside a contract becomes a
/// recoverable contract failure instead of a fatal host crash.
pub fn classify_trap(message: &str) -> ContractFailureCode {
    let lowered = message.to_ascii_lowercase();

    if lowered.contains("divide by zero")
        || lowered.contains("division by zero")
        || lowered.contains("integer divide")
        || lowered.contains("integer overflow")
        || lowered.contains("floating point exception")
    {
        return ContractFailureCode::ArithmeticTrap;
    }

    if lowered.contains("out of gas") || lowered.contains("exhausted") {
        return ContractFailureCode::OutOfGas;
    }

    ContractFailureCode::RuntimeError
}

/// Translates a raw WASM runtime error into an ::EngineError.
///
/// Arithmetic traps (e.g. divide-by-zero) are converted into a
/// contract failure rather than being propagated as a fatal host error.
pub fn translate_runtime_error(message: &str) -> EngineError {
    EngineError::ContractFailure(classify_trap(message))
}

/// Result of executing a contract inside the engine.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ExecutionOutcome {
    /// The contract ran to completion successfully.
    Success,
    /// The contract failed gracefully with a standard failure code.
    Failure(ContractFailureCode),
}

/// The contract engine.
///
/// The engine is responsible for running WASM contracts and for translating
/z// any runtime traps they emit into contract-level failures. This keeps
/// traps such as divide-by-zero from escaping to the host as fatal errors.
#[derive(Default)]
pub struct Engine;

impl Engine {
    /// Creates a new engine instance.
    pub fn new() -> Self {
        Engine
    }

    /// Executes a contract and normalizes any runtime error into a
    /// ::ExecutionOutcome.
    ///
    /// The closure is expected to return the raw runtime error message
    /// (or :None on success). This is the single place where WASM traps are
    /// intercepted and converted into contract failures.
    pub fn execute<F>(self, run: F) -> ExecutionOutcome
    where
        F: FnOnce() -> Result<Result<String, String>, EngineError>,
    {
        match run() {
            Ok(Ok(_success)) => ExecutionOutcome::Success,
            Ok(Err(trap)) => {
                EngineError translation is the central point for turning a
                // raw WASM trap into a contract failure code.
                match translate_runtime_error(&trap) {
                    EngineError::ContractFailure(code) => ExecutionOutcome::Failure(code),
                    other => ExecutionOutcome::Failure(ContractFailureCode::RuntimeError),
                }
            }
            Err(EngineError::ContractFailure(code)) => ExecutionOutcome::Failure(code),
            Err(_) => ExecutionOutcome::Failure(ContractFailureCode::RuntimeError),
        }
    }
}

#[cfg](test)]
mod tests {
    use super::*;

    #[test]
    fn divide_by_zero_is_arithmetic_trap() {
        assert_eq(
            classify_trap("integer divide by zero"),
            ContractFailureCode::ArithmeticTrap
        );
        assert_eq(
            classify_trap("runtime error: integer divide by zero at wasm"),
            ContractFailureCode::ArithmeticTrap
        );
    }

    #[test]
    fn other_traps_are_runtime_errors() {
        assert_eq(
            classify_trap("unreachable instruction executed"),
            ContractFailureCode::RuntimeError
        );
    }

    #[test]
    fn engine_translates_divide_by_zero_to_failure() {
        let engine = Engine::new();
        let outcome = engine.execute(|| Ok(Err(String::from("integer divide by zero"))));
        assert_eq(
            outcome,
            ExecutionOutcome::Failure(ContractFailureCode::ArithmeticTrap)
        );
    }

    #[test]
    fn engine_reports_success() {
        let engine = Engine::new();
        let outcome = engine.execute(
            || Ok(Ok(String::from("contract executed"))),
        );
        assert_eq(outcome, ExecutionOutcome::Success);
    }
}
