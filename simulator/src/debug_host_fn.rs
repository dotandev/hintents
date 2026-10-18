// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Extracts contract-emitted debug strings from Soroban diagnostic events.
//!
//! Soroban contracts invoke `log_from_linear_memory` to emit UTF-8 debug
//! strings. The host records these as `Diagnostic` events whose first topic
//! is the symbol `"log"`. This module locates those events and decodes them
//! into plain strings so the simulator can surface them in `SimulationResponse.logs`.

use crate::types::symbol;
use serde::{Deserialize, Serialize};
use soroban_env_host::events::Events;
use soroban_env_host::xdr::{
    ContractEventBody, ContractEventType, ScString, ScVal, ScVec, VecM,
};
use std::collections::HashMap;
use std::fs;
use std::path::Path;

/// A registry of host-function stubs to be used only during local test-mode
/// simulations. User-supplied overrides are kept as raw JSON values so a single
/// override file can provide scalar values, strings, and nested arrays for the
/// specific host functions that need deterministic behavior.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TestModeHostFnRegistry {
    #[serde(default)]
    #[serde(flatten)]
    overrides: HashMap<String, serde_json::Value>,
}

impl TestModeHostFnRegistry {
    /// Creates an empty registry.
    pub fn new() -> Self {
        Self::default()
    }

    /// Loads a registry from a JSON file.
    /// Accepts either a direct object (for example `{ "ledger_timestamp": 1234 }`)
    /// or a wrapping object containing `host_functions` / `overrides`.
    pub fn load_from_file(path: impl AsRef<Path>) -> Result<Self, String> {
        let raw = fs::read_to_string(path.as_ref())
            .map_err(|err| format!("failed to read override file {}: {err}", path.as_ref().display()))?;
        Self::from_json_str(&raw)
    }

    /// Alias for `load_from_file` used by test harnesses and callers that prefer a
    /// shorter API name.
    pub fn from_file(path: impl AsRef<Path>) -> Result<Self, String> {
        Self::load_from_file(path)
    }

    /// Alias for `from_json_str`.
    pub fn parse(raw: &str) -> Result<Self, String> {
        Self::from_json_str(raw)
    }

    /// Parses a registry from a JSON string.
    pub fn from_json_str(raw: &str) -> Result<Self, String> {
        let value = serde_json::from_str::<serde_json::Value>(raw)
            .map_err(|err| format!("failed to parse test-mode override JSON: {err}"))?;
        Self::from_json_value(&value)
    }

    /// Parses the JSON representation used by the override file.
    pub fn from_json_value(value: &serde_json::Value) -> Result<Self, String> {
        let mut map = HashMap::new();

        match value {
            serde_json::Value::Object(obj) => {
                if let Some(host_functions) = obj.get("host_functions").or_else(|| obj.get("overrides")) {
                    let nested = match host_functions {
                        serde_json::Value::Object(entries) => entries,
                        _ => {
                            return Err("test-mode overrides must be a JSON object keyed by host-function name".to_string());
                        }
                    };
                    for (name, override_value) in nested {
                        map.insert(name.clone(), override_value.clone());
                    }
                } else {
                    for (name, override_value) in obj {
                        map.insert(name.clone(), override_value.clone());
                    }
                }
            }
            _ => {
                return Err("test-mode override document must be a JSON object keyed by host-function name".to_string());
            }
        }

        Ok(Self { overrides: map })
    }

    /// Returns true when a stub exists for the given host function name.
    pub fn contains(&self, host_fn_name: &str) -> bool {
        self.overrides.contains_key(host_fn_name)
    }

    /// Returns the raw override value for a host function name, if configured.
    pub fn stub_value(&self, host_fn_name: &str) -> Option<&serde_json::Value> {
        self.overrides.get(host_fn_name)
    }

    /// Exposes the configured overrides as a map for callers that need to inspect
    /// or merge a registry before applying it.
    pub fn overrides(&self) -> &HashMap<String, serde_json::Value> {
        &self.overrides
    }

    /// Returns true when the registry has no configured stubs.
    pub fn is_empty(&self) -> bool {
        self.overrides.is_empty()
    }

    /// Resolves a configured override into the `ScVal` that the simulator should
    /// inject for this host-function call. This supports the common test-mode
    /// cases: fixed integers, strings, booleans, and arrays of primitive values.
    pub fn intercept(&self, host_fn_name: &str) -> Option<Result<ScVal, String>> {
        let value = self.stub_value(host_fn_name)?;
        Some(match json_value_to_scval(value) {
            Ok(sc) => Ok(sc),
            Err(err) => Err(format!("host function {host_fn_name} override is not supported: {err}")),
        })
    }

    /// Registers or replaces a stub override for a host function.
    pub fn insert(&mut self, host_fn_name: impl Into<String>, value: serde_json::Value) {
        self.overrides.insert(host_fn_name.into(), value);
    }

    /// Merges another registry into this one.
    pub fn extend(&mut self, other: &Self) {
        for (name, value) in &other.overrides {
            self.overrides.insert(name.clone(), value.clone());
        }
    }
}

fn json_value_to_scval(value: &serde_json::Value) -> Result<ScVal, String> {
    match value {
        serde_json::Value::Null => Ok(ScVal::Void),
        serde_json::Value::Bool(b) => Ok(ScVal::Bool(*b)),
        serde_json::Value::Number(n) => json_number_to_scval(n),
        serde_json::Value::String(s) => {
            let bytes: Vec<u8> = s.as_bytes().to_vec();
            let sc_bytes: Result<[u8; 32], _> = bytes.as_slice().try_into();
            if let Ok(bytes32) = sc_bytes {
                return Ok(ScVal::Bytes(bytes32.into()));
            }
            let string_data = bytes
                .try_into()
                .map_err(|_| "string payload exceeds the maximum length supported by the host".to_string())?;
            Ok(ScVal::String(ScString(string_data)))
        }
        serde_json::Value::Array(items) => {
            let converted = items
                .iter()
                .map(json_value_to_scval)
                .collect::<Result<Vec<_>, _>>()?;
            let vec = ScVec(VecM::try_from(converted).map_err(|_| "override array does not fit into the host's vector representation".to_string())?);
            Ok(ScVal::Vec(Some(vec)))
        }
        serde_json::Value::Object(_) => Err("JSON objects are not yet supported as host-function return values".to_string()),
    }
}

fn json_number_to_scval(num: &serde_json::Number) -> Result<ScVal, String> {
    if let Some(v) = num.as_i64() {
        return Ok(ScVal::I64(v));
    }
    if let Some(v) = num.as_u64() {
        return Ok(ScVal::U64(v));
    }
    if let Some(v) = num.as_f64() {
        if v.fract() == 0.0 && v >= i64::MIN as f64 && v <= i64::MAX as f64 {
            return Ok(ScVal::I64(v as i64));
        }
        return Err("floating-point overrides must represent an integer for host-function stubs".to_string());
    }
    Err("unsupported numeric override for host-function stub".to_string())
}

/// Convenience helper for callers that just need the resolved stub value or a
/// clear error if the override is missing or malformed.
pub fn resolve_test_mode_host_fn_override(
    registry: &TestModeHostFnRegistry,
    host_fn_name: &str,
) -> Result<ScVal, String> {
    registry
        .intercept(host_fn_name)
        .unwrap_or_else(|| Err(format!("no test-mode override registered for host function {host_fn_name}")))
        .map_err(|err| format!("invalid override for host function {host_fn_name}: {err}"))
}

/// Extracts debug log strings emitted by contracts via `log_from_linear_memory`.
///
/// Returns one `String` per log call, in emission order.
/// Non-log diagnostic events and events that cannot be decoded are silently skipped.
pub fn extract_debug_logs(events: &Events) -> Vec<String> {
    let mut out = Vec::new();

    for entry in &events.0 {
        if entry.event.type_ != ContractEventType::Diagnostic {
            continue;
        }

        let ContractEventBody::V0(v0) = &entry.event.body;

        let is_log = v0
            .topics
            .first()
            .is_some_and(|t| matches!(t, ScVal::Symbol(s) if s.as_slice() == b"log"));

        if !is_log {
            continue;
        }

        out.push(decode_log_data(&v0.data));
    }

    out
}

fn decode_log_data(data: &ScVal) -> String {
    match data {
        ScVal::String(ScString(s)) => std::str::from_utf8(s.as_slice())
            .unwrap_or("<invalid UTF-8>")
            .to_string(),
        ScVal::Vec(Some(items)) => items
            .0
            .iter()
            .map(scval_to_display)
            .collect::<Vec<_>>()
            .join(" "),
        other => scval_to_display(other),
    }
}

fn scval_to_display(val: &ScVal) -> String {
    match val {
        ScVal::String(ScString(s)) => std::str::from_utf8(s.as_slice())
            .unwrap_or("<invalid UTF-8>")
            .to_string(),
        ScVal::Symbol(s) => symbol::symbol_to_string(s.as_slice()),
        ScVal::I32(n) => n.to_string(),
        ScVal::U32(n) => n.to_string(),
        ScVal::I64(n) => n.to_string(),
        ScVal::U64(n) => n.to_string(),
        ScVal::Bool(b) => b.to_string(),
        ScVal::Void => "void".to_string(),
        other => format!("{:?}", other),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_env_host::events::{Events, HostEvent};
    use soroban_env_host::xdr::{
        ContractEvent, ContractEventBody, ContractEventType, ContractEventV0, ScString, ScSymbol,
        ScVal, ScVec, VecM,
    };

    fn make_log_event(data: ScVal) -> HostEvent {
        HostEvent {
            event: ContractEvent {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract_id: None,
                type_: ContractEventType::Diagnostic,
                body: ContractEventBody::V0(ContractEventV0 {
                    topics: vec![ScVal::Symbol(ScSymbol(
                        "log".as_bytes().try_into().expect("symbol fits"),
                    ))]
                    .try_into()
                    .expect("topics fit"),
                    data,
                }),
            },
            failed_call: false,
        }
    }

    #[test]
    fn extracts_string_log() {
        let event = make_log_event(ScVal::String(ScString(
            "hello from contract".as_bytes().try_into().expect("fits"),
        )));
        let logs = extract_debug_logs(&Events(vec![event]));
        assert_eq!(logs.len(), 1);
        assert_eq!(logs[0], "hello from contract");
    }

    #[test]
    fn skips_non_log_diagnostic_events() {
        let event = HostEvent {
            event: ContractEvent {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract_id: None,
                type_: ContractEventType::Diagnostic,
                body: ContractEventBody::V0(ContractEventV0 {
                    topics: vec![ScVal::Symbol(ScSymbol(
                        "fn_call".as_bytes().try_into().expect("fits"),
                    ))]
                    .try_into()
                    .expect("fits"),
                    data: ScVal::Void,
                }),
            },
            failed_call: false,
        };
        let logs = extract_debug_logs(&Events(vec![event]));
        assert!(logs.is_empty());
    }

    #[test]
    fn skips_non_diagnostic_events() {
        let event = HostEvent {
            event: ContractEvent {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract_id: None,
                type_: ContractEventType::Contract,
                body: ContractEventBody::V0(ContractEventV0 {
                    topics: vec![ScVal::Symbol(ScSymbol(
                        "log".as_bytes().try_into().expect("fits"),
                    ))]
                    .try_into()
                    .expect("fits"),
                    data: ScVal::String(ScString(
                        "should be ignored".as_bytes().try_into().expect("fits"),
                    )),
                }),
            },
            failed_call: false,
        };
        let logs = extract_debug_logs(&Events(vec![event]));
        assert!(logs.is_empty());
    }

    #[test]
    fn decodes_vec_data_as_space_joined() {
        let items: Vec<ScVal> = vec![
            ScVal::String(ScString("x =".as_bytes().try_into().expect("fits"))),
            ScVal::I32(42),
        ];
        let vec_val = ScVal::Vec(Some(ScVec(VecM::try_from(items).expect("fits"))));
        let event = make_log_event(vec_val);
        let logs = extract_debug_logs(&Events(vec![event]));
        assert_eq!(logs.len(), 1);
        assert_eq!(logs[0], "x = 42");
    }

    #[test]
    fn renders_invalid_symbol_without_panicking() {
        // A Symbol whose bytes are not valid UTF-8 must not crash the formatter.
        let bad_symbol = ScSymbol(vec![0x66, 0x80, 0x67].try_into().expect("fits"));
        let items: Vec<ScVal> = vec![ScVal::Symbol(bad_symbol)];
        let vec_val = ScVal::Vec(Some(ScVec(VecM::try_from(items).expect("fits"))));
        let event = make_log_event(vec_val);
        let logs = extract_debug_logs(&Events(vec![event]));
        assert_eq!(logs.len(), 1);
        assert_eq!(logs[0], crate::types::symbol::INVALID_SYMBOL_PLACEHOLDER);
    }

    #[test]
    fn empty_events_produces_empty_log() {
        assert!(extract_debug_logs(&Events(vec![])).is_empty());
    }
}
