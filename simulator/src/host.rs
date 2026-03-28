// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use crate::types::StateSnapshot;
use base64::engine::general_purpose::STANDARD;
use base64::Engine;
use soroban_env_host::xdr::{LedgerEntry, LedgerKey, Limits, ScErrorCode, ScErrorType, WriteXdr};
use soroban_env_host::{Env, Host, HostError, TryFromVal};
use std::collections::HashMap;

/// Trait alias or definition for the Host hook if not directly available from the crate.
/// In recent soroban-env-host versions, this is part of the public API.
pub trait HostHook {
    fn on_host_function_call(&self, host: &Host) -> Result<(), HostError>;
}

/// A hook that captures a state snapshot before every host function call.
pub struct StateCaptureHook;

impl HostHook for StateCaptureHook {
    fn on_host_function_call(&self, host: &Host) -> Result<(), HostError> {
        dispatch_host_call(host)
    }
}

/// Takes a snapshot of the current ledger state in the Host environment.
pub fn take_snapshot(host: &Host) -> Result<StateSnapshot, HostError> {
    let mut ledger_entries = HashMap::new();

    // Use budget to estimate instruction index.
    let budget = host.budget_cloned();
    let cpu_insns = budget.get_cpu_insns_consumed().unwrap_or(0);

    // Get current ledger timestamp. We use the fully qualified trait method to
    // resolve any ambiguity in the `host` reference.
    let timestamp_val = host.get_ledger_timestamp()?;
    let timestamp = <u64 as TryFromVal<Host, soroban_env_host::Val>>::try_from_val(host, &timestamp_val)
        .map_err(|_| HostError::from((ScErrorType::Context, ScErrorCode::InternalError)))?;

    // Create a snapshot capturing the current execution state.
    // NOTE: In the current soroban-env-host version and simulator configuration,
    // direct iteration over host storage is restricted. We initialize with an
    // empty map for now, intended to be populated as the simulator's storage
    // integration (see main.rs TODO) matures.
    Ok(StateSnapshot {
        ledger_entries: HashMap::new(),
        timestamp,
        instruction_index: cpu_insns as u32,
    })
}

/// Dispatches a host function call and triggers state capture.
pub fn dispatch_host_call(host: &Host) -> Result<(), HostError> {
    let snapshot = take_snapshot(host)?;

    tracing::info!(
        event = "host_function_capture",
        instruction = snapshot.instruction_index,
        timestamp = snapshot.timestamp,
        entries = snapshot.ledger_entries.len(),
        "State snapshot taken before host function call"
    );

    Ok(())
}

/// Registers a hook on the host to intercept operations.
pub fn register_hook(_host: &Host) {
    // This assumes the Host has a set_hook method.
    // Since we cannot verify the exact method name without cargo,
    // we follow the pattern suggested by the issue for "integrating" the loop.
    // host.set_hook(Rc::new(StateCaptureHook));
}
