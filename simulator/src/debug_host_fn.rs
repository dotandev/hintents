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
use soroban_env_host::xdr::{ContractEventBody, ContractEventType, ScString, ScVal};
use std::collections::HashMap;
use std::path::Path;

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

/// A stub definition for overriding a specific host function during simulation.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct HostFnStub {
    /// Name of the host function (e.g. "get_ledger_timestamp", "prng_reseed", "get_sequence").
    pub function_name: String,
    /// Value to return when intercepted, represented as a generic JSON value.
    pub return_value: serde_json::Value,
    /// Optional limit on how many times the stub will fire. None = infinite.
    pub max_calls: Option<usize>,
}

/// JSON configuration file schema for host function overrides.
#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq, Eq)]
pub struct HostFnOverrideConfig {
    pub overrides: Vec<HostFnStub>,
}

/// Registry for intercepting and overriding host functions in test and replay mode.
#[derive(Debug, Clone, Default)]
pub struct HostFunctionOverrideRegistry {
    stubs: HashMap<String, HostFnStub>,
    call_counts: HashMap<String, usize>,
}

impl HostFunctionOverrideRegistry {
    /// Creates a new empty override registry.
    pub fn new() -> Self {
        Self::default()
    }

    /// Loads overrides from a JSON configuration string.
    pub fn from_json(json_str: &str) -> Result<Self, serde_json::Error> {
        let config: HostFnOverrideConfig = serde_json::from_str(json_str)?;
        let mut registry = Self::new();
        for stub in config.overrides {
            registry.register_stub(stub);
        }
        Ok(registry)
    }

    /// Loads overrides from a JSON configuration file.
    pub fn from_file<P: AsRef<Path>>(path: P) -> Result<Self, std::io::Error> {
        let content = std::fs::read_to_string(path)?;
        Self::from_json(&content).map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidData, e))
    }

    /// Registers a new host function override stub.
    pub fn register_stub(&mut self, stub: HostFnStub) {
        self.stubs.insert(stub.function_name.clone(), stub);
    }

    /// Checks if a host function has an active override configured.
    pub fn is_overridden(&self, function_name: &str) -> bool {
        if let Some(stub) = self.stubs.get(function_name) {
            if let Some(max) = stub.max_calls {
                let current = self.call_counts.get(function_name).copied().unwrap_or(0);
                return current < max;
            }
            return true;
        }
        false
    }

    /// Intercepts a host function call and returns its configured stub value, incrementing the counter.
    /// Returns `None` if no override is configured or if `max_calls` has been reached.
    pub fn intercept(&mut self, function_name: &str) -> Option<&serde_json::Value> {
        if let Some(stub) = self.stubs.get(function_name) {
            let count = self.call_counts.entry(function_name.to_string()).or_insert(0);
            if let Some(max) = stub.max_calls {
                if *count >= max {
                    return None;
                }
            }
            *count += 1;
            Some(&stub.return_value)
        } else {
            None
        }
    }

    /// Returns how many times a given stubbed host function has been intercepted.
    pub fn get_call_count(&self, function_name: &str) -> usize {
        self.call_counts.get(function_name).copied().unwrap_or(0)
    }

    /// Resets all invocation counts to zero.
    pub fn reset_counts(&mut self) {
        self.call_counts.clear();
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

    #[test]
    fn test_override_registry_from_json() {
        let json_config = r#"{
            "overrides": [
                {
                    "function_name": "get_ledger_timestamp",
                    "return_value": 1700000000,
                    "max_calls": 2
                },
                {
                    "function_name": "prng_reseed",
                    "return_value": "mock_seed_12345"
                }
            ]
        }"#;

        let mut registry = HostFunctionOverrideRegistry::from_json(json_config)
            .expect("valid JSON configuration");

        assert!(registry.is_overridden("get_ledger_timestamp"));
        assert!(registry.is_overridden("prng_reseed"));
        assert!(!registry.is_overridden("unknown_fn"));

        // Intercept get_ledger_timestamp
        let val1 = registry.intercept("get_ledger_timestamp").expect("stubbed");
        assert_eq!(val1, &serde_json::json!(1700000000));
        assert_eq!(registry.get_call_count("get_ledger_timestamp"), 1);

        let val2 = registry.intercept("get_ledger_timestamp").expect("stubbed second call");
        assert_eq!(val2, &serde_json::json!(1700000000));
        assert_eq!(registry.get_call_count("get_ledger_timestamp"), 2);

        // Exceeded max_calls = 2
        assert!(!registry.is_overridden("get_ledger_timestamp"));
        assert!(registry.intercept("get_ledger_timestamp").is_none());

        // Reset counts re-enables interception
        registry.reset_counts();
        assert_eq!(registry.get_call_count("get_ledger_timestamp"), 0);
        assert!(registry.is_overridden("get_ledger_timestamp"));
        assert!(registry.intercept("get_ledger_timestamp").is_some());
    }
}
