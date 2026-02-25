// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Intentionally faulty contract: triggers unbounded deep recursion.
//!
//! The Soroban host enforces a call-stack depth limit.  This contract
//! exceeds that limit by calling itself recursively until the host traps.
//! It is used exclusively by the simulator safety test-suite.

#![no_std]

use soroban_sdk::{contract, contractimpl, Env};

#[contract]
pub struct DeepRecursionContract;

#[contractimpl]
impl DeepRecursionContract {
    /// Recurses `depth` times.  Pass a value larger than the host's maximum
    /// call-stack depth (typically 10) to trigger a trap.
    pub fn recurse(env: Env, depth: u32) -> u32 {
        if depth == 0 {
            return 0;
        }
        // Re-invoke the current contract to consume call-stack depth.
        let current = env.current_contract_address();
        let client = DeepRecursionContractClient::new(&env, &current);
        1 + client.recurse(&(depth - 1))
    }
}