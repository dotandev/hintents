// Copyright 2025 Erst Users
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

use jsonschema::JSONSchema;
use serde_json::Value;

/// Validates JSON input against the simulation-request.schema.json
#[allow(dead_code)]
pub fn validate_request(input: &str) -> Result<Value, String> {
    // include the schema at compile-time
    let schema_json = include_str!("../../../docs/schema/simulation-request.schema.json");
    let schema: Value = serde_json::from_str(schema_json).unwrap();
    let compiled = JSONSchema::compile(&schema).unwrap();

    // parse the incoming JSON
    let instance: Value = serde_json::from_str(input).map_err(|e| e.to_string())?;

    // validate against the schema
    compiled
        .validate(&instance)
        .map_err(|errors| errors.map(|e| e.to_string()).collect::<Vec<_>>().join(", "))?;

    Ok(instance)
}
