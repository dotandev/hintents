# IPC Module

This module provides the Inter-Process Communication (IPC) layer for the Erst simulator. It enables the Rust simulator core to communicate with the Go bridge and CLI via newline-delimited JSON (NDJSON) frames over stdout/stdin.

## Module Structure

```
simulator/src/ipc/
├── mod.rs                      # Module root, tests, and exports
├── types.rs                    # Core IPC types and implementations
├── validate.rs                 # Validation utilities
├── decompress.rs               # Decompression utilities
├── README.md                   # This file
├── LOG_ENTRY_SCHEMA.md         # Structured logging schema documentation
├── INTEGRATION_EXAMPLE.md      # Practical integration examples
├── QUICK_REFERENCE.md          # Developer quick reference
└── CHANGES_SUMMARY.md          # Implementation changelog
```

## Features

### 1. Stream Frames

The IPC layer supports multiple frame types for different communication patterns:

- **Snapshot**: Intermediate ledger snapshots during simulation
- **Final**: Terminal frame with complete simulation response
- **FetchResponse**: Response to FETCH_SNAPSHOT commands
- **Chunk**: Partial payload chunks for large responses
- **Log**: Structured log entries (NEW)

### 2. Chunked Streaming

For large payloads, the `ResponseStreamer` enables memory-efficient streaming:

```rust
use crate::ipc::{stream_to_stdout, DEFAULT_CHUNK_TARGET};

let mut streamer = stream_to_stdout(3, DEFAULT_CHUNK_TARGET);
streamer.feed(b"{\"data\":[")?;
streamer.feed(b"\"item1\",")?;
streamer.feed(b"\"item2\"")?;
streamer.feed(b"]}")?;
let total_chunks = streamer.finish()?;
```

### 3. Structured Logging (NEW)

Uniform JSON schema for all trace and debug logs:

```rust
use crate::ipc::{emit_debug_log, emit_error_log};

// Simple log
emit_debug_log("simulator::runner", "Contract started", None);

// Log with structured fields
emit_debug_log(
    "simulator::storage",
    "Cache operation",
    Some(serde_json::json!({
        "operation": "read",
        "key": "counter",
        "hit": true
    }))
);

// Error with context
emit_error_log(
    "simulator::host",
    "Execution failed",
    Some(serde_json::json!({
        "error": error.to_string(),
        "contract_id": contract_id
    }))
);
```

**Benefits:**
- Machine-readable JSON format
- Nanosecond-precision timestamps
- Structured context via `fields` object
- Distributed tracing support via `span` context
- Type-safe log levels

**Learn More:**
- [LOG_ENTRY_SCHEMA.md](LOG_ENTRY_SCHEMA.md) - Full schema documentation
- [INTEGRATION_EXAMPLE.md](INTEGRATION_EXAMPLE.md) - Integration guide
- [QUICK_REFERENCE.md](QUICK_REFERENCE.md) - Quick reference card

### 4. Snapshot Registry

In-memory registry for managing simulation snapshots:

```rust
use crate::ipc::SnapshotRegistry;

let mut registry = SnapshotRegistry::new();
registry.insert(0, serde_json::json!({"ledger": 0, "state": "..."}));
registry.insert(1, serde_json::json!({"ledger": 1, "state": "..."}));

// Fetch batch of snapshots
let snapshots = registry.fetch(0, 5);
```

### 5. IPC Bridge

TCP listener for external IPC connections:

```rust
use crate::ipc::start_ipc_bridge;

let listener = start_ipc_bridge("127.0.0.1:8080")?;
for stream in listener.incoming() {
    // Handle connection
}
```

## Public API

### Types

- `FrameType` - Enum of frame types (Snapshot, Final, FetchResponse, Chunk, Log)
- `StreamFrame` - NDJSON frame structure
- `IpcError` - Error type for IPC operations
- `ResponseStreamer` - Chunked streaming writer
- `SnapshotRegistry` - Snapshot management
- `LogEntry` - Structured log entry
- `LogLevel` - Log level enum (Trace, Debug, Info, Warn, Error)
- `SpanContext` - Distributed tracing span context

### Functions

**Frame Emission:**
- `emit_snapshot_frame(seq, data)` - Emit snapshot frame
- `emit_final_frame(seq, data)` - Emit final frame
- `emit_chunk_frame(seq, total, data)` - Emit chunk frame
- `emit_chunk_raw(seq, total, data)` - Emit chunk with raw bytes

**Structured Logging:**
- `emit_trace_log(target, message, fields)` - Emit trace-level log
- `emit_debug_log(target, message, fields)` - Emit debug-level log
- `emit_info_log(target, message, fields)` - Emit info-level log
- `emit_warn_log(target, message, fields)` - Emit warn-level log
- `emit_error_log(target, message, fields)` - Emit error-level log

**Streaming:**
- `stream_to_stdout(total_chunks, chunk_target)` - Create stdout streamer

**Bridge:**
- `start_ipc_bridge(addr)` - Start TCP listener

**Registry:**
- `parse_command_frame(input)` - Parse command from JSON
- `handle_stdin_command(registry)` - Handle stdin command

## Frame Format

All frames are emitted as newline-delimited JSON (NDJSON):

```json
{"type":"snapshot","seq":0,"data":{"ledger":123,"entries":[...]}}
{"type":"chunk","seq":0,"total":3,"data":"partial payload"}
{"type":"chunk","seq":1,"total":3,"data":"more payload"}
{"type":"chunk","seq":2,"total":3,"data":"final payload"}
{"type":"final","seq":1,"data":{"status":"success","result":...}}
{"type":"log","seq":0,"data":{"timestamp":"...","level":"debug",...}}
```

## Error Handling

The `IpcError` enum provides structured error handling:

```rust
match operation() {
    Ok(result) => { /* success */ }
    Err(IpcError::Io(e)) => { /* I/O error */ }
    Err(IpcError::Json(e)) => { /* JSON error */ }
    Err(IpcError::PortBindingFailed { source }) => { 
        // Inspect source.kind() for specific binding error
    }
    Err(IpcError::Decompress(msg)) => { /* decompression error */ }
}
```

## Testing

Comprehensive test suite in `mod.rs`:

```bash
cargo test --package erst-sim --lib ipc::tests
```

**Test Coverage:**
- Frame serialization/deserialization
- Chunked streaming
- Snapshot registry operations
- Command parsing
- LogEntry schema validation
- Log level serialization
- Span context handling
- Convenience function behavior

## Performance Considerations

### Chunked Streaming
- Default chunk target: 64 KiB (`DEFAULT_CHUNK_TARGET`)
- Adjust `chunk_target` based on payload characteristics
- Large payloads automatically split across multiple frames

### Structured Logging
- Use `None` for fields when no context needed
- Avoid expensive computations in hot paths
- Consider conditional logging for trace-level logs:
  ```rust
  if cfg!(debug_assertions) {
      emit_trace_log(...);
  }
  ```

### Registry
- Batch size capped at 5 snapshots per fetch
- In-memory storage - consider eviction for long-running processes

## Integration with Go Bridge

The Go bridge consumes NDJSON frames:

```go
scanner := bufio.NewScanner(os.Stdin)
for scanner.Scan() {
    var frame StreamFrame
    if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
        return err
    }
    
    switch frame.Type {
    case "snapshot":
        handleSnapshot(frame)
    case "chunk":
        handleChunk(frame)
    case "final":
        handleFinal(frame)
    case "log":
        handleLog(frame)
    }
}
```

For structured logs:

```go
type LogEntry struct {
    Timestamp string                 `json:"timestamp"`
    Level     string                 `json:"level"`
    Target    string                 `json:"target"`
    Message   string                 `json:"message"`
    Fields    map[string]interface{} `json:"fields,omitempty"`
    Span      *SpanContext           `json:"span,omitempty"`
}

func handleLog(frame StreamFrame) {
    var log LogEntry
    if err := json.Unmarshal(frame.Data, &log); err != nil {
        return err
    }
    
    fmt.Printf("[%s] %s: %s\n", log.Level, log.Target, log.Message)
    if log.Fields != nil {
        // Process structured fields
    }
}
```

## Migration Guide

### Adopting Structured Logging

**Step 1: Replace simple println/eprintln**
```rust
// Before
println!("Starting simulation");

// After
emit_info_log("simulator::main", "Starting simulation", None);
```

**Step 2: Extract structured fields**
```rust
// Before
eprintln!("Error in contract {}: {}", contract_id, error);

// After
emit_error_log(
    "simulator::runner",
    "Contract execution failed",
    Some(serde_json::json!({
        "contract_id": contract_id,
        "error": error.to_string()
    }))
);
```

**Step 3: Add module context**
Always include the module path in the `target` parameter:
```rust
emit_debug_log("simulator::storage::cache", "Cache miss", None);
```

See [INTEGRATION_EXAMPLE.md](INTEGRATION_EXAMPLE.md) for more examples.

## Constants

- `DEFAULT_CHUNK_TARGET: usize = 64 * 1024` - Default chunk size (64 KiB)

## Dependencies

- `serde` / `serde_json` - Serialization
- `chrono` - Timestamp generation (for LogEntry)
- `thiserror` - Error handling

## Future Enhancements

- [ ] Tracing subscriber integration for automatic span tracking
- [ ] Compression for large frame payloads
- [ ] Frame batching for reduced stdout syscalls
- [ ] Log filtering by level (environment variable)
- [ ] Metric frames for performance telemetry
- [ ] Binary encoding option for high-throughput scenarios

## Contributing

When adding new frame types:
1. Add variant to `FrameType` enum
2. Create corresponding emit function
3. Add serialization tests
4. Update Go bridge parser
5. Document in this README

When enhancing logging:
1. Maintain backward compatibility with existing frame types
2. Follow the LogEntry schema conventions
3. Add tests for new functionality
4. Update documentation

## License

Copyright 2026 Erst Users  
SPDX-License-Identifier: Apache-2.0
