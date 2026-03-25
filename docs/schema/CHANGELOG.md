# Changelog

All notable changes to the Simulator JSON Schemas will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-01-15

### Added

#### Core Response Schemas
- **simulation-response.schema.json** - Complete schema for simulator response structure
  - Status field with "success" and "error" enum values
  - Conditional error field requirement based on status
  - Diagnostic events, logs, flamegraph, auth trace, budget usage
  - Protocol version, stack trace, source location, WASM offset fields
  - Cross-references to nested type schemas

#### Core Request Schemas
- **simulation-request.schema.json** - Comprehensive schema for simulator request structure
  - Required XDR fields: envelope_xdr, result_meta_xdr
  - Optional configuration: ledger_entries, timestamp, ledger_sequence, wasm_path
  - Mock configuration: mock_args, mock_base_fee, mock_gas_price
  - Profiling and protocol version options
  - Authentication trace options with nested AuthTraceOptions type
  - Resource calibration configuration with nested ResourceCalibration type
  - Custom authentication configuration support

#### Diagnostic and Event Schemas
- **diagnostic-event.schema.json** - Schema for structured diagnostic events
  - Event type enum: "contract", "system", "diagnostic"
  - Contract ID, topics array, data field
  - Success context flag and WASM instruction tracking

- **categorized-event.schema.json** - Schema for simplified categorized events
  - Event type, contract ID, topics, and data fields
  - Used in categorized_events array in response

#### Authentication and Resource Schemas
- **auth-trace.schema.json** - Schema for authentication trace data
  - Overall success status and account information
  - Signer count and signature validation metrics
  - Nested types: KeyWeight, SignatureType, ThresholdConfig
  - Authentication events and failure tracking
  - Custom contract authentication support

- **budget-usage.schema.json** - Schema for resource consumption metrics
  - CPU instructions and memory bytes tracking
  - Operations count and resource limits
  - Usage percentages for CPU and memory

#### Stack Trace Schemas
- **wasm-stack-trace.schema.json** - Schema for WASM stack traces on traps
  - Trap kind and raw error message
  - Stack frames array with nested StackFrame type
  - Soroban wrapping indicator
  - Frame details: function index/name, WASM offset, module name

#### Common Definitions
- **common.schema.json** - Shared type definitions
  - Version type for semantic versioning
  - XDRBase64 type for XDR-encoded data

### Schema Features
- All schemas use JSON Schema Draft 2020-12
- Stable $id URIs following pattern: `https://simulator.stellar.org/schemas/v1/{schema-name}.schema.json`
- Semantic versioning with version field in each schema
- Relative $ref paths for cross-schema references
- Comprehensive validation constraints (enums, minimums, required fields)

### Documentation
- README.md with validation examples in multiple languages
- Schema relationships and versioning guide
- Migration guidance for future versions

---

## Template for Future Releases

## [Unreleased]

### Added
- New schemas or fields added

### Changed
- Modifications to existing schemas (backward-compatible for minor versions)

### Deprecated
- Features marked for removal in future major versions

### Removed
- Features removed (breaking changes, major version only)

### Fixed
- Bug fixes in schema definitions

### Security
- Security-related schema changes

---

## Version Guidelines

### Major Version (X.0.0)
Increment when making breaking changes:
- Removing required fields
- Changing field types
- Tightening validation constraints
- Removing enum values
- Changing $ref targets

### Minor Version (0.X.0)
Increment when adding backward-compatible features:
- Adding new optional fields
- Adding new enum values
- Relaxing validation constraints
- Adding new schemas

### Patch Version (0.0.X)
Increment for non-functional changes:
- Documentation updates
- Clarifying descriptions
- Fixing typos
- Adding examples

---

[1.0.0]: https://github.com/stellar/simulator/releases/tag/schemas-v1.0.0
