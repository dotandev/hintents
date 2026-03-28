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

use jsonschema::validator_for;
use serde_json::Value;

/// Validates JSON input against the simulation-request.schema.json
#[allow(dead_code)]
pub fn validate_request(input: &str) -> Result<Value, String> {
    let schema_json = include_str!("../../../docs/schema/simulation-request.schema.json");
    let schema: Value = serde_json::from_str(schema_json).unwrap();
    let compiled = validator_for(&schema).unwrap();

    let instance: Value = serde_json::from_str(input).map_err(|e| e.to_string())?;

    compiled.validate(&instance).map_err(|e| e.to_string())?;

    Ok(instance)
}
