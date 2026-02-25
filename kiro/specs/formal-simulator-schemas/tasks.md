# Implementation Plan: Formal Simulator Schemas

## Overview

This implementation plan creates comprehensive JSON Schema definitions for the Simulator's diagnostic output and input structures. The work involves expanding existing minimal schemas, creating new schema files for complex types, implementing versioning with stable URIs, creating documentation, and implementing property-based tests to ensure schema correctness.

## Tasks

- [x] 1. Set up schema directory structure and common definitions
  - Create docs/schema/ directory if it doesn't exist
  - Create common.schema.json with shared type definitions (Version, XDRBase64)
  - Set up schema file templates with $schema, $id, and version fields
  - _Requirements: 7.1, 10.1_

- [ ] 2. Create core nested type schemas
  - [x] 2.1 Create diagnostic-event.schema.json
    - Define DiagnosticEvent object with event_type enum, topics, data, contract_id, in_successful_contract_call, wasm_instruction
    - Add $id URI: https://simulator.stellar.org/schemas/v1/diagnostic-event.schema.json
    - Set version to "1.0.0"
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7_
  
  - [x] 2.2 Create categorized-event.schema.json
    - Define CategorizedEvent object with event_type, contract_id, topics, data
    - Add $id URI and version "1.0.0"
    - _Requirements: 1.10_
  
  - [x] 2.3 Create budget-usage.schema.json
    - Define BudgetUsage object with cpu_instructions, memory_bytes, operations_count, limits, and usage percentages
    - Add minimum: 0 constraints for unsigned integers
    - Add $id URI and version "1.0.0"
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8_

- [ ] 3. Create authentication and stack trace schemas
  - [x] 3.1 Create auth-trace.schema.json
    - Define AuthTrace object with success, account_id, signer_count, valid_signatures, signature_weights, thresholds, auth_events, failures, custom_contracts
    - Define nested types in $defs: KeyWeight, SignatureType, ThresholdConfig, AuthEvent, AuthFailure, CustomContractAuth
    - Add $id URI and version "1.0.0"
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9, 3.10_
  
  - [x] 3.2 Create wasm-stack-trace.schema.json
    - Define WasmStackTrace object with trap_kind, raw_message, frames, soroban_wrapped
    - Define StackFrame type in $defs with index, func_index, func_name, wasm_offset, module
    - Add $id URI and version "1.0.0"
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8, 5.9, 5.10, 5.11_

- [x] 4. Expand simulation-response.schema.json
  - Add status field as enum with "success" and "error"
  - Add error field with conditional requirement (required when status is "error")
  - Add events field as array of strings
  - Add diagnostic_events field referencing diagnostic-event.schema.json
  - Add logs field as array of strings
  - Add flamegraph field as string
  - Add auth_trace field referencing auth-trace.schema.json
  - Add budget_usage field referencing budget-usage.schema.json
  - Add categorized_events field referencing categorized-event.schema.json
  - Add protocol_version, stack_trace, source_location, wasm_offset fields
  - Implement conditional validation using if/then for error field
  - Update $id URI to include v1 and set version to "1.0.0"
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 1.10, 1.11, 1.12, 1.13, 1.14, 9.1, 9.2, 9.3, 9.4_

- [ ] 5. Expand simulation-request.schema.json
  - [x] 5.1 Add required fields: envelope_xdr, result_meta_xdr
    - Define XDRBase64 type constraint (string format)
    - _Requirements: 6.2, 6.3_
  
  - [x] 5.2 Add optional configuration fields
    - Add ledger_entries, timestamp, ledger_sequence, wasm_path, mock_args, profile, protocol_version, mock_base_fee, mock_gas_price
    - _Requirements: 6.4, 6.5, 6.6, 6.7, 6.8, 6.9, 6.10, 6.11, 6.12_
  
  - [x] 5.3 Add nested type definitions in $defs
    - Define AuthTraceOptions with enabled, trace_custom_contracts, capture_sig_details, max_event_depth
    - Define ResourceCalibration with sha256_fixed, sha256_per_byte, keccak256_fixed, keccak256_per_byte, ed25519_fixed
    - Add auth_trace_opts and resource_calibration fields referencing these types
    - Add custom_auth_config as flexible object
    - Update $id URI and set version to "1.0.0"
    - _Requirements: 6.13, 6.14, 6.15_

- [x] 6. Checkpoint - Validate all schema files
  - Ensure all schema files are valid JSON
  - Ensure all $ref paths use relative file paths
  - Ensure all schemas have $id, version, and $schema fields
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 7. Create schema documentation
  - [x] 7.1 Create docs/schema/README.md
    - Add overview section explaining purpose and scope
    - Add versioning guide explaining semantic versioning strategy
    - Add validation examples in JavaScript, Python, Go, Rust
    - Add schema relationships diagram
    - Add migration guide for major version upgrades
    - Document canonical URLs for published schemas
    - List all available schemas and their relationships
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 10.2_
  
  - [x] 7.2 Create docs/schema/CHANGELOG.md
    - Add initial entry for version 1.0.0
    - Document all schemas created in this release
    - Include template for future version entries
    - _Requirements: 7.4_
  
  - [x] 7.3 Create docs/schema/catalog.json
    - List all schemas with name, version, URL, and description
    - Include stable URLs following pattern: https://simulator.stellar.org/schemas/v1/{schema-name}.schema.json
    - _Requirements: 10.4_

- [ ] 8. Implement property-based tests for schema validation
  - [ ]* 8.1 Write property test for schema completeness
    - **Property 1: Schema Completeness**
    - **Validates: Requirements 1.1, 6.1**
    - Generate random Go struct instances for all simulator types
    - Marshal to JSON and validate against corresponding schemas
    - Use gopter or rapid with minimum 100 iterations
  
  - [ ]* 8.2 Write property test for conditional error field requirement
    - **Property 2: Conditional Error Field Requirement**
    - **Validates: Requirements 1.3**
    - Generate SimulationResponse instances with status "success" and "error"
    - Validate that error field is required when status is "error"
    - Validate that error field is optional when status is "success"
  
  - [ ]* 8.3 Write property test for schema versioning consistency
    - **Property 3: Schema Versioning Consistency**
    - **Validates: Requirements 7.1**
    - Read all schema files in docs/schema/
    - Validate each contains version field matching semantic versioning format (MAJOR.MINOR.PATCH)
  
  - [ ]* 8.4 Write property test for cross-reference path format
    - **Property 4: Cross-Reference Path Format**
    - **Validates: Requirements 9.6**
    - Parse all schema files and extract $ref values
    - Validate external references use relative file paths, not absolute URLs
    - Validate all referenced files exist
  
  - [ ]* 8.5 Write property test for stable schema identifiers
    - **Property 5: Stable Schema Identifiers**
    - **Validates: Requirements 10.1**
    - Read all schema files
    - Validate each contains $id field with stable URI including schema name and major version
  
  - [ ]* 8.6 Write property test for schema validation round-trip
    - **Property 6: Schema Validation Round-Trip**
    - **Validates: Requirements 1.1, 6.1**
    - Generate Go struct instances, marshal to JSON, validate against schema
    - Generate valid JSON from schema, unmarshal to Go struct
    - Verify bidirectional consistency

- [ ] 9. Implement unit tests for specific schema validations
  - [ ]* 9.1 Write unit tests for field definitions
    - Test status field is enum with "success" and "error"
    - Test event_type field is enum with "contract", "system", "diagnostic"
    - Test required fields are marked as required in schemas
    - Test unsigned integers have minimum: 0 constraint
    - _Requirements: 1.2, 2.2, 4.2, 4.3, 4.4, 4.5_
  
  - [ ]* 9.2 Write unit tests for file existence
    - Test all required schema files exist in docs/schema/
    - Test README.md, CHANGELOG.md, and catalog.json exist
    - _Requirements: 8.1, 7.4, 10.4_
  
  - [ ]* 9.3 Write unit tests for documentation content
    - Test README includes validation examples
    - Test README documents versioning strategy
    - Test README lists all schemas
    - Test CHANGELOG has proper format
    - _Requirements: 8.2, 8.3, 8.4, 8.5_
  
  - [ ]* 9.4 Write unit tests for specific valid/invalid JSON examples
    - Test valid SimulationResponse with status "success" passes validation
    - Test SimulationResponse with status "error" but no error field fails validation
    - Test DiagnosticEvent with invalid event_type fails validation
    - Test BudgetUsage with negative cpu_instructions fails validation
    - _Requirements: 1.2, 1.3, 2.2, 4.2_

- [x] 10. Final checkpoint - Ensure all tests pass
  - Run all property-based tests and verify they pass
  - Run all unit tests and verify they pass
  - Validate all schemas against JSON Schema meta-schema
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- All schemas use JSON Schema Draft 2020-12
- Cross-references use relative paths for portability
- Versioning follows semantic versioning (MAJOR.MINOR.PATCH)
