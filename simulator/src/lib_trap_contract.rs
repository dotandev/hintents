// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Intentionally faulty contract: executes an explicit Wasm trap.
//!
//! The `core::arch::wasm32::unreachable()` intrinsic compiles to a Wasm
//! `unreachable` instruction, which causes the host to trap immediately with
//! a VM-trap error.  This contract is used exclusively by the simulator
//! safety test-suite to verify that the simulator surfaces Wasm traps as
//! structured `HostError` responses rather than panicking.

#![no_std]

use soroban_sdk::{contract, contractimpl, Env};

#[contract]
pub struct TrapContract;

#[contractimpl]
impl TrapContract {
    /// Executes a Wasm `unreachable` instruction unconditionally.
    pub fn run(_env: Env) {
        // SAFETY: intentionally triggers a Wasm trap for testing purposes.
        unsafe { core::arch::wasm32::unreachable() }
    }
}