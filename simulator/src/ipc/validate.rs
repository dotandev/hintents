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

use super::types::{IpcError, ValidationErrorDetail};

/// Validates JSON input against the simulation-request.schema.json
///
/// Returns the parsed [`Value`] on success, or [`IpcError::Validation`] with
/// structured details (path, schema_path, line number, suggestion) on failure.
#[allow(dead_code)]
pub fn validate_request(input: &str) -> Result<Value, IpcError> {
    // include the schema at compile-time
    let schema_json = include_str!("../../../docs/schema/simulation-request.schema.json");
    let schema: Value = serde_json::from_str(schema_json).map_err(|e| {
        IpcError::Validation {
            message: format!("invalid schema JSON: {e}"),
            details: vec![ValidationErrorDetail {
                path: "/".into(),
                schema_path: "/".into(),
                message: format!("failed to parse embedded schema: {e}"),
                line: Some(e.line()),
                suggestion: Some("the bundled schema is corrupted; rebuild the project".into()),
            }],
        }
    })?;
    let compiled = jsonschema::validator_for(&schema).map_err(|e| {
        IpcError::Validation {
            message: format!("schema compilation failed: {e}"),
            details: vec![ValidationErrorDetail {
                path: "/".into(),
                schema_path: "/".into(),
                message: format!("could not compile schema: {e}"),
                line: None,
                suggestion: Some("the bundled schema may be invalid JSON Schema; review docs/schema/simulation-request.schema.json".into()),
            }],
        }
    })?;

    // parse the incoming JSON
    let instance: Value = serde_json::from_str(input).map_err(|e| {
        IpcError::Validation {
            message: format!("invalid JSON input: {e}"),
            details: vec![ValidationErrorDetail {
                path: "/".into(),
                schema_path: "/".into(),
                message: e.to_string(),
                line: Some(e.line()),
                suggestion: Some("fix the JSON syntax at the indicated line".into()),
            }],
        }
    })?;

    // validate against the schema
    let errors: Vec<ValidationErrorDetail> = compiled
        .iter_errors(&instance)
        .map(|e| ValidationErrorDetail {
            path: e.instance_path().to_string(),
            schema_path: e.schema_path().to_string(),
            message: e.to_string(),
            line: None,
            suggestion: suggest_for_error(&e),
        })
        .collect();

    if !errors.is_empty() {
        let message = format!(
            "{} validation error(s)",
            errors.len()
        );
        return Err(IpcError::Validation { message, details: errors });
    }

    Ok(instance)
}

fn suggest_for_error(error: &jsonschema::ValidationError<'_>) -> Option<String> {
    let path = error.instance_path().to_string();
    let msg = error.to_string();

    if msg.contains("is not of type") {
        let expected = if msg.contains("'string'") {
            "a string"
        } else if msg.contains("'integer'") {
            "an integer"
        } else if msg.contains("'number'") {
            "a number"
        } else if msg.contains("'boolean'") {
            "a boolean"
        } else if msg.contains("'array'") {
            "an array"
        } else if msg.contains("'object'") {
            "an object"
        } else {
            "the expected type"
        };
        return Some(format!("expected {expected} at \"{path}\""));
    }

    if msg.contains("is not valid under") || msg.contains("is not valid:") {
        return Some(format!("check the constraints defined for \"{path}\" in the schema"));
    }

    if msg.contains("required") {
        let missing = msg
            .split_whitespace()
            .last()
            .unwrap_or("a required field");
        return Some(format!("add the missing field {missing} to \"{path}\""));
    }

    if msg.contains("additional") {
        return Some(format!(
            "remove unexpected properties from \"{path}\""
        ));
    }

    None
}
