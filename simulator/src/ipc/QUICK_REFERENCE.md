# LogEntry Quick Reference Card

## Import

```rust
use crate::ipc::{emit_trace_log, emit_debug_log, emit_info_log, emit_warn_log, emit_error_log};
```

## Basic Usage

### Simple Message (No Fields)

```rust
emit_info_log("simulator::module", "Operation completed", None);
```

### Message with Fields

```rust
emit_debug_log(
    "simulator::module",
    "Processing item",
    Some(serde_json::json!({
        "item_id": 123,
        "status": "pending"
    }))
);
```

## Log Levels

| Function | Level | Use For |
|----------|-------|---------|
| `emit_trace_log()` | `trace` | Finest-grained execution flow |
| `emit_debug_log()` | `debug` | Development/troubleshooting info |
| `emit_info_log()` | `info` | General operational events |
| `emit_warn_log()` | `warn` | Potentially problematic situations |
| `emit_error_log()` | `error` | Failures and exceptions |

## Common Patterns

### Pattern 1: Function Entry/Exit

```rust
pub fn process_contract(contract_id: &str) {
    emit_trace_log(
        "simulator::runner",
        "Entering process_contract",
        Some(serde_json::json!({"contract_id": contract_id}))
    );
    
    // ... function body ...
    
    emit_trace_log(
        "simulator::runner",
        "Exiting process_contract",
        Some(serde_json::json!({"contract_id": contract_id, "result": "success"}))
    );
}
```

### Pattern 2: Error Handling

```rust
match operation() {
    Ok(result) => {
        emit_debug_log(
            "simulator::module",
            "Operation succeeded",
            Some(serde_json::json!({"result": result}))
        );
    }
    Err(e) => {
        emit_error_log(
            "simulator::module",
            "Operation failed",
            Some(serde_json::json!({
                "error": e.to_string(),
                "error_type": std::any::type_name_of_val(&e)
            }))
        );
    }
}
```

### Pattern 3: Performance Metrics

```rust
let start = std::time::Instant::now();
let result = expensive_operation();
let duration = start.elapsed();

emit_debug_log(
    "simulator::perf",
    "Operation completed",
    Some(serde_json::json!({
        "duration_ms": duration.as_millis(),
        "duration_us": duration.as_micros(),
        "result_size": result.len()
    }))
);
```

### Pattern 4: State Changes

```rust
emit_info_log(
    "simulator::state",
    "State transition",
    Some(serde_json::json!({
        "from": old_state,
        "to": new_state,
        "trigger": "user_action"
    }))
);
```

### Pattern 5: Cache Operations

```rust
if let Some(value) = cache.get(key) {
    emit_debug_log(
        "simulator::cache",
        "Cache hit",
        Some(serde_json::json!({
            "key": key,
            "value_size": value.len()
        }))
    );
} else {
    emit_debug_log(
        "simulator::cache",
        "Cache miss",
        Some(serde_json::json!({"key": key}))
    );
}
```

## Field Naming Conventions

- Use `snake_case` for field names
- Use standard units in field names:
  - `duration_ms`, `duration_us`, `duration_ns` (time)
  - `size_bytes`, `size_kb`, `size_mb` (size)
  - `count`, `total`, `limit` (quantities)

## Target (Module Path) Convention

Always use the full module path pattern:
- ✅ `"simulator::runner"`
- ✅ `"simulator::storage::cache"`
- ✅ `"simulator::ipc::types"`
- ❌ `"runner"` (too short)
- ❌ `"RUNNER"` (wrong case)

## Output Format

Each log becomes an IPC frame:

```json
{
  "type": "log",
  "seq": 0,
  "data": {
    "timestamp": "2026-08-26T15:30:45.123456789Z",
    "level": "debug",
    "target": "simulator::module",
    "message": "Your message",
    "fields": {
      "key": "value"
    }
  }
}
```

## Migration Cheat Sheet

| Old | New |
|-----|-----|
| `println!("msg")` | `emit_info_log("module", "msg", None)` |
| `eprintln!("err")` | `emit_error_log("module", "err", None)` |
| `tracing::debug!("msg")` | `emit_debug_log("module", "msg", None)` |
| `tracing::trace!("msg")` | `emit_trace_log("module", "msg", None)` |

## Advanced: Custom LogEntry

For full control, use the `LogEntry` type directly:

```rust
use crate::ipc::{LogEntry, LogLevel, SpanContext};

let entry = LogEntry::new(LogLevel::Debug, "simulator::vm", "Processing")
    .with_fields(serde_json::json!({"id": 123}))
    .with_span(SpanContext {
        name: "invoke_contract".to_string(),
        id: 456,
        parent_id: Some(100),
    });
entry.emit();
```

## Performance Tips

1. **Avoid expensive field computations:**
   ```rust
   // Bad: Always computes
   emit_debug_log("mod", "msg", Some(serde_json::json!({
       "result": expensive_fn()
   })));
   
   // Good: Compute only when needed
   let result = expensive_fn();
   emit_debug_log("mod", "msg", Some(serde_json::json!({
       "result": result
   })));
   ```

2. **Use `None` for simple messages:**
   ```rust
   emit_info_log("simulator::startup", "Initialized", None);
   ```

3. **Batch related fields:**
   ```rust
   // Good: Single log with multiple fields
   emit_debug_log("mod", "Status", Some(serde_json::json!({
       "field1": val1,
       "field2": val2,
       "field3": val3
   })));
   
   // Bad: Multiple logs
   emit_debug_log("mod", "Field1", Some(serde_json::json!({"field1": val1})));
   emit_debug_log("mod", "Field2", Some(serde_json::json!({"field2": val2})));
   emit_debug_log("mod", "Field3", Some(serde_json::json!({"field3": val3})));
   ```

## Common Fields

Reusable field patterns:

```rust
// Contract invocation
serde_json::json!({
    "contract_id": contract_id,
    "function": function_name,
    "args": args_array,
    "ledger": ledger_seq
})

// Storage operation
serde_json::json!({
    "key": storage_key,
    "durability": "persistent|temporary",
    "size_bytes": data.len()
})

// Performance metrics
serde_json::json!({
    "duration_us": duration.as_micros(),
    "cpu_insns": cpu_instructions,
    "mem_bytes": memory_bytes
})

// Error details
serde_json::json!({
    "error": error.to_string(),
    "error_kind": format!("{:?}", error.kind()),
    "context": additional_context
})
```

## Testing

Access structured logs in tests (when captured):

```rust
#[test]
fn test_with_logging() {
    // Structured logs are emitted but not visible in test output
    // unless captured by test infrastructure
    emit_debug_log("test", "Test message", None);
    
    // Your test assertions...
}
```

## See Also

- `LOG_ENTRY_SCHEMA.md` - Full schema documentation
- `INTEGRATION_EXAMPLE.md` - Detailed examples
- `CHANGES_SUMMARY.md` - Implementation details
