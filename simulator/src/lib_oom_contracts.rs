// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Intentionally faulty contract: triggers an out-of-memory condition.
//!
//! This contract allocates progressively larger vectors until the Soroban
//! host budget for memory bytes is exhausted.  It is used exclusively by
//! the simulator safety test-suite and must never be deployed on-chain.

#![no_std]

use soroban_sdk::{contract, contractimpl, Env, Vec};

#[contract]
pub struct OomContract;

#[contractimpl]
impl OomContract {
    /// Allocates `iterations` nested vectors, each of increasing size, to
    /// exhaust the host memory budget as quickly as possible.
    pub fn run(env: Env, iterations: u32) {
        let mut outer: Vec<Vec<u32>> = Vec::new(&env);
        for i in 0..iterations {
            let mut inner: Vec<u32> = Vec::new(&env);
            for j in 0..i {
                inner.push_back(j);
            }
            outer.push_back(inner);
        }
        // Prevent the compiler from optimising the allocation away.
        let _ = outer.len();
    }
}