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

/// Validates JSON input against the simulation-request.schema.json
#[allow(dead_code)]
pub fn validate_request(input: &str) -> Result<Value, String> {
    // include the schema at compile-time
    let schema_json = include_str!("../../../docs/schema/simulation-request.schema.json");
    let schema: Value = serde_json::from_str(schema_json).unwrap();
    let validator =
        jsonschema::validator_for(&schema).map_err(|e| format!("failed to compile schema: {e}"))?;

    // parse the incoming JSON
    let instance: Value = serde_json::from_str(input).map_err(|e| e.to_string())?;

    // validate against the schema
    let errors: Vec<String> = validator
        .iter_errors(&instance)
        .map(|e| e.to_string())
        .collect();
    if !errors.is_empty() {
        return Err(errors.join(", "));
    }

    Ok(instance)
}
