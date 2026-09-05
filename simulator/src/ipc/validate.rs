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

/// A structured validation error carrying context for the caller.
///
/// Replaces the raw string errors previously returned by `validate_request`:
/// each error names the offending field as a JSON Pointer, the 1-based line in
/// the raw input where the problem was found, and an optional suggestion for
/// how to fix it.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct ValidationError {
    /// JSON Pointer to the offending field (e.g. `/envelope_xdr`). Empty for
    /// whole-document errors such as invalid JSON syntax.
    pub path: String,
    /// 1-based line number in the raw input where the error occurs. `0` when
    /// the line cannot be determined.
    pub line: usize,
    /// Human-readable description of the problem.
    pub message: String,
    /// Optional, actionable hint for correcting the input.
    pub suggestion: Option<String>,
}

impl ValidationError {
    /// Build a validation error with the given JSON-pointer path.
    #[must_use]
    pub fn new(path: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            path: path.into(),
            line: 0,
            message: message.into(),
            suggestion: None,
        }
    }

    /// Attach a 1-based input line number.
    #[must_use]
    pub fn at_line(mut self, line: usize) -> Self {
        self.line = line;
        self
    }

    /// Attach an actionable suggestion.
    #[must_use]
    pub fn with_suggestion(mut self, suggestion: impl Into<String>) -> Self {
        self.suggestion = Some(suggestion.into());
        self
    }
}

/// Validates JSON input against the simulation-request.schema.json and returns
/// structured errors with line numbers, validation paths, and suggestions.
///
/// The schema's external `$ref`s (`common.schema.json`) are embedded at
/// compile time alongside the root schema and registered with the validator,
/// so validation works fully offline.
///
/// # Network calls
/// None — the schemas are compiled in via `include_str!`.
#[allow(dead_code)]
pub fn validate_request(input: &str) -> Result<Value, Vec<ValidationError>> {
    // include the schemas at compile-time
    let schema_json = include_str!("../../../docs/schema/simulation-request.schema.json");
    let common_schema_json = include_str!("../../../docs/schema/common.schema.json");

    let schema: Value = serde_json::from_str(schema_json).map_err(|e| {
        vec![ValidationError::new(
            "",
            format!("invalid schema JSON: {e}"),
        )]
    })?;
    let common_schema: Value = serde_json::from_str(common_schema_json).map_err(|e| {
        vec![ValidationError::new(
            "",
            format!("invalid schema JSON: {e}"),
        )]
    })?;

    // Resolve the relative `common.schema.json` refs against the root `$id`.
    let compiled = jsonschema::options()
        .with_resource(
            "https://simulator.stellar.org/schemas/v1/common.schema.json",
            jsonschema::Resource::from_contents(common_schema),
        )
        .build(&schema)
        .map_err(|e| {
            vec![ValidationError::new(
                "",
                format!("schema compilation failed: {e}"),
            )]
        })?;

    // Parse the incoming JSON; surface the syntax-error position (line/column).
    let instance: Value = match serde_json::from_str(input) {
        Ok(instance) => instance,
        Err(e) => {
            let line = e.line();
            let column = e.column();
            return Err(vec![ValidationError::new(
                "",
                format!("invalid JSON at line {line}, column {column}: {e}"),
            )
            .at_line(line)
            .with_suggestion("fix the JSON syntax near the reported line")]);
        }
    };

    // Validate against the schema, mapping each failure to a structured error.
    let errors: Vec<ValidationError> = compiled
        .iter_errors(&instance)
        .map(|error| build_validation_error(input, &error))
        .collect();
    if !errors.is_empty() {
        return Err(errors);
    }

    Ok(instance)
}

/// Build a structured error from a single schema-validation error.
fn build_validation_error(input: &str, error: &jsonschema::ValidationError<'_>) -> ValidationError {
    let instance_path = error.instance_path().as_str();
    let kind = error.kind();

    // Some error kinds name the offending field themselves while the instance
    // path points at the parent object — derive a precise path and suggestion.
    let field = match kind {
        jsonschema::error::ValidationErrorKind::Required { property } => {
            property.as_str().map(str::to_string)
        }
        jsonschema::error::ValidationErrorKind::AdditionalProperties { unexpected }
        | jsonschema::error::ValidationErrorKind::UnevaluatedProperties { unexpected } => {
            unexpected.first().cloned()
        }
        _ => None,
    };

    let path = match &field {
        Some(field) if !field.is_empty() => {
            format!("{instance_path}/{}", escape_pointer_segment(field))
        }
        _ => instance_path.to_string(),
    };

    let line = field
        .as_deref()
        .and_then(|field| non_zero_line(line_of_key(input, field)))
        .or_else(|| non_zero_line(line_of_pointer(input, &path)))
        .or_else(|| non_zero_line(first_content_line(input)))
        .unwrap_or(0);

    let suggestion = field.map(|field| match kind {
        jsonschema::error::ValidationErrorKind::Required { .. } => {
            format!("add the required property {field:?} to the request")
        }
        _ => format!("remove the unexpected property {field:?} from the request"),
    });

    let mut validation_error = ValidationError::new(path, error.to_string()).at_line(line);
    if let Some(suggestion) = suggestion {
        validation_error = validation_error.with_suggestion(suggestion);
    } else {
        validation_error = validation_error.with_suggestion(default_suggestion(instance_path));
    }
    validation_error
}

/// Return the 1-based line of the first line that contains `key` as a JSON
/// object key, or 0 when it cannot be found.
fn line_of_key(input: &str, key: &str) -> usize {
    let needle = format!("\"{key}\"");
    input
        .lines()
        .position(|line| line.contains(&needle))
        .map_or(0, |index| index + 1)
}

/// Resolve a JSON Pointer to the first object key it names and return its line.
///
/// Walks the pointer from the leaf upward, skipping array indices, and returns
/// the first line that contains a matching key.
fn line_of_pointer(input: &str, pointer: &str) -> usize {
    for segment in pointer.split('/').rev() {
        let key = segment.replace("~1", "/").replace("~0", "~");
        if key.is_empty() || key.chars().all(|c| c.is_ascii_digit()) {
            continue;
        }
        let line = line_of_key(input, &key);
        if line > 0 {
            return line;
        }
    }
    0
}

/// Return the 1-based line of the first non-empty line in the input.
fn first_content_line(input: &str) -> usize {
    input
        .lines()
        .position(|line| !line.trim().is_empty())
        .map_or(0, |index| index + 1)
}

/// Escape a string for use as a JSON Pointer reference token.
fn escape_pointer_segment(segment: &str) -> String {
    segment.replace('~', "~0").replace('/', "~1")
}

/// A generic, actionable hint for a field-level validation failure.
fn default_suggestion(path: &str) -> String {
    if path.is_empty() {
        "review the request body against the simulation-request schema".to_string()
    } else {
        format!("review the field at {path} against the simulation-request schema")
    }
}

/// Convert a valid 1-based line (> 0) to `Some(line)`, else `None`.
fn non_zero_line(line: usize) -> Option<usize> {
    (line > 0).then_some(line)
}

#[cfg(test)]
mod tests {
    use super::*;

    const VALID_REQUEST: &str = r#"{
  "version": "1.0.0",
  "request_id": "req-1",
  "network": "testnet",
  "xdr": "AAAAAA==",
  "envelope_xdr": "AAAAAA==",
  "result_meta_xdr": "AAAAAA=="
}"#;

    #[test]
    fn valid_request_passes() {
        let result = validate_request(VALID_REQUEST);
        let instance = result.expect("valid request should pass");
        assert_eq!(instance["network"], "testnet");
    }

    #[test]
    fn missing_required_field_reports_path_line_and_suggestion() {
        let input = r#"{
  "version": "1.0.0",
  "request_id": "req-2",
  "network": "testnet",
  "xdr": "AAAAAA==",
  "result_meta_xdr": "AAAAAA=="
}"#;
        let errors = validate_request(input).expect_err("missing field should fail");
        assert_eq!(errors.len(), 1);

        let err = &errors[0];
        assert_eq!(err.path, "/envelope_xdr");
        assert!(err.line > 0, "expected a line number, got {}", err.line);
        assert!(err.message.contains("envelope_xdr"));
        assert!(err
            .suggestion
            .as_deref()
            .is_some_and(|s| s.contains("add the required property")));
    }

    #[test]
    fn invalid_json_reports_line_and_suggestion() {
        let input = "{\n  \"version\": \"1.0.0\",\n  oops\n}";
        let errors = validate_request(input).expect_err("bad JSON should fail");
        assert_eq!(errors.len(), 1);

        let err = &errors[0];
        assert_eq!(err.path, "");
        assert_eq!(err.line, 3, "syntax error should point at line 3");
        assert!(err.message.contains("invalid JSON"));
        assert!(err
            .suggestion
            .as_deref()
            .is_some_and(|s| s.contains("fix the JSON syntax")));
    }

    #[test]
    fn additional_property_rejected_with_path() {
        let input = r#"{
  "version": "1.0.0",
  "request_id": "req-3",
  "network": "testnet",
  "xdr": "AAAAAA==",
  "envelope_xdr": "AAAAAA==",
  "result_meta_xdr": "AAAAAA==",
  "bogus_extra": true
}"#;
        let errors = validate_request(input).expect_err("extra property should fail");
        assert!(!errors.is_empty());

        let extra = errors.iter().find(|e| e.path == "/bogus_extra");
        assert!(
            extra.is_some(),
            "expected an error on /bogus_extra: {errors:?}"
        );
        let extra = extra.unwrap();
        assert_eq!(extra.line, 8);
        assert!(extra
            .suggestion
            .as_deref()
            .is_some_and(|s| s.contains("remove the unexpected property")));
    }

    #[test]
    fn structured_error_serializes_to_json() {
        let err = ValidationError::new("/envelope_xdr", "envelope_xdr is a required property")
            .at_line(6)
            .with_suggestion("add the required property \"envelope_xdr\" to the request");
        let value = serde_json::to_value(&err).unwrap();
        assert_eq!(value["path"], "/envelope_xdr");
        assert_eq!(value["line"], 6);
        assert_eq!(value["message"], "envelope_xdr is a required property");
        assert_eq!(
            value["suggestion"],
            "add the required property \"envelope_xdr\" to the request"
        );
    }

    #[test]
    fn empty_input_fails_as_invalid_json() {
        let errors = validate_request("").expect_err("empty input should fail");
        assert!(!errors.is_empty());
    }
}
