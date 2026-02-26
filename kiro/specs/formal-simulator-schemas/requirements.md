# Requirements Document

## Introduction

This feature formalizes and expands the JSON Schema definitions for the Simulator's diagnostic output, specifically the SimulationResponse structure. The goal is to provide comprehensive, versioned schemas that external tooling can rely on to prevent breaking changes and enable robust integration with the simulator.

## Glossary

- **Simulator**: The Stellar smart contract simulation engine that executes transactions and returns diagnostic output
- **SimulationResponse**: The JSON object returned by the Simulator containing execution results, events, diagnostics, and metadata
- **SimulationRequest**: The JSON object sent to the Simulator to initiate a simulation
- **Schema_Registry**: The collection of JSON Schema files in docs/schema/ that define the structure of simulator inputs and outputs
- **DiagnosticEvent**: A structured event emitted during simulation containing contract execution details
- **AuthTrace**: Authentication trace data capturing signature verification and authorization flow
- **BudgetUsage**: Resource consumption metrics including CPU instructions and memory usage
- **WasmStackTrace**: Structured call stack information captured when a WASM trap occurs
- **External_Tooling**: Third-party applications, libraries, or services that consume simulator output

## Requirements

### Requirement 1: Expand SimulationResponse Schema

**User Story:** As a developer building tooling on top of the simulator, I want a comprehensive JSON Schema for SimulationResponse, so that I can validate responses and detect breaking changes early.

#### Acceptance Criteria

1. THE Schema_Registry SHALL include a complete JSON Schema definition for all SimulationResponse fields
2. THE SimulationResponse_Schema SHALL define the status field as an enum with values "success" and "error"
3. THE SimulationResponse_Schema SHALL define the error field as a string that is required when success is false
4. THE SimulationResponse_Schema SHALL define the events field as an array of strings for backward compatibility
5. THE SimulationResponse_Schema SHALL define the diagnostic_events field as an array of DiagnosticEvent objects
6. THE SimulationResponse_Schema SHALL define the logs field as an array of strings
7. THE SimulationResponse_Schema SHALL define the flamegraph field as a string containing SVG content
8. THE SimulationResponse_Schema SHALL define the auth_trace field as an AuthTrace object
9. THE SimulationResponse_Schema SHALL define the budget_usage field as a BudgetUsage object
10. THE SimulationResponse_Schema SHALL define the categorized_events field as an array of CategorizedEvent objects
11. THE SimulationResponse_Schema SHALL define the protocol_version field as an optional unsigned integer
12. THE SimulationResponse_Schema SHALL define the stack_trace field as an optional WasmStackTrace object
13. THE SimulationResponse_Schema SHALL define the source_location field as an optional string
14. THE SimulationResponse_Schema SHALL define the wasm_offset field as an optional unsigned integer

### Requirement 2: Define DiagnosticEvent Schema

**User Story:** As a developer analyzing contract execution, I want a formal schema for DiagnosticEvent, so that I can reliably parse structured diagnostic information.

#### Acceptance Criteria

1. THE Schema_Registry SHALL include a JSON Schema definition for DiagnosticEvent
2. THE DiagnosticEvent_Schema SHALL define event_type as a required enum with values "contract", "system", and "diagnostic"
3. THE DiagnosticEvent_Schema SHALL define contract_id as an optional string
4. THE DiagnosticEvent_Schema SHALL define topics as a required array of strings
5. THE DiagnosticEvent_Schema SHALL define data as a required string
6. THE DiagnosticEvent_Schema SHALL define in_successful_contract_call as a required boolean
7. THE DiagnosticEvent_Schema SHALL define wasm_instruction as an optional string

### Requirement 3: Define AuthTrace Schema

**User Story:** As a developer debugging authentication issues, I want a formal schema for AuthTrace, so that I can programmatically analyze signature verification flows.

#### Acceptance Criteria

1. THE Schema_Registry SHALL include a JSON Schema definition for AuthTrace
2. THE AuthTrace_Schema SHALL define success as a required boolean
3. THE AuthTrace_Schema SHALL define account_id as a required string
4. THE AuthTrace_Schema SHALL define signer_count as a required unsigned integer
5. THE AuthTrace_Schema SHALL define valid_signatures as a required unsigned integer
6. THE AuthTrace_Schema SHALL define signature_weights as a required array of KeyWeight objects
7. THE AuthTrace_Schema SHALL define thresholds as a required ThresholdConfig object
8. THE AuthTrace_Schema SHALL define auth_events as a required array of AuthEvent objects
9. THE AuthTrace_Schema SHALL define failures as a required array of AuthFailure objects
10. THE AuthTrace_Schema SHALL define custom_contracts as an optional array of CustomContractAuth objects

### Requirement 4: Define BudgetUsage Schema

**User Story:** As a developer optimizing contract performance, I want a formal schema for BudgetUsage, so that I can track resource consumption metrics consistently.

#### Acceptance Criteria

1. THE Schema_Registry SHALL include a JSON Schema definition for BudgetUsage
2. THE BudgetUsage_Schema SHALL define cpu_instructions as a required unsigned integer
3. THE BudgetUsage_Schema SHALL define memory_bytes as a required unsigned integer
4. THE BudgetUsage_Schema SHALL define operations_count as a required integer
5. THE BudgetUsage_Schema SHALL define cpu_limit as a required unsigned integer
6. THE BudgetUsage_Schema SHALL define memory_limit as a required unsigned integer
7. THE BudgetUsage_Schema SHALL define cpu_usage_percent as a required number
8. THE BudgetUsage_Schema SHALL define memory_usage_percent as a required number

### Requirement 5: Define WasmStackTrace Schema

**User Story:** As a developer debugging contract traps, I want a formal schema for WasmStackTrace, so that I can parse stack traces programmatically.

#### Acceptance Criteria

1. THE Schema_Registry SHALL include a JSON Schema definition for WasmStackTrace
2. THE WasmStackTrace_Schema SHALL define trap_kind as a required field accepting any type
3. THE WasmStackTrace_Schema SHALL define raw_message as a required string
4. THE WasmStackTrace_Schema SHALL define frames as a required array of StackFrame objects
5. THE WasmStackTrace_Schema SHALL define soroban_wrapped as a required boolean
6. THE Schema_Registry SHALL include a JSON Schema definition for StackFrame
7. THE StackFrame_Schema SHALL define index as a required integer
8. THE StackFrame_Schema SHALL define func_index as an optional unsigned integer
9. THE StackFrame_Schema SHALL define func_name as an optional string
10. THE StackFrame_Schema SHALL define wasm_offset as an optional unsigned integer
11. THE StackFrame_Schema SHALL define module as an optional string

### Requirement 6: Expand SimulationRequest Schema

**User Story:** As a developer building simulator clients, I want a comprehensive JSON Schema for SimulationRequest, so that I can validate requests before sending them.

#### Acceptance Criteria

1. THE Schema_Registry SHALL expand the existing SimulationRequest schema to include all optional fields
2. THE SimulationRequest_Schema SHALL define envelope_xdr as a required XDRBase64 string
3. THE SimulationRequest_Schema SHALL define result_meta_xdr as a required XDRBase64 string
4. THE SimulationRequest_Schema SHALL define ledger_entries as an optional object with string keys and XDRBase64 values
5. THE SimulationRequest_Schema SHALL define timestamp as an optional integer
6. THE SimulationRequest_Schema SHALL define ledger_sequence as an optional unsigned integer
7. THE SimulationRequest_Schema SHALL define wasm_path as an optional string
8. THE SimulationRequest_Schema SHALL define mock_args as an optional array of strings
9. THE SimulationRequest_Schema SHALL define profile as an optional boolean
10. THE SimulationRequest_Schema SHALL define protocol_version as an optional unsigned integer
11. THE SimulationRequest_Schema SHALL define mock_base_fee as an optional unsigned integer
12. THE SimulationRequest_Schema SHALL define mock_gas_price as an optional unsigned integer
13. THE SimulationRequest_Schema SHALL define auth_trace_opts as an optional AuthTraceOptions object
14. THE SimulationRequest_Schema SHALL define custom_auth_config as an optional object
15. THE SimulationRequest_Schema SHALL define resource_calibration as an optional ResourceCalibration object

### Requirement 7: Schema Versioning

**User Story:** As a developer maintaining External_Tooling, I want schema versions to be tracked, so that I can detect when schemas change and update my code accordingly.

#### Acceptance Criteria

1. THE Schema_Registry SHALL include a version field in each schema file
2. WHEN a schema is modified in a backward-compatible way, THE Schema_Registry SHALL increment the minor version number
3. WHEN a schema is modified in a breaking way, THE Schema_Registry SHALL increment the major version number
4. THE Schema_Registry SHALL maintain a changelog documenting version changes

### Requirement 8: Schema Validation Documentation

**User Story:** As a developer integrating with the simulator, I want documentation on how to use the schemas, so that I can implement validation in my tooling.

#### Acceptance Criteria

1. THE Schema_Registry SHALL include a README file explaining the purpose of each schema
2. THE Schema_README SHALL provide examples of validating SimulationResponse using the schemas
3. THE Schema_README SHALL provide examples of validating SimulationRequest using the schemas
4. THE Schema_README SHALL document the schema versioning strategy
5. THE Schema_README SHALL list all available schemas and their relationships

### Requirement 9: Schema Cross-References

**User Story:** As a developer working with the schemas, I want schemas to reference each other properly, so that I can validate complex nested structures.

#### Acceptance Criteria

1. THE SimulationResponse_Schema SHALL reference DiagnosticEvent_Schema using JSON Schema $ref
2. THE SimulationResponse_Schema SHALL reference AuthTrace_Schema using JSON Schema $ref
3. THE SimulationResponse_Schema SHALL reference BudgetUsage_Schema using JSON Schema $ref
4. THE SimulationResponse_Schema SHALL reference WasmStackTrace_Schema using JSON Schema $ref
5. THE AuthTrace_Schema SHALL reference its nested type schemas using JSON Schema $ref
6. THE Schema_Registry SHALL use relative $ref paths for cross-schema references

### Requirement 10: Schema Publication

**User Story:** As a developer building External_Tooling, I want schemas to be published at stable URLs, so that I can reference them in my validation code.

#### Acceptance Criteria

1. THE Schema_Registry SHALL use stable $id URIs for each schema file
2. THE Schema_README SHALL document the canonical URLs where schemas are published
3. WHEN schemas are updated, THE Schema_Registry SHALL maintain backward-compatible URLs for previous versions
4. THE Schema_Registry SHALL provide a schema catalog listing all available schemas and versions
