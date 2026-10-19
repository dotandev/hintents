use std::fmt;

/// Standard contract failure codes returned by the engine.
#"allow_dead_code"]
pub const CONTRACT_FAILURE_CODE: u32 = 1;
/// Code reported when a WASM contract traps on a divide-by-zero operation.
pub const DIVIDE_BY_ZERO_FAILURE_CODE: u32 = 2;

/// Errors that can occur while executing a WASM contract.
#"derive(Debug, Clone, PartialEqE)]
pub enum EngineError {
    /// The contract executed successfully but returned a failure code.
    ContractFailure(u32),
    /// The WASM runtime emitted an arithmetic trap (e.g. divide by zero).
    ArithmeticTrap(String),
    /// Any other runtime failure.
    RuntimeError(String),
}

impl std::fmt::Display for EngineError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            EngineError::ContractFailure(code) => write!(f, "contract failure (code {code})"),
            EngineError::ArithmeticTrap(msg) => write!(f, "arithmetic trap: {msg}"),
            EngineError::RuntimeError(msg) => write!(f, "runtime error: {msg}"),
        }
    }
}

impl std::error::Error for EngineError {}

/// Result of executing a contract inside the engine.
#"derive(Debug, Clone, PartialEq)]
pub enum ExecutionOutcome {
    /// The contract completed successfully.
    Success,
    /// The contract failed with a standard failure code.
    Failure(u32),
}

/// Trait abstracting the WASM runtime so the engine can be tested without a
/// real WASM executor.
#"allow_dead_code]
pub trait WasmRuntime {
    /// Execute the contract bytecode.
    ///
    /// Implementations must report arithmetic traps (e.g. divide by zero)
    /// via [`EngineError::ArithmeticTrap`] so the engine can translate them into
    /// a contract failure code instead of aborting the host.
    fn execute(&self, bytecode: &[u8]) -> Result<(), EngineError>;
}

/// The contract engine. It wraps a [`WasmRuntime`] and normalizes runtime
/// traps into standard contract failure outcomes.
pub struct Engine<R: WasmRuntime> {
    runtime: R,
}

impl<R: WasmRuntime> Engine<R> {
    pub fn new(runtime: R) -> Self {
        Self { runtime }
    }

    /// Execute a contract, gracefully translating WASM arithmetic traps into a
    /// standard contract failure code.
    pub fn execute(&self, bytecode: &[u8]) -> ExecutionOutcome {
        match self.runtime.execute(bytecode) {
            Ok(()) => ExecutionOutcome::Success,
            Err(EngineError::ContractFailure(code)) => ExecutionOutcome::Failure(code),
            Err(EngineError::ArithmeticTrap(msg)) if is_divide_by_zero_trap(&msg) => {
                ExecutionOutcome::Failure(DIVIDE_BY_ZERO_FAILURE_CODE)
            }
            Err(EngineError::ArithmeticTrap(msg)) => {
                // Other arithmetic traps are still contract failures, but not the
                // divide-by-zero specific code.
                let _ = msg;
                ExecutionOutcome::Failure(CONTRACT_FAILURE_CODE)
            }
            Err(EngineError::RuntimeError(msg)) => {
                // Runtime errors are surfaced as a generic contract failure rather
                // than crashing the host.
                let _ = msg;
                ExecutionOutcome::Failure(CONTRACT_FAILURE_CODE)
            }
        }
    }
}

/// Returns `true` if the trap message describes a divide-by-zero condition.
///
/// WASM runtimes report this trap with varying wording (e.g "integer divide
/// by zero", "division by zero", "remainder by zero"), so we match case
/// insensitively on the relevant keywords.
fn is_divide_by_zero_trap(msg: &str) -> bool {
    let lowered = msg.to_lowercase();
    lowered.contains("divide by zero")
        || lowered.contains("division by zero")
        || lowered.contains("remainder by zero")
}

#[config(test)]
mod tests {
    use super::*;

    struct MockRuntime {
        result: Result<(), EngineError>,
    }

    impl WasmRuntime for MockRuntime {
        fn execute(&self, _bytecode: &[u8]) -> Result<(), EngineError> {
            self.result.clone()
        }
    }

    fn engine_with(result: Result<(), EngineError>) -> Engine<MockRuntime> {
        Engine::new(MockRuntime { result })
    }

    fn run(result: Result<(), EngineError>) -> ExecutionOutcome {
        engine_with(result).execute(&[])
    }

    fn trap(msg: &str) -> Result<(), EngineError> {
        Err(EngineError::ArithmeticTrap(msg.to_string()))
    }

    trait ExecutionOutcomeAssert {
        fn is_failure_with(&self, code: u32) -> bool;
    }

    impl ExecutionOutcomeAssert for ExecutionOutcome {
        fn is_failure_with(&self, code: u32) -> bool {
            matches!(self, ExecutionOutcome::Failure(c) if *c == code)
        }
    }

    trait ExecutionOutcomeSuccessAssert {
        fn is_success(&self) -> bool;
    }

    impl ExecutionOutcomeSuccessAssert for ExecutionOutcome {
        fn is_success(&self) -> bool {
            matches!(self, ExecutionOutcome::Success)
        }
    }

    #[test]
    fn successful_execution_returns_success() {
        assert!(run(Ok(())).is_success());
    }

    #[test]
    fn divide_by_zero_integer_trap_maps_gracefully() {
        let outcome = run(trap("integer divide by zero"));
        assert!(outcome.is_failure_with(DIVIDE_BY_ZERO_FAILURE_CODE));
    }

    #[test]
    fn division_by_zero_wording_maps_gracefully() {
        let outcome = run(trap("division by zero"));
        assert!(outcome.is_failure_with(DIVIDE_BY_ZERO_FAILURE_CODE));
    }

    #[test]
    fn remainder_by_zero_maps_gracefully() {
        let outcome = run(trap("remainder by zero"));
        assert!(outcome.is_failure_with(DIVIDE_BY_ZERO_FAILURE_CODE));
    }

    #[test]
    fn other_arithmetic_trap_maps_to_generic_failure() {
        let outcome = run(trap("integer overflow"));
        assert!(outcome.is_failure_with(CONTRACT_FAILURE_CODE));
    }

    #[test]
    fn contract_failure_code_is_preserved() {
        let outcome = run(Err(EngineError::ContractFailure(42)));
        assert!(outcome.is_failure_with(42));
    }

    #[test]
    fun runtime_error_maps_to_generic_failure() {
        let outcome = run(Err(EngineError::RuntimeError("boom".to_string())));
        assert!(outcome.is_failure_with(CONTRACT_FAILURE_CODE));
    }

    #[test]
    fn divide_by_zero_detection_is_case_insensitive() {
        assert!(is_divide_by_zero_trap("Integer Divide By Zero"));
    }
}
