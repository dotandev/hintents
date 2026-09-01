# IPC LogEntry Schema Documentation

## Overview

This document describes the standard `LogEntry` JSON schema used for all trace and debug logs sent over IPC in the simulator. The schema provides uniform structure for downstream consumers (e.g., the Go bridge, CLI, or external tooling).

## Schema Definition

### LogEntry Structure

```json
{
  "timestamp": "2026-08-26T15:30:45.123456789Z",
  "level": "debug",
  "target": "simulator::runner",
  "message": "Starting contract invocation",
  "fields": {
    "contract_id": "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM",
    "function": "increment",
    "ledger": 1234
  },
  "span": {
    "name": "invoke_contract",
    "id": 42,
    "parent_id": 10
  }
}
```

### Fields

| Field       | Type         | Required | Description                                                    |
|-------------|--------------|----------|----------------------------------------------------------------|
| `timestamp` | string       | Yes      | ISO 8601 timestamp with nanosecond precision (RFC3339 format)  |
| `level`     | string       | Yes      | Log level: `"trace"`, `"debug"`, `"info"`, `"warn"`, `"error"` |
| `target`    | string       | Yes      | Module path where the log originated (e.g., `"simulator::runner"`) |
| `message`   | string       | Yes      | Human-readable log message                                     |
| `fields`    | object       | No       | Optional key-value pairs providing additional structured context |
| `span`      | SpanContext  | No       | Optional span context for distributed tracing integration      |

### SpanContext Structure

```json
{
  "name": "invoke_contract",
  "id": 42,
  "parent_id": 10
}
```

| Field       | Type    | Required | Description                                          |
|-------------|---------|----------|------------------------------------------------------|
| `name`      | string  | Yes      | Span name (e.g., `"invoke_contract"`, `"storage_read"`) |
| `id`        | number  | Yes      | Unique span identifier within the current trace      |
| `parent_id` | number  | No       | Optional parent span ID for hierarchical traces      |

### Log Levels

The `level` field must be one of the following values:

- `"trace"` - Finest-grained logging for detailed execution flow
- `"debug"` - Debug information for development and troubleshooting
- `"info"` - General informational messages
- `"warn"` - Warning messages for potentially problematic situations
- `"error"` - Error messages for failures and exceptions

## Usage

### Rust API

The `simulator::ipc` module provides both low-level and high-level APIs for emitting structured logs:

#### Low-Level API

```rust
use simulator::ipc::{LogEntry, LogLevel, SpanContext};

// Create a basic log entry
let entry = LogEntry::new(
    LogLevel::Debug,
    "simulator::runner",
    "Contract execution started"
);
entry.emit();

// Create a log entry with structured fields
let entry = LogEntry::new(LogLevel::Info, "simulator::storage", "Cache hit")
    .with_fields(serde_json::json!({
        "key": "counter",
        "value": 42,
        "ttl": 3600
    }));
entry.emit();

// Create a log entry with span context
let span = SpanContext {
    name: "invoke_contract".to_string(),
    id: 123,
    parent_id: Some(100),
};
let entry = LogEntry::new(LogLevel::Trace, "simulator::vm", "Entering span")
    .with_span(span);
entry.emit();
```

#### High-Level Convenience Functions

```rust
use simulator::ipc::{emit_trace_log, emit_debug_log, emit_info_log, emit_warn_log, emit_error_log};

// Emit a trace-level log without fields
emit_trace_log("simulator::runner", "Function entry", None);

// Emit a debug-level log with structured fields
emit_debug_log(
    "simulator::storage",
    "Storage read completed",
    Some(serde_json::json!({
        "key": "counter",
        "duration_us": 123
    }))
);

// Emit an info-level log
emit_info_log("simulator::host", "Simulation started", None);

// Emit a warning
emit_warn_log(
    "simulator::memory",
    "Memory usage high",
    Some(serde_json::json!({
        "used_mb": 512,
        "limit_mb": 1024
    }))
);

// Emit an error
emit_error_log(
    "simulator::host",
    "Contract execution failed",
    Some(serde_json::json!({
        "error_code": 1,
        "reason": "insufficient balance"
    }))
);
```

### IPC Frame Format

Log entries are transmitted as NDJSON frames with the following structure:

```json
{
  "type": "log",
  "seq": 0,
  "data": {
    "timestamp": "2026-08-26T15:30:45.123456789Z",
    "level": "debug",
    "target": "simulator::runner",
    "message": "Starting contract invocation",
    "fields": {
      "contract_id": "CTEST",
      "ledger": 1234
    }
  }
}
```

The `data` field contains the complete `LogEntry` structure.

## Examples

### Example 1: Simple Debug Log

```rust
emit_debug_log("simulator::test", "Test started", None);
```

**Output:**
```json
{
  "type": "log",
  "seq": 0,
  "data": {
    "timestamp": "2026-08-26T15:30:45.123456789Z",
    "level": "debug",
    "target": "simulator::test",
    "message": "Test started"
  }
}
```

### Example 2: Log with Structured Fields

```rust
emit_info_log(
    "simulator::runner",
    "Contract invoked successfully",
    Some(serde_json::json!({
        "contract_id": "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM",
        "function": "increment",
        "args": ["10"],
        "result": "ok",
        "cpu_insns": 12345,
        "mem_bytes": 4096
    }))
);
```

**Output:**
```json
{
  "type": "log",
  "seq": 0,
  "data": {
    "timestamp": "2026-08-26T15:30:45.123456789Z",
    "level": "info",
    "target": "simulator::runner",
    "message": "Contract invoked successfully",
    "fields": {
      "contract_id": "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM",
      "function": "increment",
      "args": ["10"],
      "result": "ok",
      "cpu_insns": 12345,
      "mem_bytes": 4096
    }
  }
}
```

### Example 3: Log with Span Context

```rust
let span = SpanContext {
    name: "storage_operation".to_string(),
    id: 456,
    parent_id: Some(123),
};

let entry = LogEntry::new(
    LogLevel::Trace,
    "simulator::storage",
    "Reading persistent data"
)
.with_fields(serde_json::json!({
    "key": "user_balance",
    "durability": "persistent"
}))
.with_span(span);

entry.emit();
```

**Output:**
```json
{
  "type": "log",
  "seq": 0,
  "data": {
    "timestamp": "2026-08-26T15:30:45.123456789Z",
    "level": "trace",
    "target": "simulator::storage",
    "message": "Reading persistent data",
    "fields": {
      "key": "user_balance",
      "durability": "persistent"
    },
    "span": {
      "name": "storage_operation",
      "id": 456,
      "parent_id": 123
    }
  }
}
```

## Migration Guide

### Before (Unstructured Logging)

```rust
eprintln!("Starting contract execution for {}", contract_id);
println!("Debug: storage read took {} us", duration);
```

### After (Structured Logging)

```rust
emit_info_log(
    "simulator::runner",
    "Starting contract execution",
    Some(serde_json::json!({"contract_id": contract_id}))
);

emit_debug_log(
    "simulator::storage",
    "Storage read completed",
    Some(serde_json::json!({"duration_us": duration}))
);
```

## Benefits

1. **Uniform Structure**: All logs follow a consistent schema, making them easier to parse and analyze
2. **Machine-Readable**: JSON format enables automated log processing and analysis
3. **Structured Context**: The `fields` object allows attaching arbitrary key-value pairs without string formatting
4. **Tracing Support**: Built-in span context for distributed tracing integration
5. **Timestamp Precision**: Nanosecond-precision timestamps for accurate timing analysis
6. **Type Safety**: Rust enums ensure log levels are valid at compile time

## Testing

The implementation includes comprehensive tests in `simulator/src/ipc/mod.rs`:

- Schema validation tests
- Serialization/deserialization tests
- Convenience function tests
- Span context tests
- Minimal and complete LogEntry tests

Run the tests with:
```bash
cargo test --package erst-sim --lib ipc::tests
```

## See Also

- `simulator/src/ipc/types.rs` - Implementation of LogEntry, LogLevel, and SpanContext
- `simulator/src/ipc/mod.rs` - Public API and tests
