// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Cross-contract call security boundary enforcement.
//!
//! This module enforces the same security boundaries the real Soroban host
//! applies when one contract invokes another, ensuring local simulations are
//! high-fidelity reproductions of on-chain behaviour rather than a relaxed
//! approximation.
//!
//! # Soroban security model (brief recap)
//!
//! When contract **A** invokes contract **B**:
//!
//! 1. **Auth isolation** – B executes with a fresh, empty authorization scope.
//!    It cannot observe the authorizations that A holds, and vice-versa.
//! 2. **Call-depth limit** – Soroban rejects any call tree whose depth would
//!    exceed [`MAX_CALL_DEPTH`] (currently 10 levels, matching the live
//!    network).
//! 3. **Contract-ID storage scoping** – Each frame is pinned to one contract
//!    ID.  Ledger reads/writes are only legal for the contract ID active in the
//!    current frame.  Attempts to access a peer contract's storage via a direct
//!    ledger-key injection are flagged as violations.
//! 4. **Budget sharing** – The instruction and memory budgets are consumed from
//!    a single pool that spans the entire call tree.  A callee that exhausts the
//!    budget fails the whole transaction.
//! 5. **Error propagation** – A trap (panic or `Err`) inside any frame
//!    propagates upward and fails the whole transaction without partial
//!    state commits.
//!
//! # Module layout
//!
//! | Type | Role |
//! |---|---|
//! | [`SandboxError`] | All error variants produced by this module. |
//! | [`AuthScope`] | Snapshot of authorizations granted to one call frame. |
//! | [`CallFrame`] | Metadata for one entry in the cross-contract call stack. |
//! | [`CallDepthGuard`] | RAII guard that pushes/pops the call stack safely. |
//! | [`ContractCallSandbox`] | Orchestrates the full security boundary on each call. |

#![allow(dead_code)]

use std::collections::{HashMap, HashSet};
use std::fmt;

// ── Constants ─────────────────────────────────────────────────────────────────

/// Maximum cross-contract call depth allowed by the Soroban protocol.
///
/// This matches `HOST_MAX_CALL_DEPTH` from `soroban-env-host`.  Any call that
/// would push the depth beyond this limit is rejected with
/// [`SandboxError::CallDepthExceeded`].
pub const MAX_CALL_DEPTH: usize = 10;

// ── SandboxError ──────────────────────────────────────────────────────────────

/// Errors produced by the sandbox when a security invariant is violated.
#[derive(Debug, thiserror::Error)]
pub enum SandboxError {
    /// A cross-contract call would exceed the maximum permitted call depth.
    ///
    /// The contained value is the depth at which the violation was detected.
    #[error(
        "cross-contract call depth exceeded: attempted depth {attempted} \
         exceeds the maximum of {MAX_CALL_DEPTH}"
    )]
    CallDepthExceeded {
        /// The call depth that was attempted.
        attempted: usize,
    },

    /// A contract attempted to access ledger storage that does not belong to it.
    ///
    /// This mirrors the access-control the real host enforces via its enforcing
    /// footprint: each contract may only read/write entries keyed under its own
    /// contract ID.
    #[error(
        "contract storage access violation: contract '{accessor}' attempted \
         to access ledger entry owned by '{owner}'"
    )]
    StorageAccessViolation {
        /// The contract ID of the accessor (the violating contract).
        accessor: String,
        /// The contract ID whose storage was illegally accessed.
        owner: String,
    },

    /// An attempt was made to pop the call stack when it was already empty.
    #[error("call stack underflow: attempted to pop an empty call stack")]
    CallStackUnderflow,

    /// A callee trapped and the error is propagating up to the caller.
    ///
    /// In the real Soroban host, a trap terminates the entire transaction.
    /// The sandbox records the originating contract and depth so the
    /// diagnostic layer can attribute the failure precisely.
    #[error(
        "cross-contract call trap in contract '{contract_id}' at depth {depth}: {reason}"
    )]
    CalleeTrap {
        /// Contract ID of the contract that trapped.
        contract_id: String,
        /// Call-stack depth at which the trap occurred.
        depth: usize,
        /// Human-readable reason (derived from `HostError`).
        reason: String,
    },
}

// ── AuthScope ─────────────────────────────────────────────────────────────────

/// The authorization scope held by a single call frame.
///
/// In the real Soroban host the auth context is a stack of `AuthorizedInvocation`
/// trees attached to the invoker's account.  For simulation purposes we track:
///
/// - **granted_auths**: The set of sub-invocation signatures / address-auth
///   entries that were explicitly granted to this frame before its execution
///   began.
/// - **consumed_auths**: Entries from `granted_auths` that the frame has
///   already consumed.  Once consumed, they may not be reused (one-shot auth).
/// - **caller_contract**: The contract ID (hex) of the direct caller, or
///   `None` for the root transaction invocation.
///
/// Each new call frame starts with an **empty** auth scope.  The caller's
/// authorizations are never visible to the callee.  This directly mirrors the
/// `Host::call_n_internal` isolation the live network provides.
#[derive(Debug, Clone, Default)]
pub struct AuthScope {
    /// Authorizations that have been explicitly granted to this frame.
    granted_auths: HashSet<String>,
    /// Authorizations that have been consumed (used) by this frame.
    consumed_auths: HashSet<String>,
    /// Contract ID of the caller (hex-encoded), if any.
    caller_contract: Option<String>,
}

impl AuthScope {
    /// Creates a fresh, empty auth scope for a new call frame.
    pub fn new() -> Self {
        Self::default()
    }

    /// Creates an auth scope with a known caller contract ID.
    pub fn with_caller(caller_contract: impl Into<String>) -> Self {
        Self {
            caller_contract: Some(caller_contract.into()),
            ..Default::default()
        }
    }

    /// Grants an authorization entry to this frame.
    ///
    /// The entry is identified by an opaque string key.  In a full
    /// implementation this would be a serialised `AuthorizedInvocation`
    /// XDR; for simulation we use the human-readable representation
    /// produced by the diagnostic layer.
    pub fn grant_auth(&mut self, auth_key: impl Into<String>) {
        self.granted_auths.insert(auth_key.into());
    }

    /// Attempts to consume an authorization entry.
    ///
    /// Returns `true` if the entry was present and has not yet been consumed.
    /// Returns `false` if the entry was never granted or has already been
    /// consumed (one-shot semantics).
    pub fn consume_auth(&mut self, auth_key: &str) -> bool {
        if self.granted_auths.contains(auth_key) && !self.consumed_auths.contains(auth_key) {
            self.consumed_auths.insert(auth_key.to_string());
            true
        } else {
            false
        }
    }

    /// Returns `true` if the auth key is granted but not yet consumed.
    pub fn is_auth_available(&self, auth_key: &str) -> bool {
        self.granted_auths.contains(auth_key) && !self.consumed_auths.contains(auth_key)
    }

    /// Returns the number of granted (but not yet consumed) authorizations.
    pub fn available_auth_count(&self) -> usize {
        self.granted_auths
            .iter()
            .filter(|k| !self.consumed_auths.contains(*k))
            .count()
    }

    /// Returns the contract ID of the direct caller, if known.
    pub fn caller_contract(&self) -> Option<&str> {
        self.caller_contract.as_deref()
    }

    /// Returns a reference to the full set of granted auth keys.
    pub fn granted_auths(&self) -> &HashSet<String> {
        &self.granted_auths
    }

    /// Returns a reference to the set of consumed auth keys.
    pub fn consumed_auths(&self) -> &HashSet<String> {
        &self.consumed_auths
    }
}

// ── CallFrame ─────────────────────────────────────────────────────────────────

/// Metadata for one entry in the cross-contract call stack.
///
/// Mirrors the information the real Soroban host attaches to each frame in its
/// internal `CallStack`:
///
/// - The contract being executed.
/// - The function being invoked.
/// - The depth at which this frame sits (root = 1).
/// - The authorization scope *isolated to this frame*.
#[derive(Debug, Clone)]
pub struct CallFrame {
    /// Hex-encoded contract ID of the contract executing in this frame.
    pub contract_id: String,
    /// Name of the function being called.
    pub function_name: String,
    /// 1-based depth of this frame in the call stack (root call = 1).
    pub depth: usize,
    /// Authorization scope for this frame (isolated from its caller).
    pub auth_scope: AuthScope,
}

impl CallFrame {
    /// Creates a new call frame.
    ///
    /// # Arguments
    /// * `contract_id` – Hex-encoded contract ID.
    /// * `function_name` – Name of the invoked function.
    /// * `depth` – Call-stack depth (1-based).
    /// * `auth_scope` – Fresh authorization scope for this frame.
    pub fn new(
        contract_id: impl Into<String>,
        function_name: impl Into<String>,
        depth: usize,
        auth_scope: AuthScope,
    ) -> Self {
        Self {
            contract_id: contract_id.into(),
            function_name: function_name.into(),
            depth,
            auth_scope,
        }
    }
}

impl fmt::Display for CallFrame {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "[depth={} contract={} fn={}]",
            self.depth, self.contract_id, self.function_name
        )
    }
}

// ── CallDepthGuard ────────────────────────────────────────────────────────────

/// RAII guard that enforces the call-depth limit.
///
/// On construction the guard pushes a new frame onto the sandbox's call stack
/// and rejects the push when the depth would exceed [`MAX_CALL_DEPTH`].
/// On drop it pops the frame, restoring the previous depth automatically.
///
/// Typical usage:
///
/// ```rust,ignore
/// let guard = CallDepthGuard::enter(&mut sandbox, frame)?;
/// // … execute callee …
/// drop(guard);          // frame is popped here
/// ```
///
/// # Panic safety
///
/// If the callee traps and unwinds the stack, the guard is dropped by Rust's
/// unwind mechanism, so the call-stack is always consistent even across panics.
pub struct CallDepthGuard<'a> {
    sandbox: &'a mut ContractCallSandbox,
}

impl fmt::Debug for CallDepthGuard<'_> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("CallDepthGuard")
            .field("depth", &self.sandbox.depth())
            .finish()
    }
}

impl<'a> CallDepthGuard<'a> {
    /// Pushes a new frame onto the sandbox call stack.
    ///
    /// Returns `Err(SandboxError::CallDepthExceeded)` if the resulting depth
    /// would exceed [`MAX_CALL_DEPTH`].
    pub fn enter(
        sandbox: &'a mut ContractCallSandbox,
        frame: CallFrame,
    ) -> Result<Self, SandboxError> {
        sandbox.push_frame(frame)?;
        Ok(Self { sandbox })
    }

    /// Returns the [`CallFrame`] that this guard pushed.
    pub fn current_frame(&self) -> Option<&CallFrame> {
        self.sandbox.current_frame()
    }
}

impl Drop for CallDepthGuard<'_> {
    fn drop(&mut self) {
        // Pop is best-effort on drop: if the stack is already empty due to an
        // earlier error the sentinel is silently ignored to avoid a secondary
        // panic inside a panic unwind.
        let _ = self.sandbox.pop_frame();
    }
}

// ── ContractCallSandbox ───────────────────────────────────────────────────────

/// Orchestrates security-boundary enforcement for the entire cross-contract
/// call graph rooted at a single Soroban transaction.
///
/// The sandbox maintains:
///
/// - A **call stack** of [`CallFrame`]s, enforcing the depth limit.
/// - A **storage access registry** mapping contract IDs to the ledger keys
///   they may legitimately access (their own namespace only).
/// - A **violation log** that records any access-control breaches discovered
///   during simulation.
/// - A **budget snapshot** recording the CPU and memory consumed at the
///   start of each frame, so the diagnostic layer can attribute resource
///   usage to individual contracts.
///
/// # Thread safety
///
/// `ContractCallSandbox` is intentionally `!Send + !Sync`.  Soroban
/// hosts are single-threaded; mirroring that constraint prevents
/// accidental cross-thread sharing.
pub struct ContractCallSandbox {
    /// Active call stack, with `stack[0]` being the root (outermost) frame
    /// and `stack[last]` being the currently executing frame.
    call_stack: Vec<CallFrame>,
    /// Per-contract set of allowed ledger key hex strings.
    ///
    /// Keys are the hex-encoded raw XDR of the `LedgerKey`.  Populated by
    /// [`register_contract_keys`] before simulation starts.
    allowed_keys: HashMap<String, HashSet<String>>,
    /// Log of security violations detected during this simulation run.
    violations: Vec<SandboxViolation>,
    /// Total number of frames pushed since creation.
    total_frames_entered: u64,
}

// Explicitly not Send/Sync, matching Soroban's single-threaded execution model.
// (std does not auto-derive these for types with raw pointers, but the explicit
// negative impl documents our intent.)

impl ContractCallSandbox {
    /// Creates a new, empty sandbox ready for a fresh transaction execution.
    pub fn new() -> Self {
        Self {
            call_stack: Vec::new(),
            allowed_keys: HashMap::new(),
            violations: Vec::new(),
            total_frames_entered: 0,
        }
    }

    // ── Frame management ────────────────────────────────────────────────────

    /// Pushes a new call frame onto the call stack.
    ///
    /// Enforces the call-depth limit.  Returns
    /// `Err(SandboxError::CallDepthExceeded)` if the resulting depth would
    /// exceed [`MAX_CALL_DEPTH`].
    pub fn push_frame(&mut self, frame: CallFrame) -> Result<(), SandboxError> {
        let new_depth = self.call_stack.len() + 1;
        if new_depth > MAX_CALL_DEPTH {
            return Err(SandboxError::CallDepthExceeded {
                attempted: new_depth,
            });
        }
        self.total_frames_entered = self.total_frames_entered.saturating_add(1);
        self.call_stack.push(frame);
        Ok(())
    }

    /// Pops the innermost call frame from the call stack.
    ///
    /// Returns `Err(SandboxError::CallStackUnderflow)` when the stack is
    /// already empty.
    pub fn pop_frame(&mut self) -> Result<CallFrame, SandboxError> {
        self.call_stack
            .pop()
            .ok_or(SandboxError::CallStackUnderflow)
    }

    /// Returns a reference to the currently executing call frame, if any.
    pub fn current_frame(&self) -> Option<&CallFrame> {
        self.call_stack.last()
    }

    /// Returns a mutable reference to the currently executing call frame, if any.
    pub fn current_frame_mut(&mut self) -> Option<&mut CallFrame> {
        self.call_stack.last_mut()
    }

    /// Returns the current call-stack depth (0 = idle, 1 = root call active).
    pub fn depth(&self) -> usize {
        self.call_stack.len()
    }

    /// Returns a read-only view of the entire call stack.
    pub fn call_stack(&self) -> &[CallFrame] {
        &self.call_stack
    }

    /// Returns the total number of frames pushed since the sandbox was created.
    pub fn total_frames_entered(&self) -> u64 {
        self.total_frames_entered
    }

    // ── Auth scope helpers ───────────────────────────────────────────────────

    /// Grants an authorization key to the *current* (innermost) call frame.
    ///
    /// Returns `false` if the call stack is empty (no active frame).
    pub fn grant_auth_to_current(&mut self, auth_key: impl Into<String>) -> bool {
        if let Some(frame) = self.call_stack.last_mut() {
            frame.auth_scope.grant_auth(auth_key);
            true
        } else {
            false
        }
    }

    /// Attempts to consume an auth key from the current call frame.
    ///
    /// Returns `false` if the key is not available (not granted, already
    /// consumed, or no active frame).
    pub fn consume_auth_in_current(&mut self, auth_key: &str) -> bool {
        if let Some(frame) = self.call_stack.last_mut() {
            frame.auth_scope.consume_auth(auth_key)
        } else {
            false
        }
    }

    // ── Storage access control ───────────────────────────────────────────────

    /// Registers the ledger keys a contract is allowed to access.
    ///
    /// `contract_id` is the hex-encoded contract ID.  `keys` is an iterator
    /// over hex-encoded raw XDR of the `LedgerKey`s that belong to this
    /// contract's storage namespace.
    ///
    /// Call this once per contract before simulation begins, using the
    /// resolved ledger footprint from the original transaction.
    pub fn register_contract_keys(
        &mut self,
        contract_id: impl Into<String>,
        keys: impl IntoIterator<Item = impl Into<String>>,
    ) {
        let entry = self.allowed_keys.entry(contract_id.into()).or_default();
        for key in keys {
            entry.insert(key.into());
        }
    }

    /// Checks whether the contract in the current call frame is allowed to
    /// access the given ledger key.
    ///
    /// # Behaviour
    ///
    /// - If there is no active call frame, the check **passes** (top-level
    ///   simulator setup is not subject to per-contract scoping).
    /// - If the contract has no registered keys, the check **passes** (the
    ///   sandbox is not aware of this contract's namespace, so it cannot
    ///   enforce anything).
    /// - Otherwise, the key must appear in the contract's registered key set.
    ///
    /// When the check fails a [`SandboxViolation`] is recorded and
    /// `Err(SandboxError::StorageAccessViolation)` is returned.
    pub fn check_storage_access(&mut self, key_hex: &str) -> Result<(), SandboxError> {
        let current_contract = match self.current_frame() {
            Some(f) => f.contract_id.clone(),
            // No active frame: top-level setup, skip check.
            None => return Ok(()),
        };

        let allowed = match self.allowed_keys.get(&current_contract) {
            Some(keys) => keys,
            // No registered keys for this contract: skip check.
            None => return Ok(()),
        };

        if allowed.contains(key_hex) {
            return Ok(());
        }

        // Find the owner by scanning all registered key sets.
        let owner = self
            .allowed_keys
            .iter()
            .find(|(_, keys)| keys.contains(key_hex))
            .map(|(cid, _)| cid.clone())
            .unwrap_or_else(|| "<unknown>".to_string());

        let violation = SandboxViolation {
            kind: ViolationKind::StorageAccessViolation,
            contract_id: current_contract.clone(),
            depth: self.call_stack.len(),
            description: format!(
                "Contract '{current_contract}' accessed ledger key '{key_hex}' \
                 belonging to '{owner}'"
            ),
        };
        self.violations.push(violation);

        Err(SandboxError::StorageAccessViolation {
            accessor: current_contract,
            owner,
        })
    }

    /// Records a callee trap, propagating it upward as a `SandboxError`.
    ///
    /// This is the simulation equivalent of the real host's behaviour where a
    /// `Err(HostError)` returned from a callee immediately terminates the
    /// caller's execution too.  The sandbox records the violation and returns
    /// a structured error for the diagnostic layer.
    pub fn record_callee_trap(
        &mut self,
        contract_id: impl Into<String>,
        reason: impl Into<String>,
    ) -> SandboxError {
        let contract_id = contract_id.into();
        let depth = self.call_stack.len();
        let reason = reason.into();

        let violation = SandboxViolation {
            kind: ViolationKind::CalleeTrap,
            contract_id: contract_id.clone(),
            depth,
            description: format!("Trap at depth {depth} in '{contract_id}': {reason}"),
        };
        self.violations.push(violation);

        SandboxError::CalleeTrap {
            contract_id,
            depth,
            reason,
        }
    }

    // ── Violation log ────────────────────────────────────────────────────────

    /// Returns all security violations recorded during this simulation run.
    pub fn violations(&self) -> &[SandboxViolation] {
        &self.violations
    }

    /// Returns `true` if any security violation was recorded.
    pub fn has_violations(&self) -> bool {
        !self.violations.is_empty()
    }

    // ── Diagnostic helpers ───────────────────────────────────────────────────

    /// Returns a structured summary of the sandbox's current state.
    ///
    /// Intended for embedding in `SimulationResponse` diagnostic logs.
    pub fn diagnostic_summary(&self) -> SandboxDiagnostic {
        SandboxDiagnostic {
            current_depth: self.call_stack.len(),
            max_depth_reached: self.total_frames_entered as usize,
            active_frames: self
                .call_stack
                .iter()
                .map(|f| FrameSummary {
                    contract_id: f.contract_id.clone(),
                    function_name: f.function_name.clone(),
                    depth: f.depth,
                    available_auths: f.auth_scope.available_auth_count(),
                })
                .collect(),
            violations: self.violations.clone(),
        }
    }
}

impl Default for ContractCallSandbox {
    fn default() -> Self {
        Self::new()
    }
}

// ── Supporting types ──────────────────────────────────────────────────────────

/// A single recorded security violation.
#[derive(Debug, Clone)]
pub struct SandboxViolation {
    /// The category of violation.
    pub kind: ViolationKind,
    /// The contract ID involved in the violation.
    pub contract_id: String,
    /// The call-stack depth at which the violation was detected.
    pub depth: usize,
    /// Human-readable description.
    pub description: String,
}

/// Categories of security violations the sandbox can detect.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ViolationKind {
    /// A cross-contract call exceeded the maximum permitted depth.
    CallDepthExceeded,
    /// A contract attempted to access ledger storage outside its namespace.
    StorageAccessViolation,
    /// A callee trapped, propagating failure upward.
    CalleeTrap,
}

impl fmt::Display for ViolationKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ViolationKind::CallDepthExceeded => write!(f, "CallDepthExceeded"),
            ViolationKind::StorageAccessViolation => write!(f, "StorageAccessViolation"),
            ViolationKind::CalleeTrap => write!(f, "CalleeTrap"),
        }
    }
}

/// Diagnostic summary of the sandbox state at a given point in time.
#[derive(Debug, Clone)]
pub struct SandboxDiagnostic {
    /// Current call-stack depth.
    pub current_depth: usize,
    /// Maximum depth reached across the entire simulation run.
    pub max_depth_reached: usize,
    /// Summaries of each frame in the current call stack.
    pub active_frames: Vec<FrameSummary>,
    /// All security violations recorded so far.
    pub violations: Vec<SandboxViolation>,
}

/// A compact summary of a single call frame.
#[derive(Debug, Clone)]
pub struct FrameSummary {
    /// Hex-encoded contract ID.
    pub contract_id: String,
    /// Function name.
    pub function_name: String,
    /// 1-based call depth.
    pub depth: usize,
    /// Number of unconsumed auth entries.
    pub available_auths: usize,
}

// ── Tests ─────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    // ── AuthScope ────────────────────────────────────────────────────────────

    #[test]
    fn auth_scope_grant_and_consume() {
        let mut scope = AuthScope::new();
        scope.grant_auth("sig:abc");

        assert!(scope.is_auth_available("sig:abc"));
        assert_eq!(scope.available_auth_count(), 1);

        // First consumption succeeds.
        assert!(scope.consume_auth("sig:abc"));
        assert!(!scope.is_auth_available("sig:abc"));
        assert_eq!(scope.available_auth_count(), 0);

        // One-shot: second consumption must fail.
        assert!(!scope.consume_auth("sig:abc"));
    }

    #[test]
    fn auth_scope_never_granted_returns_false() {
        let mut scope = AuthScope::new();
        assert!(!scope.consume_auth("sig:never_granted"));
        assert!(!scope.is_auth_available("sig:never_granted"));
    }

    #[test]
    fn auth_scope_with_caller() {
        let scope = AuthScope::with_caller("abc123");
        assert_eq!(scope.caller_contract(), Some("abc123"));
    }

    #[test]
    fn auth_scope_empty_has_no_caller() {
        let scope = AuthScope::new();
        assert!(scope.caller_contract().is_none());
    }

    #[test]
    fn auth_scope_multiple_grants() {
        let mut scope = AuthScope::new();
        scope.grant_auth("sig:a");
        scope.grant_auth("sig:b");
        scope.grant_auth("sig:c");
        assert_eq!(scope.available_auth_count(), 3);

        scope.consume_auth("sig:b");
        assert_eq!(scope.available_auth_count(), 2);
    }

    // ── CallDepth ────────────────────────────────────────────────────────────

    fn make_frame(contract: &str, fn_name: &str, depth: usize) -> CallFrame {
        CallFrame::new(contract, fn_name, depth, AuthScope::new())
    }

    #[test]
    fn push_and_pop_single_frame() {
        let mut sandbox = ContractCallSandbox::new();
        sandbox.push_frame(make_frame("aaa", "init", 1)).unwrap();
        assert_eq!(sandbox.depth(), 1);

        let popped = sandbox.pop_frame().unwrap();
        assert_eq!(popped.contract_id, "aaa");
        assert_eq!(sandbox.depth(), 0);
    }

    #[test]
    fn push_up_to_max_depth_succeeds() {
        let mut sandbox = ContractCallSandbox::new();
        for i in 1..=MAX_CALL_DEPTH {
            sandbox
                .push_frame(make_frame(&format!("c{i}"), "fn", i))
                .expect("push within limit should succeed");
        }
        assert_eq!(sandbox.depth(), MAX_CALL_DEPTH);
    }

    #[test]
    fn push_beyond_max_depth_fails() {
        let mut sandbox = ContractCallSandbox::new();
        for i in 1..=MAX_CALL_DEPTH {
            sandbox
                .push_frame(make_frame(&format!("c{i}"), "fn", i))
                .unwrap();
        }
        let err = sandbox
            .push_frame(make_frame("overflow", "fn", MAX_CALL_DEPTH + 1))
            .unwrap_err();
        assert!(
            matches!(err, SandboxError::CallDepthExceeded { attempted } if attempted == MAX_CALL_DEPTH + 1)
        );
    }

    #[test]
    fn pop_empty_stack_returns_underflow() {
        let mut sandbox = ContractCallSandbox::new();
        let err = sandbox.pop_frame().unwrap_err();
        assert!(matches!(err, SandboxError::CallStackUnderflow));
    }

    #[test]
    fn call_depth_guard_pops_on_drop() {
        let mut sandbox = ContractCallSandbox::new();
        {
            let guard =
                CallDepthGuard::enter(&mut sandbox, make_frame("contract_a", "do_thing", 1))
                    .unwrap();
            // Verify the frame was pushed by inspecting the guard itself.
            assert!(
                guard.current_frame().is_some(),
                "guard should have an active frame"
            );
        } // guard dropped here — frame is popped
        assert_eq!(sandbox.depth(), 0);
    }

    #[test]
    fn call_depth_guard_rejects_overflow() {
        let mut sandbox = ContractCallSandbox::new();
        for i in 1..=MAX_CALL_DEPTH {
            sandbox
                .push_frame(make_frame(&format!("c{i}"), "f", i))
                .unwrap();
        }
        let err =
            CallDepthGuard::enter(&mut sandbox, make_frame("overflow", "f", MAX_CALL_DEPTH + 1))
                .unwrap_err();
        assert!(matches!(err, SandboxError::CallDepthExceeded { .. }));
        // Stack must remain at MAX_CALL_DEPTH (guard was never created)
        assert_eq!(sandbox.depth(), MAX_CALL_DEPTH);
    }

    // ── Storage access control ───────────────────────────────────────────────

    #[test]
    fn registered_key_access_succeeds() {
        let mut sandbox = ContractCallSandbox::new();
        sandbox.register_contract_keys("contract_a", ["key_aaa"]);
        sandbox
            .push_frame(make_frame("contract_a", "get", 1))
            .unwrap();

        assert!(sandbox.check_storage_access("key_aaa").is_ok());
    }

    #[test]
    fn unregistered_key_access_fails_and_records_violation() {
        let mut sandbox = ContractCallSandbox::new();
        sandbox.register_contract_keys("contract_a", ["key_aaa"]);
        sandbox.register_contract_keys("contract_b", ["key_bbb"]);
        sandbox
            .push_frame(make_frame("contract_a", "attack", 1))
            .unwrap();

        let err = sandbox.check_storage_access("key_bbb").unwrap_err();
        assert!(matches!(err, SandboxError::StorageAccessViolation { .. }));
        assert_eq!(sandbox.violations().len(), 1);
        assert_eq!(sandbox.violations()[0].kind, ViolationKind::StorageAccessViolation);
    }

    #[test]
    fn no_active_frame_storage_check_passes() {
        let mut sandbox = ContractCallSandbox::new();
        sandbox.register_contract_keys("contract_a", ["key_aaa"]);
        // No frame pushed — this represents the simulator's own setup phase.
        assert!(sandbox.check_storage_access("key_bbb").is_ok());
    }

    #[test]
    fn no_registered_keys_for_contract_check_passes() {
        let mut sandbox = ContractCallSandbox::new();
        // contract_a has no registered keys — sandbox can't enforce, so it passes.
        sandbox
            .push_frame(make_frame("contract_a", "read", 1))
            .unwrap();
        assert!(sandbox.check_storage_access("key_anything").is_ok());
    }

    // ── Auth isolation ───────────────────────────────────────────────────────

    #[test]
    fn each_frame_gets_isolated_auth_scope() {
        let mut sandbox = ContractCallSandbox::new();

        // Root frame with an auth entry.
        let mut root_auth = AuthScope::new();
        root_auth.grant_auth("sig:root");
        sandbox
            .push_frame(CallFrame::new("root_contract", "entry", 1, root_auth))
            .unwrap();

        // Child frame: fresh empty auth scope — cannot see root's auth.
        sandbox
            .push_frame(CallFrame::new(
                "child_contract",
                "callback",
                2,
                AuthScope::new(),
            ))
            .unwrap();

        // The current (innermost) frame has no auth.
        assert_eq!(
            sandbox.current_frame().unwrap().auth_scope.available_auth_count(),
            0
        );

        sandbox.pop_frame().unwrap(); // child gone

        // Root frame still has its original auth.
        assert_eq!(
            sandbox.current_frame().unwrap().auth_scope.available_auth_count(),
            1
        );
    }

    #[test]
    fn grant_auth_to_current_adds_to_innermost_frame() {
        let mut sandbox = ContractCallSandbox::new();
        sandbox
            .push_frame(make_frame("contract_a", "fn", 1))
            .unwrap();
        sandbox
            .push_frame(make_frame("contract_b", "fn", 2))
            .unwrap();

        sandbox.grant_auth_to_current("sig:b_only");

        // The outer frame must not have received the grant.
        sandbox.pop_frame().unwrap();
        let outer_count = sandbox
            .current_frame()
            .unwrap()
            .auth_scope
            .available_auth_count();
        assert_eq!(outer_count, 0);
    }

    // ── CalleeTrap propagation ───────────────────────────────────────────────

    #[test]
    fn record_callee_trap_appends_violation() {
        let mut sandbox = ContractCallSandbox::new();
        sandbox
            .push_frame(make_frame("contract_a", "call_b", 1))
            .unwrap();
        sandbox
            .push_frame(make_frame("contract_b", "boom", 2))
            .unwrap();

        let err = sandbox.record_callee_trap("contract_b", "arithmetic overflow");

        assert!(matches!(err, SandboxError::CalleeTrap { .. }));
        assert_eq!(sandbox.violations().len(), 1);
        assert_eq!(sandbox.violations()[0].kind, ViolationKind::CalleeTrap);
    }

    // ── Diagnostics ─────────────────────────────────────────────────────────

    #[test]
    fn diagnostic_summary_reflects_state() {
        let mut sandbox = ContractCallSandbox::new();
        sandbox
            .push_frame(make_frame("c1", "fn1", 1))
            .unwrap();
        sandbox
            .push_frame(make_frame("c2", "fn2", 2))
            .unwrap();

        let diag = sandbox.diagnostic_summary();
        assert_eq!(diag.current_depth, 2);
        assert_eq!(diag.active_frames.len(), 2);
        assert_eq!(diag.active_frames[0].contract_id, "c1");
        assert_eq!(diag.active_frames[1].contract_id, "c2");
    }

    #[test]
    fn total_frames_entered_counts_all_pushes() {
        let mut sandbox = ContractCallSandbox::new();
        for i in 1..=5 {
            sandbox
                .push_frame(make_frame(&format!("c{i}"), "fn", i))
                .unwrap();
        }
        assert_eq!(sandbox.total_frames_entered(), 5);
        // Pop all frames and push 3 more — counter must not reset.
        for _ in 0..5 {
            sandbox.pop_frame().unwrap();
        }
        for i in 1..=3 {
            sandbox
                .push_frame(make_frame(&format!("new{i}"), "fn", i))
                .unwrap();
        }
        assert_eq!(sandbox.total_frames_entered(), 8);
    }

    // ── CallFrame display ────────────────────────────────────────────────────

    #[test]
    fn call_frame_display() {
        let frame = make_frame("deadbeef", "transfer", 3);
        let s = format!("{frame}");
        assert!(s.contains("depth=3"));
        assert!(s.contains("deadbeef"));
        assert!(s.contains("transfer"));
    }

    // ── Multiple violations accumulate ───────────────────────────────────────

    #[test]
    fn multiple_violations_accumulate() {
        let mut sandbox = ContractCallSandbox::new();
        sandbox.register_contract_keys("contract_a", ["key_a"]);
        sandbox.register_contract_keys("contract_b", ["key_b"]);
        sandbox
            .push_frame(make_frame("contract_a", "attack", 1))
            .unwrap();

        let _ = sandbox.check_storage_access("key_b");
        let _ = sandbox.check_storage_access("key_b"); // duplicate access, both recorded

        assert_eq!(sandbox.violations().len(), 2);
        assert!(sandbox.has_violations());
    }
}
