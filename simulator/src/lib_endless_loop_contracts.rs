// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Intentionally faulty contract: triggers CPU budget exhaustion via an
//! endless loop.
//!
//! The Soroban host meters every Wasm instruction.  A non-terminating loop
//! will consume all available CPU instructions and cause the host to trap
//! with a budget-exceeded error.  This contract is used exclusively by the
//! simulator safety test-suite.

#![no_std]

use soroban_sdk::{contract, contractimpl, Env};

#[contract]
pub struct EndlessLoopContract;

#[contractimpl]
impl EndlessLoopContract {
    /// Enters a loop that never terminates.  The host will trap once the CPU
    /// instruction budget is exhausted.
    pub fn run(_env: Env) {
        // A plain `loop {}` is sufficient; the host metering will fire before
        // the Wasm engine can spin indefinitely on real hardware.
        #[allow(clippy::empty_loop)]
        loop {}
    }
}