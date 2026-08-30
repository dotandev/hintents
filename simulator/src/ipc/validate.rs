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

/// The schema is embedded at compile time and parsed+compiled exactly once,
/// on first use, instead of on every incoming request.
static COMPILED_SCHEMA: Lazy<jsonschema::Validator> = Lazy::new(|| {
    let schema_json = include_str!("../../../docs/schema/simulation-request.schema.json");
    let schema: Value = serde_json::from_str(schema_json)
        .unwrap_or_else(|e| panic!("invalid embedded schema JSON: {e}"));
    jsonschema::validator_for(&schema)
        .unwrap_or_else(|e| panic!("schema compilation failed: {e}"))
});

/// Validates JSON input against the simulation-request.schema.json
#[allow(dead_code)]
pub fn validate_request(input: &str) -> Result<Value, String> {
    // parse the incoming JSON
    let instance: Value = serde_json::from_str(input).map_err(|e| e.to_string())?;

    // validate against the globally compiled schema (compiled once, lazily)
    let compiled = &*COMPILED_SCHEMA;
    let errors: Vec<String> = compiled
        .iter_errors(&instance)
        .map(|e| e.to_string())
        .collect();
    if !errors.is_empty() {
        return Err(errors.join(", "));
    }

    Ok(instance)
}
