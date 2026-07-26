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

use once_cell::sync::Lazy;
use serde_json::Value;

/// The compiled JSON Schema validator for `simulation-request.schema.json`.
///
/// Compiling a JSON Schema is comparatively expensive (parsing the schema
/// document and building the internal validation graph), so this is done
/// once, lazily, on first use, and the resulting `Validator` is reused for
/// every subsequent request instead of being rebuilt from scratch each time.
static SCHEMA_VALIDATOR: Lazy<jsonschema::Validator> = Lazy::new(|| {
    // include the schema at compile-time
    let schema_json = include_str!("../../../docs/schema/simulation-request.schema.json");
    let schema: Value = serde_json::from_str(schema_json)
        .expect("simulation-request.schema.json must be valid JSON");
    jsonschema::validator_for(&schema)
        .expect("simulation-request.schema.json must be a valid JSON Schema")
});

/// Validates JSON input against the simulation-request.schema.json
#[allow(dead_code)]
pub fn validate_request(input: &str) -> Result<Value, String> {
    // parse the incoming JSON
    let instance: Value = serde_json::from_str(input).map_err(|e| e.to_string())?;

    // validate against the schema (already compiled once, above)
    let errors: Vec<String> = SCHEMA_VALIDATOR
        .iter_errors(&instance)
        .map(|e| e.to_string())
        .collect();
    if !errors.is_empty() {
        return Err(errors.join(", "));
    }

    Ok(instance)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_malformed_json() {
        let err = validate_request("not json").unwrap_err();
        assert!(!err.is_empty());
    }

    #[test]
    fn rejects_missing_required_fields() {
        // Exercises the schema validator (and its lazy compilation) against
        // a syntactically valid but schema-invalid instance.
        let err = validate_request("{}").unwrap_err();
        assert!(!err.is_empty());
    }

    #[test]
    fn repeated_calls_reuse_the_cached_validator() {
        // The schema validator is compiled once, lazily, and reused across
        // calls. This doesn't directly observe caching (that's a
        // Lazy<T> guarantee), but confirms repeated validation against the
        // shared validator remains consistent and doesn't panic or corrupt
        // state under repeated use.
        for _ in 0..50 {
            assert!(validate_request("{}").is_err());
            assert!(validate_request("not json").is_err());
        }
    }
}
