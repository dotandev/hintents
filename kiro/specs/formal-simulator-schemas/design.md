# Design Document: Formal Simulator Schemas

## Overview

This design formalizes and expands the JSON Schema definitions for the Simulator's diagnostic output and input structures. The system will provide comprehensive, versioned schemas that external tooling can rely on to prevent breaking changes and enable robust integration.

The design addresses the current gap where the simulator has minimal schema coverage (only basic request/response structures) but returns rich diagnostic data including events, authentication traces, budget metrics, and stack traces. By formalizing these structures with JSON Schema, we enable:

- Type-safe client generation in multiple languages
- Automated validation of simulator responses
- Breaking change detection through semantic versioning
- Self-documenting API contracts

The schemas will be published at stable URLs with semantic versioning, allowing external tools to pin to specific versions while the simulator evolves.

## Architecture

### Schema Organization

The schema registry follows a modular architecture where complex types are defined in separate files and referenced via JSON Schema `$ref`:

```
docs/schema/
├── common.schema.json              # Shared type definitions (Version, XDRBase64)
├── simulation-request.schema.json  # Request structure (expanded)
├── simulation-response.schema.json # Response structure (expanded)
├── diagnostic-event.schema.json    # DiagnosticEvent type
├── auth-trace.schema.json          # AuthTrace and nested types
├── budget-usage.schema.json        # BudgetUsage metrics
├── wasm-stack-trace.schema.json    # WasmStackTrace and StackFrame
├── categorized-event.schema.json   # CategorizedEvent type
└── README.md                       # Documentation and examples
```

### Versioning Strategy

Each schema file includes:
- `$schema`: JSON Schema draft version (2020-12)
- `$id`: Stable URI for the schema (includes version)
- `version`: Semantic version string (e.g., "1.0.0")

Version increments follow semantic versioning:
- **Major**: Breaking changes (removing fields, changing types, tightening constraints)
- **Minor**: Backward-compatible additions (new optional fields, relaxing constraints)
- **Patch**: Documentation updates, clarifications

A `CHANGELOG.md` in `docs/schema/` tracks all version changes with migration notes.

### Cross-Reference Pattern

Schemas reference each other using relative paths:
```json
{
  "diagnostic_events": {
    "type": "array",
    "items": { "$ref": "diagnostic-event.schema.json" }
  }
}
```

This allows validators to resolve the full schema graph while keeping individual files focused and maintainable.

## Components and Interfaces

### Core Schema Files

#### 1. simulation-response.schema.json (Expanded)

Extends the existing minimal schema to include all diagnostic fields returned by the simulator. Key additions:

- `status`: Enum constrained to "success" | "error"
- `error`: String, required when status is "error"
- `events`: Array of strings (legacy format for backward compatibility)
- `diagnostic_events`: Array of DiagnosticEvent objects
- `logs`: Array of strings (host debug logs)
- `flamegraph`: String containing SVG content
- `auth_trace`: AuthTrace object
- `budget_usage`: BudgetUsage object
- `categorized_events`: Array of CategorizedEvent objects
- `protocol_version`: Optional unsigned integer
- `stack_trace`: Optional WasmStackTrace object
- `source_location`: Optional string
- `wasm_offset`: Optional unsigned integer

Conditional validation ensures `error` is required when `status` is "error" using JSON Schema `if/then` constructs.

#### 2. diagnostic-event.schema.json (New)

Defines the structure of diagnostic events emitted during contract execution:

```json
{
  "type": "object",
  "required": ["event_type", "topics", "data", "in_successful_contract_call"],
  "properties": {
    "event_type": {
      "type": "string",
      "enum": ["contract", "system", "diagnostic"]
    },
    "contract_id": { "type": "string" },
    "topics": {
      "type": "array",
      "items": { "type": "string" }
    },
    "data": { "type": "string" },
    "in_successful_contract_call": { "type": "boolean" },
    "wasm_instruction": { "type": "string" }
  }
}
```

#### 3. auth-trace.schema.json (New)

Defines authentication trace data with nested type definitions:

Main AuthTrace object:
- `success`: Boolean indicating overall auth success
- `account_id`: String identifier
- `signer_count`: Unsigned integer
- `valid_signatures`: Unsigned integer
- `signature_weights`: Array of KeyWeight objects
- `thresholds`: ThresholdConfig object
- `auth_events`: Array of AuthEvent objects
- `failures`: Array of AuthFailure objects
- `custom_contracts`: Optional array of CustomContractAuth objects

Nested types (defined in same file using `$defs`):
- **KeyWeight**: `{ public_key: string, weight: uint, type: SignatureType }`
- **SignatureType**: Enum of "ed25519" | "secp256k1" | "pre_authorized" | "custom_account"
- **ThresholdConfig**: `{ low_threshold: uint, medium_threshold: uint, high_threshold: uint }`
- **AuthEvent**: Event with timestamp, type, account, signer details, status
- **AuthFailure**: Detailed failure information with required/collected weights
- **CustomContractAuth**: Custom contract authentication result

#### 4. budget-usage.schema.json (New)

Defines resource consumption metrics:

```json
{
  "type": "object",
  "required": [
    "cpu_instructions", "memory_bytes", "operations_count",
    "cpu_limit", "memory_limit", 
    "cpu_usage_percent", "memory_usage_percent"
  ],
  "properties": {
    "cpu_instructions": { "type": "integer", "minimum": 0 },
    "memory_bytes": { "type": "integer", "minimum": 0 },
    "operations_count": { "type": "integer" },
    "cpu_limit": { "type": "integer", "minimum": 0 },
    "memory_limit": { "type": "integer", "minimum": 0 },
    "cpu_usage_percent": { "type": "number", "minimum": 0 },
    "memory_usage_percent": { "type": "number", "minimum": 0 }
  }
}
```

#### 5. wasm-stack-trace.schema.json (New)

Defines WASM stack trace structure captured on traps:

Main WasmStackTrace object:
- `trap_kind`: Any type (flexible to accommodate various trap representations)
- `raw_message`: String with original error message
- `frames`: Array of StackFrame objects
- `soroban_wrapped`: Boolean indicating if error passed through Soroban Host

StackFrame type (defined using `$defs`):
- `index`: Integer position in call stack (0 = trap site)
- `func_index`: Optional unsigned integer (WASM function index)
- `func_name`: Optional string (demangled function name)
- `wasm_offset`: Optional unsigned integer (byte offset in module)
- `module`: Optional string (module name from name section)

#### 6. simulation-request.schema.json (Expanded)

Expands the existing minimal schema to include all optional fields:

New fields:
- `envelope_xdr`: Required XDRBase64 string
- `result_meta_xdr`: Required XDRBase64 string
- `ledger_entries`: Optional object with string keys and XDRBase64 values
- `timestamp`: Optional integer
- `ledger_sequence`: Optional unsigned integer
- `wasm_path`: Optional string
- `mock_args`: Optional array of strings
- `profile`: Optional boolean
- `protocol_version`: Optional unsigned integer
- `mock_base_fee`: Optional unsigned integer
- `mock_gas_price`: Optional unsigned integer
- `auth_trace_opts`: Optional AuthTraceOptions object
- `custom_auth_config`: Optional object (flexible structure)
- `resource_calibration`: Optional ResourceCalibration object

Nested types:
- **AuthTraceOptions**: `{ enabled: bool, trace_custom_contracts: bool, capture_sig_details: bool, max_event_depth?: int }`
- **ResourceCalibration**: `{ sha256_fixed: uint, sha256_per_byte: uint, keccak256_fixed: uint, keccak256_per_byte: uint, ed25519_fixed: uint }`

#### 7. categorized-event.schema.json (New)

Defines the simplified event structure used in categorized_events:

```json
{
  "type": "object",
  "required": ["event_type", "topics", "data"],
  "properties": {
    "event_type": { "type": "string" },
    "contract_id": { "type": "string" },
    "topics": {
      "type": "array",
      "items": { "type": "string" }
    },
    "data": { "type": "string" }
  }
}
```

### Schema Publication

Schemas will be published at stable URLs following this pattern:
```
https://simulator.stellar.org/schemas/v1/simulation-response.schema.json
https://simulator.stellar.org/schemas/v1/diagnostic-event.schema.json
...
```

The version in the URL path (`v1`) represents the major version. Minor and patch updates maintain the same URL to allow automatic updates for non-breaking changes.

A schema catalog file (`catalog.json`) lists all available schemas with their versions and URLs:

```json
{
  "schemas": [
    {
      "name": "simulation-response",
      "version": "1.0.0",
      "url": "https://simulator.stellar.org/schemas/v1/simulation-response.schema.json",
      "description": "Complete simulator response structure"
    },
    ...
  ]
}
```

### Documentation

The `docs/schema/README.md` file provides:

1. **Overview**: Purpose and scope of the schema registry
2. **Versioning Guide**: How to interpret version numbers and handle updates
3. **Validation Examples**: Code samples in JavaScript, Python, Go, Rust
4. **Schema Relationships**: Diagram showing how schemas reference each other
5. **Migration Guide**: How to upgrade between major versions
6. **Contributing**: How to propose schema changes

Example validation code (JavaScript):
```javascript
const Ajv = require('ajv');
const ajv = new Ajv();

const schema = require('./simulation-response.schema.json');
const validate = ajv.compile(schema);

const response = { /* simulator output */ };
const valid = validate(response);

if (!valid) {
  console.error(validate.errors);
}
```

## Data Models

### Type Mapping: Go to JSON Schema

The schemas directly reflect the Go struct definitions in `internal/simulator/schema.go` and `internal/authtrace/types.go`:

| Go Type | JSON Schema Type | Constraints |
|---------|------------------|-------------|
| `string` | `"type": "string"` | - |
| `*string` | `"type": "string"` | Optional (not in required array) |
| `bool` | `"type": "boolean"` | - |
| `int` | `"type": "integer"` | - |
| `uint32`, `uint64` | `"type": "integer"` | `"minimum": 0` |
| `*uint32`, `*uint64` | `"type": "integer"` | `"minimum": 0`, optional |
| `float64` | `"type": "number"` | - |
| `[]string` | `"type": "array", "items": {"type": "string"}` | - |
| `map[string]string` | `"type": "object", "additionalProperties": {"type": "string"}` | - |
| `map[string]interface{}` | `"type": "object"` | No constraints (flexible) |
| `interface{}` | No type constraint | Accepts any JSON value |

### Schema Validation Rules

1. **Required Fields**: All non-pointer fields in Go structs are marked as required in schemas
2. **Optional Fields**: Pointer fields in Go are optional in schemas (not in required array)
3. **Enums**: String constants in Go become JSON Schema enums
4. **Nested Objects**: Go structs become nested object schemas or separate schema files with `$ref`
5. **Arrays**: Go slices become JSON arrays with typed items
6. **Maps**: Go maps become JSON objects with `additionalProperties`

### Backward Compatibility

The `events` field in SimulationResponse remains as an array of strings for backward compatibility with existing clients. New clients should use `diagnostic_events` for structured access.

The schemas use `"additionalProperties": false` to catch typos and unexpected fields, but this can be relaxed to `true` if forward compatibility is prioritized over strict validation.


## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Schema Completeness

*For any* Go struct type used in the simulator (SimulationRequest, SimulationResponse, DiagnosticEvent, AuthTrace, BudgetUsage, WasmStackTrace, etc.), when that struct is serialized to JSON, the corresponding JSON Schema SHALL successfully validate the output.

**Validates: Requirements 1.1, 6.1**

This property ensures that the schemas are complete and accurately reflect the Go type definitions. It can be tested by generating instances of each Go struct (with various field combinations), marshaling to JSON, and validating against the schema.

### Property 2: Conditional Error Field Requirement

*For any* SimulationResponse object, when the status field is "error", the schema validation SHALL require the error field to be present, and when the status field is "success", the schema validation SHALL allow the error field to be absent.

**Validates: Requirements 1.3**

This property tests the conditional validation logic using JSON Schema's `if/then` constructs. It ensures that error information is always present when needed and optional when not.

### Property 3: Schema Versioning Consistency

*For any* schema file in the Schema_Registry, the file SHALL contain a version field that follows semantic versioning format (MAJOR.MINOR.PATCH).

**Validates: Requirements 7.1**

This property ensures all schemas are properly versioned and can be tracked over time.

### Property 4: Cross-Reference Path Format

*For any* `$ref` value in any schema file that references another schema file (not an internal `$defs` reference), the reference SHALL use a relative file path format (e.g., "diagnostic-event.schema.json") rather than an absolute URL.

**Validates: Requirements 9.6**

This property ensures schemas can be validated locally without network access and remain portable across different deployment environments.

### Property 5: Stable Schema Identifiers

*For any* schema file in the Schema_Registry, the file SHALL contain a `$id` field with a stable URI that includes the schema name and major version.

**Validates: Requirements 10.1**

This property ensures schemas can be uniquely identified and referenced by external tools, with major version changes reflected in the URI.

### Property 6: Schema Validation Round-Trip

*For any* valid Go struct instance from the simulator codebase, serializing to JSON and then validating against the corresponding schema SHALL succeed, and any JSON that passes schema validation SHALL be deserializable to the Go struct without error.

**Validates: Requirements 1.1, 6.1**

This is a round-trip property that ensures bidirectional consistency between the Go types and JSON schemas. It catches cases where the schema is too strict (rejects valid Go output) or too loose (accepts invalid JSON that Go cannot parse).

## Error Handling

### Schema Validation Errors

When a JSON document fails schema validation, the error messages should clearly indicate:
- Which field(s) failed validation
- What constraint was violated (type mismatch, missing required field, enum violation, etc.)
- The path to the failing field in the JSON document

Most JSON Schema validators provide this information automatically through their error reporting mechanisms.

### Missing Schema Files

If a schema file references another schema via `$ref` and that file cannot be found:
- The validator should report a clear error indicating which schema is missing
- The error should include the `$ref` path that failed to resolve
- Documentation should guide users to ensure all schema files are present

### Version Mismatch

If a client attempts to validate against an incompatible schema version:
- The schema catalog should clearly indicate which versions are available
- Breaking changes should be documented in the CHANGELOG
- The README should provide migration guidance between major versions

### Invalid Schema Syntax

If a schema file itself is malformed (invalid JSON or invalid JSON Schema):
- The error should be caught during schema loading, not during validation
- CI/CD pipelines should validate all schema files against the JSON Schema meta-schema
- Schema files should be tested with multiple validators to ensure compatibility

## Testing Strategy

### Dual Testing Approach

This feature requires both unit tests and property-based tests:

**Unit Tests** focus on:
- Verifying specific schema field definitions (e.g., status field is enum with "success" and "error")
- Checking that required schema files exist
- Validating README content includes required sections
- Testing specific examples of valid and invalid JSON against schemas
- Verifying CHANGELOG format and content

**Property-Based Tests** focus on:
- Schema completeness: generating random Go struct instances and validating against schemas
- Round-trip validation: Go → JSON → Schema validation → JSON → Go
- Cross-reference integrity: all `$ref` paths resolve correctly
- Version field presence: all schemas have valid version strings
- $id field presence: all schemas have stable URIs

### Property-Based Testing Configuration

We will use:
- **Go**: Use `gopter` or `rapid` for property-based testing
- **JavaScript/TypeScript**: Use `fast-check` for testing schema validation
- **Minimum iterations**: 100 per property test

Each property test must include a comment tag referencing the design property:
```go
// Feature: formal-simulator-schemas, Property 1: Schema Completeness
func TestProperty_SchemaCompleteness(t *testing.T) { ... }
```

### Test Organization

```
tests/
├── schema/
│   ├── unit/
│   │   ├── field_definitions_test.go      # Unit tests for specific field checks
│   │   ├── file_existence_test.go         # Unit tests for required files
│   │   ├── documentation_test.go          # Unit tests for README/CHANGELOG
│   │   └── examples_test.go               # Unit tests for specific valid/invalid cases
│   └── property/
│       ├── completeness_test.go           # Property 1: Schema completeness
│       ├── conditional_validation_test.go # Property 2: Conditional error field
│       ├── versioning_test.go             # Property 3: Version consistency
│       ├── cross_reference_test.go        # Property 4: $ref path format
│       ├── identifiers_test.go            # Property 5: Stable $id URIs
│       └── roundtrip_test.go              # Property 6: Round-trip validation
```

### Integration Testing

Integration tests should verify:
- The simulator's actual output validates against the schemas
- Schema validation can be performed in multiple languages (Go, JavaScript, Python, Rust)
- The published schemas at stable URLs are accessible and valid
- The schema catalog correctly lists all available schemas

### Continuous Integration

CI pipelines should:
1. Validate all schema files against the JSON Schema meta-schema
2. Run all unit and property-based tests
3. Generate sample simulator output and validate against schemas
4. Check that CHANGELOG is updated when schemas change
5. Verify version numbers are incremented appropriately
6. Test schema validation in multiple languages

### Manual Testing

Manual review should verify:
- README examples are accurate and runnable
- Documentation is clear and complete
- Schema changes are backward-compatible (for minor/patch versions)
- Migration guides are helpful (for major versions)
