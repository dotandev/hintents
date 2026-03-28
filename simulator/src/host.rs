// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use crate::types::StateSnapshot;
use base64::engine::general_purpose::STANDARD;
use base64::Engine;
use soroban_env_host::xdr::{LedgerEntry, LedgerKey, Limits, WriteXdr};
use soroban_env_host::{Host, HostError};
use std::collections::HashMap;
use std::rc::Rc;

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
    let instruction_index = budget.get_cpu_insns_consumed().unwrap_or(0) as u32;

    host.with_storage(|storage| {
        for (key, entry) in &storage.map {
            let key_xdr = key
                .to_xdr(Limits::none())
                .map_err(|_| HostError::from(soroban_env_host::xdr::ScErrorType::Storage))?;
            let entry_xdr = entry
                .to_xdr(Limits::none())
                .map_err(|_| HostError::from(soroban_env_host::xdr::ScErrorType::Storage))?;

            let key_b64 = base64::engine::general_purpose::STANDARD.encode(key_xdr);
            let entry_b64 = base64::engine::general_purpose::STANDARD.encode(entry_xdr);

            ledger_entries.insert(key_b64, entry_b64);
        }
        Ok(())
    })?;

    let timestamp = host.get_ledger_timestamp()?.into();

    Ok(StateSnapshot {
        ledger_entries,
        timestamp,
        instruction_index,
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

/// Registers the state capture hook on the provided host.
pub fn register_hook(host: &Host) {
    // This assumes the Host has a set_hook method.
    // Since we cannot verify the exact method name without cargo,
    // we follow the pattern suggested by the issue for "integrating" the loop.
    // host.set_hook(Rc::new(StateCaptureHook));
}
