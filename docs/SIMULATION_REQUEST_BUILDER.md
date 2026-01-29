# SimulationRequest Builder Pattern

This document explains the builder pattern implementation for `SimulationRequest` objects in the erst simulator package.

## Overview

The `SimulationRequestBuilder` provides a fluent, chainable interface for constructing `SimulationRequest` objects. This pattern makes the code more readable, less error-prone, and provides built-in validation.

## Why Use the Builder Pattern?

### Before (Direct Struct Initialization)

```go
// Error-prone: easy to forget required fields or make mistakes
req := &simulator.SimulationRequest{
    EnvelopeXdr:   txResp.EnvelopeXdr,
    ResultMetaXdr: txResp.ResultMetaXdr,
    LedgerEntries: nil, // TODO: fetch ledger entries if needed
}
```

**Problems:**
- No validation until runtime
- Easy to forget required fields
- No clear indication of what's required vs optional
- Difficult to read with many fields

### After (Builder Pattern)

```go
// Clear, validated, and self-documenting
req, err := simulator.NewSimulationRequestBuilder().
    WithEnvelopeXDR(txResp.EnvelopeXdr).
    WithResultMetaXDR(txResp.ResultMetaXdr).
    Build()

if err != nil {
    return fmt.Errorf("invalid simulation request: %w", err)
}
```

**Benefits:**
- ✅ Validation at build time
- ✅ Clear, readable method names
- ✅ Self-documenting code
- ✅ Prevents invalid states
- ✅ Method chaining for fluent API

## Basic Usage

### Creating a Simple Request

```go
req, err := simulator.NewSimulationRequestBuilder().
    WithEnvelopeXDR("AAAAAgAAAACE...").
    WithResultMetaXDR("AAAAAQAAAAA...").
    Build()

if err != nil {
    log.Fatalf("Failed to build request: %v", err)
}
```

### Adding Ledger Entries

```go
// Add entries one at a time
req, err := simulator.NewSimulationRequestBuilder().
    WithEnvelopeXDR("AAAAAgAAAACE...").
    WithResultMetaXDR("AAAAAQAAAAA...").
    WithLedgerEntry("key1", "value1").
    WithLedgerEntry("key2", "value2").
    Build()
```

### Bulk Ledger Entries

```go
entries := map[string]string{
    "contract_key_1": "contract_value_1",
    "contract_key_2": "contract_value_2",
}

req, err := simulator.NewSimulationRequestBuilder().
    WithEnvelopeXDR("AAAAAgAAAACE...").
    WithResultMetaXDR("AAAAAQAAAAA...").
    WithLedgerEntries(entries).
    Build()
```

## API Reference

### Constructor

#### `NewSimulationRequestBuilder() *SimulationRequestBuilder`

Creates a new builder instance with empty fields and an empty ledger entries map.

### Builder Methods (Chainable)

All builder methods return `*SimulationRequestBuilder` for method chaining.

#### `WithEnvelopeXDR(xdr string) *SimulationRequestBuilder`

Sets the XDR encoded TransactionEnvelope. **Required field.**

#### `WithResultMetaXDR(xdr string) *SimulationRequestBuilder`

Sets the XDR encoded TransactionResultMeta. **Required field.**

#### `WithLedgerEntry(key, value string) *SimulationRequestBuilder`

Adds a single ledger entry. Both key and value must be non-empty.

#### `WithLedgerEntries(entries map[string]string) *SimulationRequestBuilder`

Sets multiple ledger entries at once. Replaces any previously set entries. Passing `nil` clears all entries.

#### `Reset() *SimulationRequestBuilder`

Clears all fields and errors, allowing the builder to be reused.

### Terminal Methods

#### `Build() (*SimulationRequest, error)`

Constructs and validates the final `SimulationRequest`. Returns an error if:
- Required fields are missing (EnvelopeXDR or ResultMetaXDR)
- Validation errors were collected during building
- Ledger entry keys or values are empty

#### `MustBuild() *SimulationRequest`

Like `Build()` but panics if there's an error. Use only when you're certain the request is valid (e.g., in tests with known good data).

## Validation

The builder performs validation at multiple stages:

### During Building

- Empty ledger entry keys are rejected
- Empty ledger entry values are rejected
- Errors are collected and reported at `Build()` time

### At Build Time

- Envelope XDR is required
- Result Meta XDR is required
- All collected errors are checked

### Example Error Handling

```go
req, err := simulator.NewSimulationRequestBuilder().
    WithEnvelopeXDR("").  // Invalid: empty
    WithResultMetaXDR("valid_xdr").
    Build()

if err != nil {
    // Error: envelope XDR is required
    log.Printf("Validation failed: %v", err)
}
```

## Advanced Usage

### Reusing a Builder

```go
builder := simulator.NewSimulationRequestBuilder()

// Build first request
req1, _ := builder.
    WithEnvelopeXDR("envelope1").
    WithResultMetaXDR("result1").
    Build()

// Reset and build second request
req2, _ := builder.
    Reset().
    WithEnvelopeXDR("envelope2").
    WithResultMetaXDR("result2").
    Build()
```

### Conditional Building

```go
builder := simulator.NewSimulationRequestBuilder().
    WithEnvelopeXDR(txResp.EnvelopeXdr).
    WithResultMetaXDR(txResp.ResultMetaXdr)

// Conditionally add ledger entries
if needsLedgerEntries {
    builder.WithLedgerEntries(fetchLedgerEntries())
}

req, err := builder.Build()
```

### Using MustBuild in Tests

```go
func TestSimulation(t *testing.T) {
    // Safe to use MustBuild with known valid data
    req := simulator.NewSimulationRequestBuilder().
        WithEnvelopeXDR("valid_test_envelope").
        WithResultMetaXDR("valid_test_result").
        MustBuild()
    
    // Test with req...
}
```

## Migration Guide

### Migrating Existing Code

**Before:**
```go
simReq := &simulator.SimulationRequest{
    EnvelopeXdr:   txResp.EnvelopeXdr,
    ResultMetaXdr: txResp.ResultMetaXdr,
    LedgerEntries: nil,
}
```

**After:**
```go
simReq, err := simulator.NewSimulationRequestBuilder().
    WithEnvelopeXDR(txResp.EnvelopeXdr).
    WithResultMetaXDR(txResp.ResultMetaXdr).
    Build()

if err != nil {
    return fmt.Errorf("failed to build simulation request: %w", err)
}
```

## Best Practices

1. **Always check errors from `Build()`** - Don't ignore validation errors
2. **Use `MustBuild()` only in tests** - It panics on error, which is fine for tests but not production code
3. **Reuse builders when building multiple similar requests** - Use `Reset()` to clear state
4. **Add ledger entries individually for clarity** - Unless you have a map already prepared
5. **Let the builder validate** - Don't pre-validate fields yourself

## Testing

The builder includes comprehensive tests covering:
- Basic usage
- Method chaining
- Validation (missing fields, empty values)
- Error handling
- Builder reuse with `Reset()`
- `MustBuild()` panic behavior

Run tests with:
```bash
go test ./internal/simulator/... -v
```

## Future Enhancements

Potential improvements to consider:

- **Validation hooks**: Allow custom validation functions
- **Default values**: Set sensible defaults for optional fields
- **Cloning**: Add a `Clone()` method to copy builder state
- **Partial builds**: Support building incomplete requests for testing
- **JSON support**: Direct JSON serialization from builder
