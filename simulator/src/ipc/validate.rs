// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//
// You may obtain a copy of the License at
//
//
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

//
// You may obtain a copy of the License at
//
//
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

use serde_json::Value;

use super::types::IpcError;

/// Validates a JSON request payload against the simulation-request schema.
///
/// # Errors
///
/// Returns [`IpcError::Validation`] when:
/// - `input` is not valid JSON (malformed bytes / truncated input)
/// - The parsed value does not satisfy the embedded JSON Schema
///   (missing required fields, wrong field types, unexpected properties, …)
///
/// The embedded schema is compiled once at call-time from the bytes baked
/// in at compile-time via `include_str!`, so the call can never panic on
/// a malformed schema either — any schema compile error surfaces as an
/// `IpcError::Validation`.
///
/// Valid payloads are returned unchanged as a [`serde_json::Value`].
#[allow(dead_code)]
pub fn validate_request(input: &str) -> Result<Value, IpcError> {
    // Embed the schema at compile-time — always present, never a runtime path error.
    let schema_json =
        include_str!("../../../docs/schema/simulation-request.schema.json");

    // Compile the embedded schema.  Map any schema-compile error into
    // IpcError::Validation rather than panicking or returning a raw String.
    let schema: Value = serde_json::from_str(schema_json)
        .map_err(|e| IpcError::Validation(format!("invalid embedded schema JSON: {e}")))?;

    let validator = jsonschema::validator_for(&schema)
        .map_err(|e| IpcError::Validation(format!("failed to compile schema: {e}")))?;

    // Parse the caller-supplied JSON.  A malformed payload surfaces here as a
    // descriptive IpcError::Validation instead of propagating a raw serde error
    // or panicking.
    let instance: Value = serde_json::from_str(input).map_err(|e| {
        IpcError::Validation(format!("malformed JSON in request payload: {e}"))
    })?;

    // Collect every schema violation and return them as a single error.
    let errors: Vec<String> = validator
        .iter_errors(&instance)
        .map(|e| e.to_string())
        .collect();

    if !errors.is_empty() {
        return Err(IpcError::Validation(errors.join("; ")));
    }

    Ok(instance)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    // ------------------------------------------------------------------
    // Helpers
    // ------------------------------------------------------------------

    /// Minimal valid payload satisfying every required field.
    fn valid_payload() -> &'static str {
        r#"{
            "version": "1.0.0",
            "request_id": "req-001",
            "network": "testnet",
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC"
        }"#
    }

    // ------------------------------------------------------------------
    // Happy-path: valid payload
    // ------------------------------------------------------------------

    #[test]
    fn test_valid_payload_returns_ok() {
        let result = validate_request(valid_payload());
        assert!(result.is_ok(), "expected Ok for valid payload, got: {result:?}");
    }

    #[test]
    fn test_valid_payload_value_preserved() {
        let value = validate_request(valid_payload()).expect("valid payload must succeed");
        assert_eq!(value["request_id"], "req-001");
        assert_eq!(value["network"], "testnet");
    }

    #[test]
    fn test_valid_payload_with_optional_fields() {
        let input = r#"{
            "version": "1.0.0",
            "request_id": "req-002",
            "network": "public",
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC",
            "ledger_sequence": 42,
            "profile": true
        }"#;
        assert!(validate_request(input).is_ok());
    }

    // ------------------------------------------------------------------
    // Malformed JSON — must never panic; must return IpcError::Validation
    // ------------------------------------------------------------------

    #[test]
    fn test_malformed_json_returns_validation_error() {
        let result = validate_request("{not valid json at all!!!}");
        assert!(
            result.is_err(),
            "expected Err for malformed JSON, got Ok"
        );
        match result.unwrap_err() {
            IpcError::Validation(msg) => {
                assert!(
                    msg.contains("malformed JSON"),
                    "expected 'malformed JSON' in message, got: {msg}"
                );
            }
            other => panic!("expected IpcError::Validation, got: {other:?}"),
        }
    }

    #[test]
    fn test_truncated_json_returns_validation_error() {
        let result = validate_request(r#"{"version": "1.0.0", "request_id":"#);
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    #[test]
    fn test_empty_string_returns_validation_error() {
        let result = validate_request("");
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    /// Regression: malformed JSON must not panic in production code.
    #[test]
    fn test_malformed_json_does_not_panic() {
        // std::panic::catch_unwind proves there is no panic path.
        let result = std::panic::catch_unwind(|| {
            let _ = validate_request("}{broken");
        });
        assert!(result.is_ok(), "validate_request panicked on malformed JSON");
    }

    // ------------------------------------------------------------------
    // Missing required fields
    // ------------------------------------------------------------------

    #[test]
    fn test_missing_version_returns_validation_error() {
        let input = r#"{
            "request_id": "req-003",
            "network": "testnet",
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC"
        }"#;
        let result = validate_request(input);
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    #[test]
    fn test_missing_network_returns_validation_error() {
        let input = r#"{
            "version": "1.0.0",
            "request_id": "req-004",
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC"
        }"#;
        let result = validate_request(input);
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    #[test]
    fn test_all_required_fields_missing_returns_validation_error() {
        let result = validate_request("{}");
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    // ------------------------------------------------------------------
    // Invalid field types
    // ------------------------------------------------------------------

    #[test]
    fn test_network_wrong_type_returns_validation_error() {
        // `network` must be a string enum; passing an integer is a type error.
        let input = r#"{
            "version": "1.0.0",
            "request_id": "req-005",
            "network": 42,
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC"
        }"#;
        let result = validate_request(input);
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    #[test]
    fn test_ledger_sequence_wrong_type_returns_validation_error() {
        let input = r#"{
            "version": "1.0.0",
            "request_id": "req-006",
            "network": "testnet",
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC",
            "ledger_sequence": "not-a-number"
        }"#;
        let result = validate_request(input);
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    #[test]
    fn test_profile_wrong_type_returns_validation_error() {
        let input = r#"{
            "version": "1.0.0",
            "request_id": "req-007",
            "network": "testnet",
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC",
            "profile": "yes"
        }"#;
        let result = validate_request(input);
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    // ------------------------------------------------------------------
    // Unexpected payload structure / additionalProperties
    // ------------------------------------------------------------------

    #[test]
    fn test_unknown_top_level_field_returns_validation_error() {
        // The schema sets `additionalProperties: false`.
        let input = r#"{
            "version": "1.0.0",
            "request_id": "req-008",
            "network": "testnet",
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC",
            "totally_unexpected_field": true
        }"#;
        let result = validate_request(input);
        assert!(
            matches!(result, Err(IpcError::Validation(_))),
            "expected Err for unknown field due to additionalProperties:false"
        );
    }

    #[test]
    fn test_network_invalid_enum_value_returns_validation_error() {
        // `network` must be one of "public" | "testnet" | "futurenet".
        let input = r#"{
            "version": "1.0.0",
            "request_id": "req-009",
            "network": "mainnet",
            "xdr": "AAAA",
            "envelope_xdr": "BBBB",
            "result_meta_xdr": "CCCC"
        }"#;
        let result = validate_request(input);
        assert!(matches!(result, Err(IpcError::Validation(_))));
    }

    // ------------------------------------------------------------------
    // Error message quality
    // ------------------------------------------------------------------

    #[test]
    fn test_validation_error_message_is_descriptive() {
        let result = validate_request("{}");
        match result {
            Err(IpcError::Validation(msg)) => {
                assert!(!msg.is_empty(), "error message must not be empty");
            }
            other => panic!("expected IpcError::Validation, got: {other:?}"),
        }
    }

    #[test]
    fn test_error_display_includes_prefix() {
        let result = validate_request("{bad}");
        let err = result.unwrap_err();
        let display = err.to_string();
        assert!(
            display.starts_with("IPC validation error:"),
            "Display impl must include prefix; got: {display}"
        );
    }
}
