// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Tests for signature verification mocking functionality

use crate::types::SimulationRequest;

#[test]
fn test_signature_verification_mock_true() {
    let request = SimulationRequest {
        envelope_xdr: String::new(),
        result_meta_xdr: String::new(),
        ledger_entries: None,
        contract_wasm: None,
        wasm_path: None,
        enable_optimization_advisor: false,
        profile: None,
        timestamp: String::new(),
        mock_base_fee: None,
        mock_gas_price: None,
        mock_signature_verification: Some(true),
        enable_coverage: false,
        coverage_lcov_path: None,
        resource_calibration: None,
        memory_limit: None,
        restore_preamble: None,
    };

    assert!(request.mock_signature_verification.is_some());
    assert_eq!(request.mock_signature_verification.unwrap(), true);
}

#[test]
fn test_signature_verification_mock_false() {
    let request = SimulationRequest {
        envelope_xdr: String::new(),
        result_meta_xdr: String::new(),
        ledger_entries: None,
        contract_wasm: None,
        wasm_path: None,
        enable_optimization_advisor: false,
        profile: None,
        timestamp: String::new(),
        mock_base_fee: None,
        mock_gas_price: None,
        mock_signature_verification: Some(false),
        enable_coverage: false,
        coverage_lcov_path: None,
        resource_calibration: None,
        memory_limit: None,
        restore_preamble: None,
    };

    assert!(request.mock_signature_verification.is_some());
    assert_eq!(request.mock_signature_verification.unwrap(), false);
}

#[test]
fn test_signature_verification_mock_disabled() {
    let request = SimulationRequest {
        envelope_xdr: String::new(),
        result_meta_xdr: String::new(),
        ledger_entries: None,
        contract_wasm: None,
        wasm_path: None,
        enable_optimization_advisor: false,
        profile: None,
        timestamp: String::new(),
        mock_base_fee: None,
        mock_gas_price: None,
        mock_signature_verification: None,
        enable_coverage: false,
        coverage_lcov_path: None,
        resource_calibration: None,
        memory_limit: None,
        restore_preamble: None,
    };

    assert!(request.mock_signature_verification.is_none());
}
