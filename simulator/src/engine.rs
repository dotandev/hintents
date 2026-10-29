// Copyright 2025 Hintents Contributors
// SPDX-License-Identifier: Apache-2.0

use std::fmt;

use wasmi::{Engine, Linker, Module, Store, Trap, TrapCode, Value};

/// Standard contract failure code returned when a WASM contract traps.
pub const CONTRACT_FAILURE_CODE: i32 = -1;

/// Standard contract failure code for arithmetic traps such as divide-by-zero.
pub const CONTRACT_ARITHMETIC_FAILURE_CODE: i32 = -2;

/// Outcome of invoking a contract entry point.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ContractOutcome {
    /// The contract returned normally with the given values.
    Success(Vec<i64>),
    /// The contract failed with a standard failure code and a human-readable reason.
    Failure { code: i32, reason: String },
}

#[derive(Debug)]
pub enum EngineError {
    Compile(String),
    Instantiate(String),
    MissingExport(String),
    Host(String),
}

impl fmt::Display for EngineError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            EngineError::Compile(m) => write!(f, "compile error: {m}"),
            EngineError::Instantiate(m) => write!(f, "instantiation error: {m}"),
            EngineError::MissingExport(m) => write!(f, "missing export: {m}"),
            EngineError::Host(m) => write!(f, "host error: {m}"),
        }
    }
}

impl std::error::Error for EngineError {}

/// Maps a WASM trap into a standard contract failure code.
///
/// Arithmetic traps (divide-by-zero, integer overflow, invalid conversion)
/// are mapped to `CONTRACT_ARITHMETIC_FAILURE_CODE`; all other traps map to
/// `CONTRACT_FAILURE_CODE`. Traps are never propagated as host panics.
pub fn trap_to_failure(trap: &Trap) -> ContractOutcome {
    let (code, reason) = match trap.trap_code() {
        Some(TrapCode::IntegerDivisionByZero) => (
            CONTRACT_ARITHMETIC_FAILURE_CODE,
            "integer divide by zero".to_string(),
        ),
        Some(TrapCode::IntegerOverflow) => (
            CONTRACT_ARITHMETIC_FAILURE_CODE,
            "integer overflow".to_string(),
        ),
        Some(TrapCode::BadConversionToInteger) => (
            CONTRACT_ARITHMETIC_FAILURE_CODE,
            "invalid float-to-integer conversion".to_string(),
        ),
        Some(other) => (CONTRACT_FAILURE_CODE, format!("trap: {other:?}")),
        None => (CONTRACT_FAILURE_CODE, format!("trap: {trap}")),
    };
    ContractOutcome::Failure { code, reason }
}

/// Thin wrapper around a wasmi engine that executes contract entry points
/// and converts runtime traps into contract failure outcomes.
pub struct ContractEngine {
    engine: Engine,
}

impl Default for ContractEngine {
    fn default() -> Self {
        Self::new()
    }
}

impl ContractEngine {
    pub fn new() -> Self {
        Self {
            engine: Engine::default(),
        }
    }

    /// Invokes `entry` in the WASM module with the given i64 arguments.
    ///
    /// Runtime traps (including divide-by-zero) are intercepted and returned as
    /// `ContractOutcome::Failure`, so the host process does not crash.
    pub fn invoke(
        &self,
        wasm: &[u8],
        entry: &str,
        args: &[i64],
    ) -> Result<ContractOutcome, EngineError> {
        let module = Module::new(&self.engine, wasm)
            .map_err(|e| EngineError::Compile(e.to_string()))?;
        let mut store = Store::new(&self.engine, ());
        let linker = <Linker<()>>::new(&self.engine);
        let instance = linker
            .instantiate(&mut store, &module)
            .and_then(|pre| pre.start(&mut store))
            .map_err(|e| EngineError::Instantiate(e.to_string()))?;

        let func = instance
            .get_func(&store, entry)
            .ok_or_else(|| EngineError::MissingExport(entry.to_string()))?;

        let ty = func.ty(&store);
        let params: Vec<Value> = args.iter().map(|v| Value::I64(*v)).collect();
        let mut results: Vec<Value> = ty
            .results()
            .iter()
            .map(|t| Value::default(*t))
            .collect();

        match func.call(&mut store, &params, &mut results) {
            Ok(()) => {
                let out = results
                    .iter()
                    .map(|v| match v {
                        Value::I64(x) => *x,
                        Value::I32(x) => *x as i64,
                        _ => 0,
                    })
                    .collect();
                Ok(ContractOutcome::Success(out))
            }
            Err(err) => match err.as_trap_code() {
                Some(code) => Ok(trap_to_failure(&Trap::from(code))),
                None => Err(EngineError::Host(err.to_string())),
            },
        }
    }
}

// Copyright 2025 Hintents Contributors
// SPDX-License-Identifier: Apache-2.0
