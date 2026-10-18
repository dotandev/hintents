// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(clippy::unreadable_literal)]

//! Integration tests for mock ledger clock drift functionality.
//!
//! These tests verify that the simulator's clock drift API works correctly
//! for testing time-bound contracts. They cover:
//! - Basic clock initialization and advancement
//! - Clock state preservation across snapshot/restore cycles
//! - Multiple clock advance operations
//! - Integration with `SimHost` lifecycle

use erst_sim::runner::{HostConfig, SimHost};
use erst_sim::types::SimulationRequest;

#[test]
fn test_clock_initialization_with_timestamp() {
    let mut host = SimHost::new(HostConfig::default());

    // Initialize the clock to a specific timestamp
    let base_timestamp = 1700000000u64; // Nov 15, 2023
    host.init_clock(base_timestamp);

    assert_eq!(host.get_clock(), base_timestamp);
}

#[test]
fn test_clock_single_advancement() {
    let mut host = SimHost::new(HostConfig::default());
    host.init_clock(1700000000);

    // Advance by 1 hour
    host.advance_clock(3600);

    assert_eq!(host.get_clock(), 1700003600);
}

#[test]
fn test_clock_multiple_advancements() {
    let mut host = SimHost::new(HostConfig::default());
    host.init_clock(1700000000);

    // Simulate a series of transactions over time
    host.advance_clock(3600); // +1 hour
    assert_eq!(host.get_clock(), 1700003600);

    host.advance_clock(7200); // +2 hours
    assert_eq!(host.get_clock(), 1700010800);

    host.advance_clock(86400); // +1 day
    assert_eq!(host.get_clock(), 1700097200);
}

#[test]
fn test_clock_advancement_with_negative_seconds() {
    let mut host = SimHost::new(HostConfig::default());
    host.init_clock(1700000000);

    host.advance_clock(5000);
    assert_eq!(host.get_clock(), 1700005000);

    // Go back in time
    host.advance_clock(-2500);
    assert_eq!(host.get_clock(), 1700002500);
}

#[test]
fn test_clock_set_absolute_timestamp() {
    let mut host = SimHost::new(HostConfig::default());
    host.init_clock(1700000000);

    // Advance clock
    host.advance_clock(10000);
    assert_eq!(host.get_clock(), 1700010000);

    // Jump to a different absolute time
    host.set_clock(1800000000);
    assert_eq!(host.get_clock(), 1800000000);

    // After set_clock, the next advance_clock clears the override
    // and advances from the system time. This is expected behavior
    // to use set_clock for absolute positioning and then advance from there,
    // you need to init_clock first with the new base.

    // To advance from the set absolute time, reinitialize:
    host.init_clock(1800000000);
    host.advance_clock(100);
    assert_eq!(host.get_clock(), 1800000100);
}

#[test]
fn test_clock_survives_snapshot_and_restore() {
    let mut host = SimHost::new(HostConfig::default());
    host.init_clock(1700000000);

    // Capture initial state
    let snapshot1 = host.capture_snapshot().expect("should capture snapshot");

    // Advance clock
    host.advance_clock(3600);
    assert_eq!(host.get_clock(), 1700003600);

    // Capture state after advancement
    let snapshot2 = host.capture_snapshot().expect("should capture snapshot");

    // Restore to initial snapshot - ledger state resets but clock continues
    host.restore_from_snapshot(&snapshot1)
        .expect("should restore");
    assert_eq!(
        host.get_clock(),
        1700003600,
        "clock should be preserved across restore"
    );

    // Restore to second snapshot - clock should still be at advanced time
    host.restore_from_snapshot(&snapshot2)
        .expect("should restore");
    assert_eq!(host.get_clock(), 1700003600);
}

#[test]
fn test_clock_reset_to_system_time() {
    let mut host = SimHost::new(HostConfig::default());
    host.init_clock(1700000000);
    host.advance_clock(50000);

    let before_reset = host.get_clock();
    assert_eq!(before_reset, 1700050000);

    // Reset clock to default (system time)
    host.reset_clock();

    let after_reset = host.get_clock();

    // After reset, clock should be approximately current system time
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs();

    // Should be within 2 seconds (allowing for test execution time)
    assert!((after_reset.cast_signed() - now.cast_signed()).abs() <= 2);
}

#[test]
fn test_simulation_request_with_ledger_timestamp() {
    // Verify that SimulationRequest can accept ledger_timestamp field
    let request = SimulationRequest {
        envelope_xdr: String::new(),
        result_meta_xdr: String::new(),
        ledger_entries: None,
        control_command: None,
        rewind_step: None,
        fork_params: None,
        harness_reset: false,
        ledger_entries_zstd: None,
        contract_wasm: None,
        wasm_path: None,
        no_cache: false,
        enable_optimization_advisor: false,
        profile: None,
        _timestamp: None,
        timestamp: String::new(),
        mock_base_fee: None,
        mock_gas_price: None,
        mock_signature_verification: None,
        enable_coverage: false,
        coverage_lcov_path: None,
        resource_calibration: None,
        memory_limit: None,
        restore_preamble: None,
        include_linear_memory: false,
        enable_asset_safety: false,
        pprof_output_path: None,
        ledger_timestamp: Some(1700000000),
        clock_advance_seconds: Some(3600),
    };

    assert_eq!(request.ledger_timestamp, Some(1700000000));
    assert_eq!(request.clock_advance_seconds, Some(3600));
}

#[test]
fn test_simulation_request_fields_are_optional() {
    // Verify that new fields are optional (backward compatibility)
    let request = SimulationRequest {
        envelope_xdr: String::new(),
        result_meta_xdr: String::new(),
        ledger_entries: None,
        control_command: None,
        rewind_step: None,
        fork_params: None,
        harness_reset: false,
        ledger_entries_zstd: None,
        contract_wasm: None,
        wasm_path: None,
        no_cache: false,
        enable_optimization_advisor: false,
        profile: None,
        _timestamp: None,
        timestamp: String::new(),
        mock_base_fee: None,
        mock_gas_price: None,
        mock_signature_verification: None,
        enable_coverage: false,
        coverage_lcov_path: None,
        resource_calibration: None,
        memory_limit: None,
        restore_preamble: None,
        include_linear_memory: false,
        enable_asset_safety: false,
        pprof_output_path: None,
        ledger_timestamp: None,
        clock_advance_seconds: None,
    };

    assert_eq!(request.ledger_timestamp, None);
    assert_eq!(request.clock_advance_seconds, None);
}

#[test]
fn test_timelock_contract_scenario() {
    // Scenario: A hypothetical time-lock contract that should release funds
    // after a specific timestamp.
    // This test demonstrates how clock drift would be used to test such contracts.

    let mut host = SimHost::new(HostConfig::default());

    // Contract lock timestamp is 1700086400 (Nov 16, 2023, 00:00:00 UTC)
    let contract_unlock_time = 1700086400u64;

    // Initialize simulator clock to before the unlock time
    let simulation_start = 1700000000u64; // Nov 15, 2023
    host.init_clock(simulation_start);

    // Verify we're before the unlock time
    assert!(host.get_clock() < contract_unlock_time);

    // Advance time to the unlock moment
    let time_until_unlock = contract_unlock_time - simulation_start;
    host.advance_clock(time_until_unlock.cast_signed());

    // Now the contract should unlock
    assert_eq!(host.get_clock(), contract_unlock_time);

    // Advance a bit more
    host.advance_clock(60); // +1 minute
    assert!(host.get_clock() > contract_unlock_time);
}

#[test]
fn test_clock_with_large_advancements() {
    let mut host = SimHost::new(HostConfig::default());
    host.init_clock(1700000000);

    // Advance by 1 year (approximately)
    let one_year_seconds = 365 * 24 * 3600;
    host.advance_clock(one_year_seconds);

    assert_eq!(
        host.get_clock(),
        1_700_000_000 + one_year_seconds.cast_unsigned()
    );
}

#[test]
fn test_clock_multiple_hosts_independent() {
    let mut host1 = SimHost::new(HostConfig::default());
    let mut host2 = SimHost::new(HostConfig::default());

    host1.init_clock(1_700_000_000);
    host2.init_clock(1_700_000_000);

    host1.advance_clock(3600);
    host2.advance_clock(7200);

    // Hosts should have independent clock states
    assert_eq!(host1.get_clock(), 1_700_003_600);
    assert_eq!(host2.get_clock(), 1_700_007_200);
}

#[test]
fn test_clock_state_across_snapshots_with_concurrent_advancement() {
    let mut host = SimHost::new(HostConfig::default());
    host.init_clock(1_700_000_000);

    // Snapshot 1: Initial state
    let snapshot_t0 = host.capture_snapshot().expect("should capture");

    // Advance clock
    host.advance_clock(1000);
    let clock_t1 = host.get_clock();
    assert_eq!(clock_t1, 1_700_001_000);

    // Snapshot 2: After first advancement
    let snapshot_t1 = host.capture_snapshot().expect("should capture");

    // Further advancement
    host.advance_clock(2000);
    let clock_t2 = host.get_clock();
    assert_eq!(clock_t2, 1_700_003_000);

    // Restore to t0, then to t1, verifying clock is preserved in each
    host.restore_from_snapshot(&snapshot_t0)
        .expect("should restore");
    assert_eq!(
        host.get_clock(),
        clock_t2,
        "clock should continue advancing even after restore"
    );

    host.restore_from_snapshot(&snapshot_t1)
        .expect("should restore");
    assert_eq!(host.get_clock(), clock_t2);

    // One more advancement to verify it continues
    host.advance_clock(500);
    assert_eq!(host.get_clock(), 1_700_003_500);
}
