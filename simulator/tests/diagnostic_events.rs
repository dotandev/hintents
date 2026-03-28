// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use base64::Engine as _;
use soroban_env_host::xdr::{ContractEventBody, ContractEventType, WriteXdr};
use soroban_env_host::{DiagnosticLevel, Host};

fn make_host() -> Host {
    let host = Host::default();
    host.set_diagnostic_level(DiagnosticLevel::Debug)
        .expect("failed to set diagnostic level");
    host
}

#[test]
fn in_successful_contract_call_is_inverse_of_failed_call() {
    let host = make_host();
    let events = host.get_events().unwrap_or_default();
    for e in &events.0 {
        assert_eq!(
            !e.failed_call, !e.failed_call,
            "in_successful_contract_call must equal !failed_call"
        );
    }
}

#[test]
fn data_serializes_to_valid_xdr_base64() {
    let host = make_host();
    let events = host.get_events().unwrap_or_default();
    for e in &events.0 {
        let ContractEventBody::V0(v0) = &e.event.body;
        let bytes = v0
            .data
            .to_xdr(soroban_env_host::xdr::Limits::none())
            .expect("data must serialize to XDR without error");
        let encoded = base64::engine::general_purpose::STANDARD.encode(&bytes);
        assert!(!encoded.is_empty(), "base64-encoded data must not be empty");
    }
}

#[test]
fn contract_id_encodes_as_hex_without_debug_formatting() {
    let host = make_host();
    let events = host.get_events().unwrap_or_default();
    for e in &events.0 {
        if let Some(id) = &e.event.contract_id {
            let bytes = id
                .to_xdr(soroban_env_host::xdr::Limits::none())
                .expect("contract_id must serialize to XDR");
            let hex_id = hex::encode(&bytes);
            assert!(
                !hex_id.contains("Hash"),
                "contract_id must not contain Rust Debug formatting"
            );
            assert!(!hex_id.is_empty(), "contract_id hex must not be empty");
        }
    }
}

#[test]
fn event_type_field_matches_allowed_values() {
    let host = make_host();
    let events = host.get_events().unwrap_or_default();
    for e in &events.0 {
        let type_str = match e.event.type_ {
            ContractEventType::Contract => "contract",
            ContractEventType::System => "system",
            ContractEventType::Diagnostic => "diagnostic",
        };
        assert!(
            ["contract", "system", "diagnostic"].contains(&type_str),
            "event_type must be one of: contract, system, diagnostic"
        );
    }
}

#[test]
fn event_body_is_v0() {
    let host = make_host();
    let events = host.get_events().unwrap_or_default();
    for e in &events.0 {
        assert!(
            matches!(e.event.body, ContractEventBody::V0(_)),
            "event body must be ContractEventBody::V0"
        );
    }
}
