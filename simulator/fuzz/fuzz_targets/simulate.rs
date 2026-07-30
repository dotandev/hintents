// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Fuzz target for the simulator's untrusted-input paths.
//!
//! Everything reached from here is reachable from a transaction envelope or a
//! ledger snapshot that a caller hands the simulator, which is precisely the
//! surface an attacker controls.
//!
//! # Why this target used to hang
//!
//! `Host::invoke_function` runs Wasm the input chose, and a contract that loops
//! forever gives libFuzzer nothing to report: it simply waits. The default
//! `-timeout` is 1200 seconds and `-rss_limit_mb` only reacts once the memory is
//! already gone, so one bad input could stall a campaign for twenty minutes and
//! then die without a useful reproducer. A run that spins inside native decoding
//! code was worse still, because the Soroban budget is never charged for it.
//!
//! Four ceilings from `erst_sim::metering` now make that impossible:
//!
//! - A Soroban budget with an explicit CPU instruction cap, so a metered loop
//!   traps in milliseconds instead of running to the network's full allowance.
//! - A cooperative [`RunGuard`] deadline, checked between operations, so the
//!   harness abandons a slow run under its own control.
//! - A watchdog thread that aborts the process when a run outlives its
//!   wall-clock budget, which converts a hang into a crash with a saved input.
//! - A tracking global allocator that fails allocations past a heap ceiling,
//!   so an oversized length field cannot push the machine into swap.
//!
//! Together they mean every input either finishes or fails; none of them can
//! wait forever.

#![no_main]

use erst_sim::metering::{check_budget, install_fuzz_limits};
use erst_sim::metering::{MeteringLimits, RunGuard, TrackingAllocator};
use erst_sim::runner::SimHost;
use erst_sim::snapshot::{decode_ledger_entry, decode_ledger_key, LedgerSnapshot};
use libfuzzer_sys::fuzz_target;
use soroban_env_host::xdr::{FeeBumpTransactionInnerTx, Limits, Operation};
use soroban_env_host::xdr::{OperationBody, ReadXdr, TransactionEnvelope};
use std::alloc::System;

/// Every allocation in this process is accounted for, so a run that tries to
/// allocate its way past the CPU ceiling fails fast instead of swapping.
#[global_allocator]
static ALLOC: TrackingAllocator<System> =
    TrackingAllocator::new(System, MeteringLimits::FUZZ_HEAP_BYTES);

/// Recursion depth allowed when decoding fuzzer-controlled XDR.
///
/// `Limits::none()` would let a depth bomb exhaust the stack, which aborts the
/// process without telling us anything about the simulator.
const XDR_DEPTH_LIMIT: u32 = 64;

fuzz_target!(|data: &[u8]| {
    let limits = MeteringLimits::fuzzing();
    if data.len() > limits.max_input_bytes {
        return;
    }

    // Applies the CPU rlimit and starts the watchdog thread on the first run.
    let watchdog = install_fuzz_limits();
    let guard = watchdog.arm(limits.wall_clock);

    drive(data, &limits, &guard);
});

/// Picks an entry point from the first input byte and spends the rest on it.
fn drive(data: &[u8], limits: &MeteringLimits, guard: &RunGuard<'_>) {
    let Some((&selector, payload)) = data.split_first() else {
        return;
    };

    match selector % 3 {
        0 => invoke_envelope(payload, limits, guard),
        1 => round_trip_snapshot(payload, guard),
        _ => decode_base64_entry(payload, guard),
    }
}

/// Decodes a transaction envelope and invokes every host function in it.
///
/// This is the path that hangs without metering: the input chooses the Wasm and
/// the Wasm chooses how long to run.
fn invoke_envelope(payload: &[u8], limits: &MeteringLimits, guard: &RunGuard<'_>) {
    let Ok(envelope) = TransactionEnvelope::from_xdr(payload, decode_limits(payload)) else {
        return;
    };

    let host = SimHost::new(Some(limits.budget_limits()), None, Some(limits.mem_bytes));

    for operation in operations(&envelope) {
        if guard.check().is_err() {
            return;
        }

        let budget = host.inner.budget_cloned();
        if check_budget(&budget, limits).is_err() {
            // A ceiling is already gone; the next call would only trap.
            return;
        }

        if let OperationBody::InvokeHostFunction(invoke) = &operation.body {
            // Errors are the expected outcome for fuzzed input, including the
            // budget trap that a non-terminating contract earns itself.
            let _ = host.inner.invoke_function(invoke.host_function.clone());
        }
    }
}

/// Decodes a binary ledger snapshot and re-encodes what came back.
fn round_trip_snapshot(payload: &[u8], guard: &RunGuard<'_>) {
    if guard.check().is_err() {
        return;
    }

    if let Ok(snapshot) = LedgerSnapshot::from_bytes(payload) {
        let _ = snapshot.to_bytes();
    }
}

/// Feeds the payload to the base64 plus XDR decoders the IPC bridge uses.
fn decode_base64_entry(payload: &[u8], guard: &RunGuard<'_>) {
    if guard.check().is_err() {
        return;
    }

    let Ok(encoded) = std::str::from_utf8(payload) else {
        return;
    };

    let _ = decode_ledger_key(encoded);
    let _ = decode_ledger_entry(encoded);
}

/// Decode bounds for one payload: never deeper than the stack tolerates, and
/// never longer than the payload itself, so a forged length field cannot drive
/// an allocation the input did not pay for.
fn decode_limits(payload: &[u8]) -> Limits {
    Limits {
        depth: XDR_DEPTH_LIMIT,
        len: payload.len(),
    }
}

/// The operations carried by any envelope flavour.
fn operations(envelope: &TransactionEnvelope) -> &[Operation] {
    match envelope {
        TransactionEnvelope::Tx(v1) => v1.tx.operations.as_slice(),
        TransactionEnvelope::TxV0(v0) => v0.tx.operations.as_slice(),
        TransactionEnvelope::TxFeeBump(bump) => match &bump.tx.inner_tx {
            FeeBumpTransactionInnerTx::Tx(v1) => v1.tx.operations.as_slice(),
        },
    }
}
