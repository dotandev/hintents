// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! End-to-end tests verifying that `SimulationRequest` fields (`protocol_version`,
//! `ledger_timestamp`, `clock_advance_seconds`, and `ledger_sequence`) deserialize
//! cleanly and reach the Soroban host `LedgerInfo` context without silent drops.

use base64::Engine as _;
use erst_sim::runner::{host_protocol_version, HostConfig, SimHost, SimHostError};
use erst_sim::types::SimulationRequest;
use serde_json::{json, Value};
use soroban_env_host::xdr::{
    ContractDataDurability, ContractDataEntry, ContractId, ExtensionPoint, Hash, LedgerEntry,
    LedgerEntryData, LedgerEntryExt, LedgerKey, LedgerKeyContractData, Limits, ScAddress, ScVal,
    TransactionEnvelope, TransactionV1Envelope, WriteXdr,
};
use std::io::Write as _;
use std::process::{Command, Stdio};

fn build_request_json(extra: &Value) -> String {
    let mut base = json!({
        "envelope_xdr": minimal_envelope_base64(),
        "result_meta_xdr": "",
        "enable_optimization_advisor": false,
    });
    if let (Some(base_obj), Some(extra_obj)) = (base.as_object_mut(), extra.as_object()) {
        for (k, v) in extra_obj {
            base_obj.insert(k.clone(), v.clone());
        }
    }
    base.to_string()
}

fn minimal_envelope_base64() -> String {
    let envelope = TransactionEnvelope::Tx(TransactionV1Envelope::default());
    let bytes = envelope
        .to_xdr(Limits::none())
        .expect("default TransactionEnvelope should encode to XDR");
    base64::engine::general_purpose::STANDARD.encode(bytes)
}

fn run_simulator_binary(request_json: &str) -> Value {
    let mut child = Command::new(env!("CARGO_BIN_EXE_erst-sim"))
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("failed to spawn erst-sim binary");

    child
        .stdin
        .take()
        .expect("stdin pipe should exist")
        .write_all(request_json.as_bytes())
        .expect("failed to write request JSON to erst-sim stdin");

    let output = child
        .wait_with_output()
        .expect("failed to wait on erst-sim process");
    let stdout = String::from_utf8(output.stdout).expect("stdout must be UTF-8");
    serde_json::from_str(&stdout).unwrap_or_else(|err| {
        let stderr = String::from_utf8_lossy(&output.stderr);
        panic!("invalid JSON from erst-sim ({err}): stdout={stdout:?}, stderr={stderr:?}");
    })
}

#[test]
fn protocol_version_and_clock_fields_reach_host_ledger_info() {
    let expected_proto = host_protocol_version();
    let payload = build_request_json(&json!({
        "ledger_sequence": 512,
        "ledger_timestamp": 1_700_000_000_u64,
        "clock_advance_seconds": 3_600_i64,
        "protocol_version": expected_proto,
    }));

    let request: SimulationRequest =
        serde_json::from_str(&payload).expect("SimulationRequest JSON should deserialize");
    assert_eq!(request.protocol_version, Some(expected_proto));
    assert_eq!(request.ledger_timestamp, Some(1_700_000_000));
    assert_eq!(request.clock_advance_seconds, Some(3_600));

    let mut host = SimHost::new(HostConfig::default());
    host.apply_request_ledger_context(&request)
        .expect("supported protocol_version should configure host ledger context");

    let info = host
        .ledger_info()
        .expect("host ledger_info should be populated after setup");
    assert_eq!(info.protocol_version, expected_proto);
    assert_eq!(info.sequence_number, 512);
    assert_eq!(info.timestamp, 1_700_003_600);
    assert_eq!(host.get_clock(), 1_700_003_600);
}

#[test]
fn protocol_version_22_reaches_host_even_without_ledger_sequence() {
    let payload = build_request_json(&json!({
        "ledger_timestamp": 1_700_000_000_u64,
        "clock_advance_seconds": 3_600_i64,
        "protocol_version": 22,
    }));

    let request: SimulationRequest =
        serde_json::from_str(&payload).expect("SimulationRequest JSON should deserialize");
    assert_eq!(request.protocol_version, Some(22));
    assert_eq!(request.ledger_sequence, None);

    let mut host = SimHost::new(HostConfig::default());
    let err = host.apply_request_ledger_context(&request).expect_err(
        "host built for a newer protocol must reject protocol 22 rather than ignore it",
    );
    assert!(
        matches!(err, SimHostError::Host(_)),
        "expected HostError from soroban-env-host, got: {err:?}"
    );
    assert!(
        err.to_string().contains("too old for host"),
        "unexpected host rejection message: {err}"
    );

    let info = host
        .ledger_info()
        .expect("host writes LedgerInfo prior to protocol floor validation");
    assert_eq!(info.protocol_version, 22);
    assert_eq!(info.timestamp, 1_700_003_600);
}

#[test]
fn omitted_protocol_version_uses_default_host_protocol() {
    let payload = build_request_json(&json!({
        "ledger_sequence": 42,
        "ledger_timestamp": 1_700_000_000_u64,
    }));

    let request: SimulationRequest =
        serde_json::from_str(&payload).expect("SimulationRequest JSON without protocol_version");
    assert_eq!(request.protocol_version, None);

    let mut host = SimHost::new(HostConfig::default());
    host.apply_request_ledger_context(&request)
        .expect("omitted protocol_version should default cleanly to host protocol");

    let info = host.ledger_info().expect("ledger_info should be set");
    assert_eq!(info.protocol_version, host_protocol_version());
    assert_eq!(info.sequence_number, 42);
    assert_eq!(info.timestamp, 1_700_000_000);
}

#[test]
fn ledger_context_survives_ledger_entry_injection() {
    let expected_proto = host_protocol_version();
    let payload = build_request_json(&json!({
        "ledger_sequence": 77,
        "ledger_timestamp": 1_700_000_000_u64,
        "clock_advance_seconds": 120_i64,
        "protocol_version": expected_proto,
    }));
    let request: SimulationRequest = serde_json::from_str(&payload).unwrap();

    let mut host = SimHost::new(HostConfig::default());
    host.apply_request_ledger_context(&request).unwrap();

    let contract = ScAddress::Contract(ContractId(Hash([3u8; 32])));
    let key_val = ScVal::U32(1);
    let key = LedgerKey::ContractData(LedgerKeyContractData {
        contract: contract.clone(),
        key: key_val.clone(),
        durability: ContractDataDurability::Persistent,
    });
    let entry = LedgerEntry {
        last_modified_ledger_seq: 77,
        data: LedgerEntryData::ContractData(ContractDataEntry {
            ext: ExtensionPoint::V0,
            contract,
            key: key_val,
            durability: ContractDataDurability::Persistent,
            val: ScVal::U64(99),
        }),
        ext: LedgerEntryExt::V0,
    };

    host.set_ledger_entry(key, entry)
        .expect("ledger entry injection should preserve ledger context");

    let info = host.ledger_info().unwrap();
    assert_eq!(info.protocol_version, expected_proto);
    assert_eq!(info.sequence_number, 77);
    assert_eq!(info.timestamp, 1_700_000_120);
}

#[test]
fn binary_executes_request_with_default_and_explicit_protocol_version() {
    let default_req = build_request_json(&json!({
        "ledger_sequence": 100,
        "ledger_timestamp": 1_700_000_000_u64,
        "clock_advance_seconds": 3_600_i64,
    }));
    let default_resp = run_simulator_binary(&default_req);
    assert_eq!(
        default_resp.get("status").and_then(Value::as_str),
        Some("success"),
        "expected omitted protocol_version to succeed: {default_resp}"
    );

    let explicit_supported_req = build_request_json(&json!({
        "ledger_timestamp": 1_700_000_000_u64,
        "clock_advance_seconds": 3_600_i64,
        "protocol_version": host_protocol_version(),
    }));
    let explicit_supported_resp = run_simulator_binary(&explicit_supported_req);
    assert_eq!(
        explicit_supported_resp.get("status").and_then(Value::as_str),
        Some("success"),
        "expected supported protocol_version without ledger_sequence to succeed: {explicit_supported_resp}"
    );

    let older_proto_req = build_request_json(&json!({
        "ledger_timestamp": 1_700_000_000_u64,
        "clock_advance_seconds": 3_600_i64,
        "protocol_version": 22,
    }));
    let older_proto_resp = run_simulator_binary(&older_proto_req);
    assert_eq!(
        older_proto_resp.get("status").and_then(Value::as_str),
        Some("error"),
        "expected protocol_version 22 to reach host floor check: {older_proto_resp}"
    );
    let error_msg = older_proto_resp
        .get("error")
        .and_then(Value::as_str)
        .unwrap_or("");
    assert!(
        error_msg.contains("protocol version"),
        "expected protocol version error from host, got: {error_msg}"
    );
}
