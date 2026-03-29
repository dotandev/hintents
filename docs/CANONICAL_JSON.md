# Canonical JSON Serialization for Audit Logs

## Overview

This document describes the canonical JSON serialization implementation used for audit log payload hashing. This ensures deterministic hash generation across different operating systems, Go versions, and runtime environments.

## Problem Statement

Standard `json.Marshal` in Go does not guarantee consistent key ordering in JSON objects. While the Go specification states that map iteration order is randomized for security reasons, the `encoding/json` package does maintain stable ordering within a single process. However, to ensure absolute determinism across:

- Different OS targets (Linux, macOS, Windows)
- Different Go compiler versions
- Different architectures (amd64, arm64)
- Different runtime environments

We implement a canonical JSON serialization that enforces strict ordering rules.

## Implementation

### Canonical JSON Rules

The canonical JSON implementation follows these rules:

1. **Sorted Keys**: All object keys are sorted alphabetically (lexicographically)
2. **No Whitespace**: No extra whitespace, indentation, or newlines
3. **Consistent Encoding**: Strings, numbers, booleans, and null values use standard JSON encoding
4. **Recursive Application**: Nested objects and arrays are also canonicalized
5. **Array Order Preservation**: Array element order is preserved (not sorted)

### Code Structure

The implementation consists of three main functions:

#### `marshalCanonical(v interface{}) ([]byte, error)`

The primary entry point that:

1. Converts the input struct to a generic `interface{}` via standard JSON marshaling
2. Applies canonical encoding rules
3. Returns the deterministic byte representation

#### `canonicalJSON(v interface{}) ([]byte, error)`

Handles the canonical encoding by dispatching to type-specific encoders.

#### `encodeObject(buf *bytes.Buffer, obj map[string]interface{}) error`

Encodes JSON objects with sorted keys:

```go
// Keys are sorted alphabetically
keys := make([]string, 0, len(obj))
for k := range obj {
    keys = append(keys, k)
}
sort.Strings(keys)
```

## Usage in Audit Logs

### Generation (`audit.go`)

When generating an audit log, the payload is serialized using canonical JSON before hashing:

```go
// Serialize Payload to calculate hash using canonical JSON
payloadBytes, err := marshalCanonical(payload)
if err != nil {
    return nil, fmt.Errorf("failed to marshal payload: %w", err)
}

// Calculate SHA256 hash
hash := sha256.Sum256(payloadBytes)
```

### Verification (`verify.go`)

When verifying an audit log, the same canonical serialization is used:

```go
// Re-calculate Trace Hash using canonical JSON
payloadBytes, err := marshalCanonical(log.Payload)
if err != nil {
    return fmt.Errorf("failed to marshal payload: %w", err)
}

hash := sha256.Sum256(payloadBytes)
```

## Testing

### Test Coverage

The implementation includes comprehensive tests:

1. **Key Ordering Tests**: Verify alphabetical sorting of object keys
2. **Array Tests**: Ensure array order is preserved
3. **Data Type Tests**: Validate all JSON data types
4. **Struct Tests**: Test with Go structs (like `Payload`)
5. **Determinism Tests**: Verify multiple serializations produce identical output
6. **Cross-Platform Tests**: Ensure consistency regardless of field declaration order
7. **Edge Cases**: Empty values, nil arrays, nested structures

### Running Tests

```bash
# Run all canonical JSON tests
go test -v ./internal/cmd -run TestCanonicalJSON

# Run audit-specific tests
go test -v ./internal/cmd -run TestGenerate
go test -v ./internal/cmd -run TestVerify

# Run the complete test suite
./test_canonical_json.sh
```

## Example

Given this payload:

```go
payload := Payload{
    EnvelopeXdr:   "envelope_data",
    ResultMetaXdr: "result_data",
    Events:        []string{"event1", "event2"},
    Logs:          []string{"log1"},
}
```

Canonical JSON output (keys sorted alphabetically):

```json
{
  "envelope_xdr": "envelope_data",
  "events": ["event1", "event2"],
  "logs": ["log1"],
  "result_meta_xdr": "result_data"
}
```

Note that regardless of the order fields are declared in the struct or assigned in code, the JSON output will always have keys in alphabetical order: `envelope_xdr`, `events`, `logs`, `result_meta_xdr`.

## Benefits

1. **Cross-Platform Consistency**: Same payload always produces the same hash
2. **Reproducible Audits**: Audit logs can be verified on any system
3. **Version Independence**: Works across different Go versions
4. **Security**: Prevents hash mismatches that could be exploited
5. **Debugging**: Easier to compare and debug serialized payloads

## References

- [RFC 8785: JSON Canonicalization Scheme (JCS)](https://tools.ietf.org/html/rfc8785)
- Go `encoding/json` package documentation
- Issue #178: Implement Audit payload canonical JSON serialization
