// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Extracts contract-emitted debug strings from Soroban diagnostic events.
//!
//! Soroban contracts invoke `log_from_linear_memory` to emit UTF-8 debug
//! strings. The host records these as `Diagnostic` events whose first topic
//! is the symbol `"log"`. This module locates those events and decodes them
//! into plain strings so the simulator can surface them in `SimulationResponse.logs`.

use soroban_env_host::events::Events;
use soroban_env_host::xdr::{ContractEventBody, ContractEventType, ScString, ScVal};

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
        ScVal::Symbol(s) => std::str::from_utf8(s.as_slice())
            .unwrap_or("<invalid symbol>")
            .to_string(),
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
    fn empty_events_produces_empty_log() {
        assert!(extract_debug_logs(&Events(vec![])).is_empty());
    }
}
