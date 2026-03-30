// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Tests for signature verification mocking functionality

use simulator::types::SimulationRequest;

#[test]
fn test_signature_verification_mock_true() {
    let request = SimulationRequest {
        envelope_xdr: String::new(),
        result_meta_xdr: String::new(),
        ledger_entries: None,
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
        mock_signature_verification: Some(true),
        enable_coverage: false,
        coverage_lcov_path: None,
        resource_calibration: None,
        memory_limit: None,
        restore_preamble: None,
        include_linear_memory: false,
    };

    assert!(request.mock_signature_verification.is_some());
    assert!(request.mock_signature_verification.unwrap());
}

#[test]
fn test_signature_verification_mock_false() {
    let request = SimulationRequest {
        envelope_xdr: String::new(),
        result_meta_xdr: String::new(),
        ledger_entries: None,
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
        mock_signature_verification: Some(false),
        enable_coverage: false,
        coverage_lcov_path: None,
        resource_calibration: None,
        memory_limit: None,
        restore_preamble: None,
        include_linear_memory: false,
    };

    assert!(request.mock_signature_verification.is_some());
    assert!(!request.mock_signature_verification.unwrap());
}

#[test]
fn test_signature_verification_mock_disabled() {
    let request = SimulationRequest {
        envelope_xdr: String::new(),
        result_meta_xdr: String::new(),
        ledger_entries: None,
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
    };

    assert!(request.mock_signature_verification.is_none());
}
