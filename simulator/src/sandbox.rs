// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

use std::collections::HashSet;

/// Maximum allowed cross-contract call depth.
pub const DEFAULT_MAX_CALL_DEPTH: u32 = 8;

/// Error types for sandbox boundary violations.
#[derive(Debug, thiserror::Error)]
pub enum SandboxBoundaryError {
    #[error(
        "cross-contract call depth exceeded: max={max}, current={current}"
    )]
    MaxCallDepthExceeded { max: u32, current: u32 },

    #[error(
        "unauthorized cross-contract call: caller {caller} invoked {callee} without authorization"
    )]
    UnauthorizedCrossContractCall { caller: String, callee: String },

    #[error(
        "call stack underflow: pop without matching push"
    )]
    CallStackUnderflow,

    #[error(
        "storage boundary violation: contract {contract_id} accessed storage outside its boundary"
    )]
    StorageBoundaryViolation { contract_id: String },

    #[error(
        "contract {0} not found in call stack"
    )]
    ContractNotInCallStack(String),
}

/// Represents a single cross-contract call frame in the simulated call stack.
#[derive(Debug, Clone)]
pub struct CrossContractCall {
    /// The contract that initiated the call.
    pub caller_id: String,
    /// The contract being called.
    pub callee_id: String,
    /// The function name being invoked on the callee.
    pub function_name: String,
}

/// Tracks and validates cross-contract call boundaries during simulation.
///
/// Mirrors the security boundaries of the real Soroban environment:
/// - Maximum call depth enforcement
/// - Call stack tracking for contract context
/// - Authorization boundary detection
pub struct SandboxBoundaryChecker {
    contract_call_stack: Vec<CrossContractCall>,
    enabled: bool,
    max_call_depth: u32,
    authorized_pairs: HashSet<(String, String)>,
}

impl SandboxBoundaryChecker {
    /// Creates a new boundary checker with default settings (depth limit 8).
    pub fn new() -> Self {
        Self {
            contract_call_stack: Vec::new(),
            enabled: true,
            max_call_depth: DEFAULT_MAX_CALL_DEPTH,
            authorized_pairs: HashSet::new(),
        }
    }

    /// Creates a new boundary checker with a custom max call depth.
    pub fn with_max_depth(max_call_depth: u32) -> Self {
        Self {
            contract_call_stack: Vec::new(),
            enabled: true,
            max_call_depth,
            authorized_pairs: HashSet::new(),
        }
    }

    /// Register a (caller, callee) pair as authorized for cross-contract calls.
    pub fn authorize_call(&mut self, caller: impl Into<String>, callee: impl Into<String>) {
        self.authorized_pairs
            .insert((caller.into(), callee.into()));
    }

    /// Returns whether sandbox boundary checking is enabled.
    pub fn enabled(&self) -> bool {
        self.enabled
    }

    /// Enable or disable sandbox boundary checking.
    pub fn set_enabled(&mut self, enabled: bool) {
        self.enabled = enabled;
    }

    /// Returns the current call stack depth.
    pub fn current_depth(&self) -> u32 {
        self.contract_call_stack.len() as u32
    }

    /// Returns the maximum allowed call depth.
    pub fn max_call_depth(&self) -> u32 {
        self.max_call_depth
    }

    /// Returns the contract ID at the top of the call stack (currently executing).
    pub fn current_contract_id(&self) -> Option<&str> {
        self.contract_call_stack
            .last()
            .map(|call| call.callee_id.as_str())
    }

    /// Returns the caller contract ID (the one that made the current call).
    pub fn caller_contract_id(&self) -> Option<&str> {
        self.contract_call_stack
            .last()
            .map(|call| call.caller_id.as_str())
    }

    /// Returns a reference to the full call stack.
    pub fn call_stack(&self) -> &[CrossContractCall] {
        &self.contract_call_stack
    }

    /// Records entering a new contract context through a cross-contract call.
    ///
    /// Returns an error if:
    /// - The call stack depth exceeds `max_call_depth`
    /// - The caller/callee pair is not in the authorized set
    pub fn push_call(
        &mut self,
        caller_id: impl Into<String>,
        callee_id: impl Into<String>,
        function_name: impl Into<String>,
    ) -> Result<(), SandboxBoundaryError> {
        if !self.enabled {
            self.contract_call_stack.push(CrossContractCall {
                caller_id: caller_id.into(),
                callee_id: callee_id.into(),
                function_name: function_name.into(),
            });
            return Ok(());
        }

        let caller = caller_id.into();
        let callee = callee_id.into();
        let func = function_name.into();

        let next_depth = self.contract_call_stack.len() as u32 + 1;
        if next_depth > self.max_call_depth {
            return Err(SandboxBoundaryError::MaxCallDepthExceeded {
                max: self.max_call_depth,
                current: next_depth,
            });
        }

        if !self.authorized_pairs.contains(&(caller.clone(), callee.clone())) {
            return Err(SandboxBoundaryError::UnauthorizedCrossContractCall {
                caller,
                callee,
            });
        }

        self.contract_call_stack.push(CrossContractCall {
            caller_id: caller,
            callee_id: callee,
            function_name: func,
        });
        Ok(())
    }

    /// Records returning from the current contract context.
    ///
    /// Returns an error if the call stack is already empty (underflow).
    pub fn pop_call(&mut self) -> Result<CrossContractCall, SandboxBoundaryError> {
        self.contract_call_stack
            .pop()
            .ok_or(SandboxBoundaryError::CallStackUnderflow)
    }

    /// Check whether a contract ID matches the currently executing contract.
    ///
    /// This validates that storage access is within the current contract's boundary.
    /// Returns `Ok(())` if the check passes or boundary checking is disabled.
    pub fn check_storage_access(
        &self,
        contract_id: &str,
    ) -> Result<(), SandboxBoundaryError> {
        if !self.enabled {
            return Ok(());
        }
        if let Some(current) = self.current_contract_id() {
            if current != contract_id {
                return Err(SandboxBoundaryError::StorageBoundaryViolation {
                    contract_id: contract_id.to_string(),
                });
            }
        }
        Ok(())
    }
}

impl Default for SandboxBoundaryChecker {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_checker() -> SandboxBoundaryChecker {
        let mut checker = SandboxBoundaryChecker::new();
        checker.authorize_call("contract_a", "contract_b");
        checker.authorize_call("contract_b", "contract_c");
        checker.authorize_call("contract_a", "contract_c");
        checker
    }

    #[test]
    fn test_new_checker_is_enabled_with_default_depth() {
        let checker = SandboxBoundaryChecker::new();
        assert!(checker.enabled());
        assert_eq!(checker.max_call_depth(), DEFAULT_MAX_CALL_DEPTH);
        assert_eq!(checker.current_depth(), 0);
        assert!(checker.current_contract_id().is_none());
    }

    #[test]
    fn test_push_and_pop_call() {
        let mut checker = make_checker();
        checker
            .push_call("contract_a", "contract_b", "hello")
            .expect("authorized call should succeed");
        assert_eq!(checker.current_depth(), 1);
        assert_eq!(checker.current_contract_id(), Some("contract_b"));
        assert_eq!(checker.caller_contract_id(), Some("contract_a"));

        let frame = checker.pop_call().expect("pop should succeed");
        assert_eq!(frame.callee_id, "contract_b");
        assert_eq!(checker.current_depth(), 0);
    }

    #[test]
    fn test_unauthorized_call_is_rejected() {
        let mut checker = make_checker();
        let err = checker
            .push_call("contract_a", "contract_evil", "steal")
            .expect_err("unauthorized call should be rejected");
        assert!(
            matches!(err, SandboxBoundaryError::UnauthorizedCrossContractCall { .. }),
            "expected UnauthorizedCrossContractCall, got {err}"
        );
    }

    #[test]
    fn test_max_call_depth_exceeded() {
        let mut checker = SandboxBoundaryChecker::with_max_depth(2);
        checker.authorize_call("a", "b");
        checker.authorize_call("b", "c");
        checker.authorize_call("c", "d");

        checker.push_call("a", "b", "f1").expect("depth 1 ok");
        checker.push_call("b", "c", "f2").expect("depth 2 ok");
        let err = checker
            .push_call("c", "d", "f3")
            .expect_err("depth 3 should exceed limit");
        assert!(
            matches!(err, SandboxBoundaryError::MaxCallDepthExceeded { max: 2, current: 3 }),
            "expected MaxCallDepthExceeded, got {err}"
        );
    }

    #[test]
    fn test_pop_empty_stack_returns_underflow() {
        let mut checker = SandboxBoundaryChecker::new();
        let err = checker.pop_call().expect_err("pop from empty stack");
        assert!(
            matches!(err, SandboxBoundaryError::CallStackUnderflow),
            "expected CallStackUnderflow, got {err}"
        );
    }

    #[test]
    fn test_disabled_checker_skips_all_validation() {
        let mut checker = SandboxBoundaryChecker::new();
        checker.set_enabled(false);
        // Even without auth, disabled checker allows the call
        assert!(checker.push_call("any", "any", "fn").is_ok());
        assert_eq!(checker.current_depth(), 1);
        assert!(checker.pop_call().is_ok());
    }

    #[test]
    fn test_check_storage_access_passes_for_current_contract() {
        let mut checker = make_checker();
        checker
            .push_call("contract_a", "contract_b", "fn")
            .expect("push");
        assert!(checker.check_storage_access("contract_b").is_ok());
    }

    #[test]
    fn test_call_stack_snapshot() {
        let mut checker = make_checker();
        checker
            .push_call("a", "b", "fn1")
            .expect("push a->b");
        checker
            .push_call("b", "c", "fn2")
            .expect("push b->c");

        let stack = checker.call_stack();
        assert_eq!(stack.len(), 2);
        assert_eq!(stack[0].callee_id, "b");
        assert_eq!(stack[1].callee_id, "c");
    }

    #[test]
    fn test_initial_depth_and_contract() {
        let checker = SandboxBoundaryChecker::new();
        assert_eq!(checker.current_depth(), 0);
        assert!(checker.current_contract_id().is_none());
        assert!(checker.caller_contract_id().is_none());
    }
}
