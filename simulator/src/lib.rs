// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(clippy::pedantic, clippy::nursery, dead_code)]

pub mod asset_tracker;
pub mod context;
pub mod gas_optimizer;
pub mod git_detector;
pub mod host;
pub mod hsm;
pub mod ipc;
pub mod memory;
pub mod metering;
pub mod runner;
pub mod snapshot;
pub mod source_map_cache;
pub mod source_mapper;
pub mod stack_trace;
pub mod state;
pub mod types;
pub mod wasm_types;

#[cfg(test)]
mod memory_limit_test;

#[cfg(test)]
mod tests;
