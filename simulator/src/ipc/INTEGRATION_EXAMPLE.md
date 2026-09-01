# LogEntry Integration Example

This document shows how to replace unstructured logging with the new structured `LogEntry` API.

## Before and After Comparisons

### Example 1: Simple Debug Message

#### Before
```rust
eprintln!("Cache bypassed via --no-cache flag. Re-parsing WASM symbols.");
```

#### After
```rust
use crate::ipc::emit_debug_log;

emit_debug_log(
    "simulator::source_map_cache",
    "Cache bypassed via --no-cache flag",
    Some(serde_json::json!({"reason": "no_cache_flag"}))
);
```

### Example 2: Error with Context

#### Before
```rust
eprintln!("Failed to open cache file: {}", e);
```

#### After
```rust
use crate::ipc::emit_error_log;

emit_error_log(
    "simulator::source_map_cache",
    "Failed to open cache file",
    Some(serde_json::json!({
        "error": e.to_string(),
        "cache_path": cache_path.display().to_string()
    }))
);
```

### Example 3: Info Message with Structured Data

#### Before
```rust
println!(
    "Cache hit! Loading source map from cache for WASM: {}",
    &cache_key[..8.min(cache_key.len())]
);
```

#### After
```rust
use crate::ipc::emit_info_log;

emit_info_log(
    "simulator::source_map_cache",
    "Cache hit! Loading source map from cache",
    Some(serde_json::json!({
        "cache_key_prefix": &cache_key[..8.min(cache_key.len())],
        "wasm_hash": wasm_hash
    }))
);
```

### Example 4: Trace-Level Logging

#### Before
```rust
tracing::debug!("--no-cache: skipping cache, re-parsing WASM symbols from scratch.");
```

#### After
```rust
use crate::ipc::emit_debug_log;

emit_debug_log(
    "simulator::source_mapper",
    "Skipping cache, re-parsing WASM symbols from scratch",
    Some(serde_json::json!({"reason": "no_cache_flag"}))
);
```

### Example 5: Test Injection Logging

#### Before
```rust
eprintln!(
    "Injecting ContractData: contract={:?}, key={:?}, durability={:?}",
    data.contract, data.key, data.durability
);
```

#### After
```rust
use crate::ipc::emit_debug_log;

emit_debug_log(
    "simulator::test",
    "Injecting ContractData",
    Some(serde_json::json!({
        "contract": format!("{:?}", data.contract),
        "key": format!("{:?}", data.key),
        "durability": format!("{:?}", data.durability)
    }))
);
```

## Integration Pattern

### Step 1: Import the API

Add to your module:
```rust
use crate::ipc::{emit_trace_log, emit_debug_log, emit_info_log, emit_warn_log, emit_error_log};
```

### Step 2: Identify Log Level

Map your existing logs to appropriate levels:
- `println!` → `emit_info_log` (informational messages)
- `eprintln!` → `emit_warn_log` or `emit_error_log` (depending on severity)
- `tracing::debug!` → `emit_debug_log` (debug information)
- `tracing::trace!` → `emit_trace_log` (detailed execution flow)

### Step 3: Extract Structured Fields

Convert string interpolation to structured fields:

#### Before
```rust
eprintln!("Failed to remove cache file {:?}: {}", cache_path, e);
```

#### After
```rust
emit_error_log(
    "simulator::source_map_cache",
    "Failed to remove cache file",
    Some(serde_json::json!({
        "cache_path": cache_path.display().to_string(),
        "error": e.to_string()
    }))
);
```

### Step 4: Add Module Context

Always include the module path in the `target` parameter:
```rust
emit_debug_log(
    "simulator::runner",  // Module path
    "Contract execution started",
    None
);
```

## Full Example: Refactoring a Function

### Before
```rust
pub fn load_cache(&self, cache_key: &str) -> Option<Entry> {
    println!("Loading cache for key: {}", cache_key);
    
    let cache_path = self.cache_dir.join(cache_key);
    let file = match File::open(&cache_path) {
        Ok(f) => f,
        Err(e) => {
            eprintln!("Failed to open cache file: {}", e);
            return None;
        }
    };
    
    println!("Cache hit! Found entry.");
    Some(entry)
}
```

### After
```rust
use crate::ipc::{emit_debug_log, emit_error_log, emit_info_log};

pub fn load_cache(&self, cache_key: &str) -> Option<Entry> {
    emit_debug_log(
        "simulator::cache",
        "Attempting to load cache",
        Some(serde_json::json!({
            "cache_key": cache_key
        }))
    );
    
    let cache_path = self.cache_dir.join(cache_key);
    let file = match File::open(&cache_path) {
        Ok(f) => f,
        Err(e) => {
            emit_error_log(
                "simulator::cache",
                "Failed to open cache file",
                Some(serde_json::json!({
                    "cache_path": cache_path.display().to_string(),
                    "error": e.to_string(),
                    "error_kind": format!("{:?}", e.kind())
                }))
            );
            return None;
        }
    };
    
    emit_info_log(
        "simulator::cache",
        "Cache hit! Found entry",
        Some(serde_json::json!({
            "cache_key": cache_key
        }))
    );
    
    Some(entry)
}
```

## Performance Considerations

1. **Conditional Logging**: Consider adding log level checks for hot paths:
   ```rust
   if log_enabled!(Level::Debug) {
       emit_debug_log(...);
   }
   ```

2. **Field Allocation**: Avoid expensive computations for fields that might not be logged:
   ```rust
   // Bad: Always computes expensive_computation()
   emit_debug_log(
       "simulator::hot_path",
       "Processing",
       Some(serde_json::json!({
           "result": expensive_computation()
       }))
   );
   
   // Good: Only computes when needed
   if log_enabled!(Level::Debug) {
       emit_debug_log(
           "simulator::hot_path",
           "Processing",
           Some(serde_json::json!({
               "result": expensive_computation()
           }))
       );
   }
   ```

3. **Structured Fields**: Use `None` for simple messages without context:
   ```rust
   emit_info_log("simulator::startup", "Initialization complete", None);
   ```

## Testing

When writing tests, structured logs help verify behavior:

```rust
#[test]
fn test_cache_miss() {
    // Your test code...
    
    // The structured log output can be captured and verified
    // by the test infrastructure
}
```

## Downstream Consumption

The Go bridge and CLI can now consume structured logs:

```go
// Go example
type LogEntry struct {
    Timestamp string                 `json:"timestamp"`
    Level     string                 `json:"level"`
    Target    string                 `json:"target"`
    Message   string                 `json:"message"`
    Fields    map[string]interface{} `json:"fields,omitempty"`
    Span      *SpanContext           `json:"span,omitempty"`
}

type SpanContext struct {
    Name     string  `json:"name"`
    ID       uint64  `json:"id"`
    ParentID *uint64 `json:"parent_id,omitempty"`
}

// Parse log frame
var frame StreamFrame
if err := json.Unmarshal(line, &frame); err != nil {
    return err
}

if frame.Type == "log" {
    var logEntry LogEntry
    if err := json.Unmarshal(frame.Data, &logEntry); err != nil {
        return err
    }
    
    // Process structured log
    fmt.Printf("[%s] %s: %s\n", logEntry.Level, logEntry.Target, logEntry.Message)
    
    // Access structured fields
    if logEntry.Fields != nil {
        for k, v := range logEntry.Fields {
            fmt.Printf("  %s: %v\n", k, v)
        }
    }
}
```

## Best Practices

1. **Use Consistent Target Names**: Follow the module path pattern
   - ✅ `"simulator::runner"`
   - ✅ `"simulator::storage::cache"`
   - ❌ `"runner"`
   - ❌ `"RUNNER"`

2. **Provide Context in Fields**: Include relevant identifiers
   ```rust
   emit_debug_log(
       "simulator::runner",
       "Contract invoked",
       Some(serde_json::json!({
           "contract_id": contract_id,
           "function": function_name,
           "ledger": ledger_seq
       }))
   );
   ```

3. **Use Appropriate Log Levels**:
   - `trace`: Very detailed execution flow
   - `debug`: Development/troubleshooting info
   - `info`: General operational events
   - `warn`: Potentially problematic situations
   - `error`: Failures and exceptions

4. **Include Error Details**: When logging errors, include error type and context
   ```rust
   emit_error_log(
       "simulator::io",
       "File operation failed",
       Some(serde_json::json!({
           "operation": "read",
           "path": path.display().to_string(),
           "error": error.to_string(),
           "error_kind": format!("{:?}", error.kind())
       }))
   );
   ```

5. **Avoid PII**: Don't log sensitive user data in structured fields
