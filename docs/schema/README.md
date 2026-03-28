# Stellar Simulator Schema Registry

## Overview

This directory contains formal JSON Schema definitions for the Stellar smart contract simulator's input and output structures. These schemas provide comprehensive, versioned specifications that enable:

- **Type-safe client generation** in multiple programming languages
- **Automated validation** of simulator requests and responses
- **Breaking change detection** through semantic versioning
- **Self-documenting API contracts** for external tooling

The schemas formalize the simulator's diagnostic output including execution results, events, authentication traces, resource metrics, and stack traces. By providing machine-readable specifications, we enable robust integration with external tools while preventing breaking changes.

## Purpose and Scope

The schema registry covers:

- **SimulationRequest**: Complete request structure with all optional configuration fields
- **SimulationResponse**: Full response structure including diagnostic data
- **DiagnosticEvent**: Structured events emitted during contract execution
- **AuthTrace**: Authentication and signature verification flow data
- **BudgetUsage**: Resource consumption metrics (CPU, memory, operations)
- **WasmStackTrace**: Stack trace information captured on WASM traps
- **CategorizedEvent**: Simplified event structure for categorized output
- **Common**: Shared type definitions (Version, XDRBase64)

These schemas directly reflect the Go struct definitions in the simulator codebase, ensuring consistency between implementation and specification.

## Available Schemas

### Core Schemas

| Schema | Description | Version |
|--------|-------------|---------|
| `simulation-request.schema.json` | Complete simulator request structure with envelope XDR, ledger state, and configuration options | 1.0.0 |
| `simulation-response.schema.json` | Full simulator response including status, results, events, diagnostics, and metadata | 1.0.0 |

### Diagnostic Schemas

| Schema | Description | Version |
|--------|-------------|---------|
| `diagnostic-event.schema.json` | Structured events emitted during execution with type, topics, data, and context | 1.0.0 |
| `categorized-event.schema.json` | Simplified event structure used in categorized_events array | 1.0.0 |
| `auth-trace.schema.json` | Authentication trace capturing signature verification, weights, thresholds, and failures | 1.0.0 |
| `budget-usage.schema.json` | Resource consumption metrics including CPU instructions, memory bytes, and usage percentages | 1.0.0 |
| `wasm-stack-trace.schema.json` | Stack trace information with trap kind, frames, function names, and WASM offsets | 1.0.0 |

### Shared Schemas

| Schema | Description | Version |
|--------|-------------|---------|
| `common.schema.json` | Shared type definitions including Version (semantic version string) and XDRBase64 (base64-encoded XDR) | 1.0.0 |

## Schema Relationships

The schemas form a dependency graph where complex types reference simpler types:

```
simulation-response.schema.json
├── common.schema.json (Version)
├── diagnostic-event.schema.json
├── categorized-event.schema.json
├── auth-trace.schema.json
│   └── (nested types via $defs)
├── budget-usage.schema.json
└── wasm-stack-trace.schema.json
    └── (StackFrame via $defs)

simulation-request.schema.json
├── common.schema.json (XDRBase64)
└── (nested types via $defs)
```

Schemas use relative `$ref` paths (e.g., `"$ref": "diagnostic-event.schema.json"`) to reference each other, allowing validators to resolve the full schema graph while keeping individual files focused and maintainable.

## Versioning Strategy

### Semantic Versioning

All schemas follow [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR version** (X.0.0): Breaking changes
  - Removing fields
  - Changing field types
  - Tightening constraints (e.g., making optional field required)
  - Changing enum values
  
- **MINOR version** (1.X.0): Backward-compatible additions
  - Adding new optional fields
  - Relaxing constraints (e.g., making required field optional)
  - Adding new enum values
  
- **PATCH version** (1.0.X): Non-functional changes
  - Documentation updates
  - Clarifications in descriptions
  - Fixing typos

### Version Fields

Each schema file includes:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://simulator.stellar.org/schemas/v1/simulation-response.schema.json",
  "version": "1.0.0",
  ...
}
```

- `$schema`: JSON Schema draft version (2020-12)
- `$id`: Stable URI including major version in path (`/v1/`)
- `version`: Semantic version string for tracking minor/patch updates

### Version Management

- The `$id` URL path includes the major version (`/v1/`, `/v2/`, etc.)
- Minor and patch updates maintain the same `$id` URL
- Breaking changes (major version bumps) result in new `$id` URLs
- Previous major versions remain available at their original URLs

## Canonical URLs

Schemas are published at stable URLs following this pattern:

```
https://simulator.stellar.org/schemas/v1/{schema-name}.schema.json
```

### Published Schema URLs

- `https://simulator.stellar.org/schemas/v1/simulation-request.schema.json`
- `https://simulator.stellar.org/schemas/v1/simulation-response.schema.json`
- `https://simulator.stellar.org/schemas/v1/diagnostic-event.schema.json`
- `https://simulator.stellar.org/schemas/v1/categorized-event.schema.json`
- `https://simulator.stellar.org/schemas/v1/auth-trace.schema.json`
- `https://simulator.stellar.org/schemas/v1/budget-usage.schema.json`
- `https://simulator.stellar.org/schemas/v1/wasm-stack-trace.schema.json`
- `https://simulator.stellar.org/schemas/v1/common.schema.json`

These URLs provide stable references for external tools. The major version in the path (`v1`) ensures that breaking changes result in new URLs, while minor and patch updates are served at the same URL for automatic compatibility.

## Validation Examples

### JavaScript (Node.js)

Using [Ajv](https://ajv.js.org/) JSON Schema validator:

```javascript
const Ajv = require('ajv');
const addFormats = require('ajv-formats');

// Create validator instance
const ajv = new Ajv({ allErrors: true });
addFormats(ajv);

// Load schemas
const responseSchema = require('./simulation-response.schema.json');
const diagnosticEventSchema = require('./diagnostic-event.schema.json');
const authTraceSchema = require('./auth-trace.schema.json');
const budgetUsageSchema = require('./budget-usage.schema.json');
const wasmStackTraceSchema = require('./wasm-stack-trace.schema.json');
const categorizedEventSchema = require('./categorized-event.schema.json');
const commonSchema = require('./common.schema.json');

// Add all schemas to validator
ajv.addSchema(commonSchema);
ajv.addSchema(diagnosticEventSchema);
ajv.addSchema(categorizedEventSchema);
ajv.addSchema(authTraceSchema);
ajv.addSchema(budgetUsageSchema);
ajv.addSchema(wasmStackTraceSchema);

// Compile response schema
const validate = ajv.compile(responseSchema);

// Validate simulator response
const response = {
  version: "1.0.0",
  request_id: "req-12345",
  status: "success",
  result: { fee_charged: "1000" },
  diagnostic_events: [
    {
      event_type: "contract",
      topics: ["transfer"],
      data: "...",
      in_successful_contract_call: true
    }
  ],
  budget_usage: {
    cpu_instructions: 50000,
    memory_bytes: 2048,
    operations_count: 10,
    cpu_limit: 100000,
    memory_limit: 4096,
    cpu_usage_percent: 50.0,
    memory_usage_percent: 50.0
  }
};

const valid = validate(response);

if (!valid) {
  console.error('Validation errors:', validate.errors);
} else {
  console.log('Response is valid!');
}
```

### Python

Using [jsonschema](https://python-jsonschema.readthedocs.io/) library:

```python
import json
from jsonschema import validate, RefResolver, ValidationError
from pathlib import Path

# Load schemas
schema_dir = Path(__file__).parent
response_schema = json.loads((schema_dir / 'simulation-response.schema.json').read_text())

# Create resolver for $ref resolution
resolver = RefResolver(
    base_uri=f'file://{schema_dir}/',
    referrer=response_schema
)

# Simulator response to validate
response = {
    "version": "1.0.0",
    "request_id": "req-12345",
    "status": "success",
    "result": {"fee_charged": "1000"},
    "diagnostic_events": [
        {
            "event_type": "contract",
            "topics": ["transfer"],
            "data": "...",
            "in_successful_contract_call": True
        }
    ],
    "budget_usage": {
        "cpu_instructions": 50000,
        "memory_bytes": 2048,
        "operations_count": 10,
        "cpu_limit": 100000,
        "memory_limit": 4096,
        "cpu_usage_percent": 50.0,
        "memory_usage_percent": 50.0
    }
}

# Validate
try:
    validate(instance=response, schema=response_schema, resolver=resolver)
    print("Response is valid!")
except ValidationError as e:
    print(f"Validation error: {e.message}")
    print(f"Failed at path: {list(e.path)}")
```

### Go

Using [gojsonschema](https://github.com/xeipuuv/gojsonschema):

```go
package main

import (
    "encoding/json"
    "fmt"
    "github.com/xeipuuv/gojsonschema"
)

func main() {
    // Load schema
    schemaLoader := gojsonschema.NewReferenceLoader("file:///path/to/simulation-response.schema.json")
    
    // Simulator response to validate
    response := map[string]interface{}{
        "version":    "1.0.0",
        "request_id": "req-12345",
        "status":     "success",
        "result": map[string]interface{}{
            "fee_charged": "1000",
        },
        "diagnostic_events": []interface{}{
            map[string]interface{}{
                "event_type":                  "contract",
                "topics":                      []string{"transfer"},
                "data":                        "...",
                "in_successful_contract_call": true,
            },
        },
        "budget_usage": map[string]interface{}{
            "cpu_instructions":      50000,
            "memory_bytes":          2048,
            "operations_count":      10,
            "cpu_limit":             100000,
            "memory_limit":          4096,
            "cpu_usage_percent":     50.0,
            "memory_usage_percent":  50.0,
        },
    }
    
    // Convert to JSON
    responseJSON, _ := json.Marshal(response)
    documentLoader := gojsonschema.NewBytesLoader(responseJSON)
    
    // Validate
    result, err := gojsonschema.Validate(schemaLoader, documentLoader)
    if err != nil {
        panic(err)
    }
    
    if result.Valid() {
        fmt.Println("Response is valid!")
    } else {
        fmt.Println("Validation errors:")
        for _, err := range result.Errors() {
            fmt.Printf("- %s: %s\n", err.Field(), err.Description())
        }
    }
}
```

### Rust

Using [jsonschema](https://docs.rs/jsonschema/) crate:

```rust
use jsonschema::{Draft, JSONSchema};
use serde_json::json;
use std::fs;

fn main() {
    // Load schema
    let schema_str = fs::read_to_string("simulation-response.schema.json")
        .expect("Failed to read schema file");
    let schema_json: serde_json::Value = serde_json::from_str(&schema_str)
        .expect("Failed to parse schema");
    
    // Compile schema
    let compiled = JSONSchema::options()
        .with_draft(Draft::Draft202012)
        .compile(&schema_json)
        .expect("Failed to compile schema");
    
    // Simulator response to validate
    let response = json!({
        "version": "1.0.0",
        "request_id": "req-12345",
        "status": "success",
        "result": {
            "fee_charged": "1000"
        },
        "diagnostic_events": [
            {
                "event_type": "contract",
                "topics": ["transfer"],
                "data": "...",
                "in_successful_contract_call": true
            }
        ],
        "budget_usage": {
            "cpu_instructions": 50000,
            "memory_bytes": 2048,
            "operations_count": 10,
            "cpu_limit": 100000,
            "memory_limit": 4096,
            "cpu_usage_percent": 50.0,
            "memory_usage_percent": 50.0
        }
    });
    
    // Validate
    match compiled.validate(&response) {
        Ok(_) => println!("Response is valid!"),
        Err(errors) => {
            println!("Validation errors:");
            for error in errors {
                println!("- {}: {}", error.instance_path, error);
            }
        }
    }
}
```

## Migration Guide

### Upgrading Between Major Versions

Major version changes indicate breaking changes that require code updates. Follow these steps when upgrading:

#### 1. Review the CHANGELOG

Check `CHANGELOG.md` for detailed information about what changed between versions:

```bash
# Example: Upgrading from v1 to v2
git diff v1.0.0..v2.0.0 -- docs/schema/CHANGELOG.md
```

#### 2. Update Schema URLs

Update your schema references to point to the new major version:

```javascript
// Before (v1)
const schema = require('./simulation-response.schema.json');
// $id: https://simulator.stellar.org/schemas/v1/simulation-response.schema.json

// After (v2)
const schema = require('./simulation-response.schema.json');
// $id: https://simulator.stellar.org/schemas/v2/simulation-response.schema.json
```

#### 3. Handle Removed Fields

If fields were removed, update your code to handle their absence:

```javascript
// Before (v1)
const eventType = response.diagnostic_events[0].event_type;

// After (v2) - if event_type was removed
const eventType = response.diagnostic_events[0].type; // New field name
```

#### 4. Handle Type Changes

If field types changed, update your parsing logic:

```javascript
// Before (v1) - protocol_version was integer
const version = response.protocol_version; // number

// After (v2) - protocol_version is now string
const version = parseInt(response.protocol_version); // parse string to number
```

#### 5. Handle New Required Fields

If previously optional fields became required, ensure you provide them:

```javascript
// Before (v1) - error field was optional
const request = {
  version: "1.0.0",
  request_id: "req-123",
  status: "error"
  // error field was optional
};

// After (v2) - error field is now required when status is "error"
const request = {
  version: "2.0.0",
  request_id: "req-123",
  status: "error",
  error: "Simulation failed" // Now required
};
```

#### 6. Test Thoroughly

Run your validation tests against the new schema version:

```bash
npm test
# or
pytest tests/
# or
go test ./...
# or
cargo test
```

### Backward Compatibility

Minor and patch version updates are backward-compatible:

- **Minor versions** (1.X.0): Add new optional fields, safe to upgrade without code changes
- **Patch versions** (1.0.X): Documentation only, no functional changes

You can safely upgrade to new minor/patch versions without modifying your code. However, you may want to take advantage of new optional fields added in minor versions.

### Version Pinning

For production systems, consider pinning to specific versions:

```json
{
  "dependencies": {
    "@stellar/simulator-schemas": "1.2.3"
  }
}
```

This prevents unexpected changes from automatic updates while allowing you to upgrade on your own schedule.

## Contributing

### Proposing Schema Changes

To propose changes to the schemas:

1. **Open an issue** describing the proposed change and rationale
2. **Determine version impact**:
   - Breaking change → Major version bump
   - New optional field → Minor version bump
   - Documentation fix → Patch version bump
3. **Update schema files** with the proposed changes
4. **Update CHANGELOG.md** with the change description
5. **Update version field** in affected schema files
6. **Run validation tests** to ensure schemas are valid
7. **Submit a pull request** with all changes

### Schema Validation

Before submitting changes, validate all schemas:

```bash
# Using Node.js validation script
node docs/schema/validate-schemas.js

# Or using ajv-cli
npx ajv compile -s "docs/schema/*.schema.json"
```

### Testing Requirements

All schema changes must include:

- **Unit tests** for specific field validations
- **Property-based tests** for schema completeness
- **Integration tests** validating real simulator output
- **Documentation updates** in README.md and CHANGELOG.md

## Additional Resources

- [JSON Schema Specification](https://json-schema.org/)
- [JSON Schema 2020-12 Draft](https://json-schema.org/draft/2020-12/json-schema-core.html)
- [Understanding JSON Schema](https://json-schema.org/understanding-json-schema/)
- [Stellar Simulator Documentation](https://github.com/stellar/stellar-simulator)

## Support

For questions or issues with the schemas:

- Open an issue in the [GitHub repository](https://github.com/stellar/stellar-simulator/issues)
- Join the [Stellar Developer Discord](https://discord.gg/stellar)
- Check the [CHANGELOG.md](./CHANGELOG.md) for recent updates
