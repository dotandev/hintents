# Real-Time Event Streaming

## Overview

Erst supports real-time streaming of diagnostic events during transaction simulation. Instead of waiting for the entire simulation to complete, events are delivered as they occur via Unix domain sockets.

## Architecture

```
┌─────────────┐                    ┌──────────────────┐
│   Go CLI    │                    │  Rust Simulator  │
│             │                    │                  │
│  1. Creates │                    │                  │
│     Socket  │                    │                  │
│             │                    │                  │
│  2. Starts  │                    │                  │
│   Listening │                    │                  │
│             │                    │                  │
│  3. Spawns  │──── stdin/JSON ───>│  4. Connects to  │
│  Simulator  │                    │     Socket       │
│             │                    │                  │
│  6. Receives│<─── Unix Socket ───│  5. Streams      │
│    Events   │     (real-time)    │     Events       │
│             │                    │                  │
│  7. Displays│                    │  8. Sends        │
│   Progress  │                    │     Complete     │
└─────────────┘                    └──────────────────┘
```

## Benefits

1. **Real-time Feedback**: See events as they happen, not after completion
2. **Better UX**: Progress indicators for long-running simulations
3. **Lower Memory**: Events don't need to be buffered in memory
4. **Debugging**: Identify issues earlier in the execution

## Usage

### Enabling Streaming in Go

```go
import "github.com/dotandev/hintents/internal/simulator"

// Create runner with streaming enabled
runner := &simulator.Runner{
    BinaryPath:      "/path/to/erst-sim",
    EnableStreaming: true,
}

// Run simulation - events will be streamed in real-time
response, err := runner.Run(request)
```

### Custom Stream Handler

Implement the `StreamHandler` interface to process events as they arrive:

```go
type MyHandler struct {
    eventCount int
}

func (h *MyHandler) OnEvent(event simulator.DiagnosticEvent) {
    h.eventCount++
    fmt.Printf("Event #%d: %s\n", h.eventCount, event.EventType)
}

func (h *MyHandler) OnLog(message string) {
    fmt.Printf("LOG: %s\n", message)
}

func (h *MyHandler) OnBudgetUpdate(cpu, mem uint64) {
    fmt.Printf("Budget: CPU=%d, Memory=%d\n", cpu, mem)
}

func (h *MyHandler) OnComplete() {
    fmt.Println("Simulation completed!")
}

func (h *MyHandler) OnError(message string) {
    fmt.Printf("ERROR: %s\n", message)
}
```

### Rust Simulator Integration

The Rust simulator automatically detects the socket path from the request:

```rust
// In SimulationRequest
pub struct SimulationRequest {
    // ... other fields ...
    pub socket_path: Option<String>,
}

// Streaming is automatic when socket_path is provided
if let Some(path) = &request.socket_path {
    let mut streamer = SocketStreamer::connect(path)?;
    
    // Events are streamed as they occur
    streamer.send_event(diagnostic_event)?;
    
    // Budget updates sent periodically
    streamer.send_budget_update(cpu, mem)?;
    
    // Completion signal
    streamer.send_complete()?;
}
```

## Message Protocol

Messages are sent as newline-delimited JSON over the Unix socket:

### Event Message
```json
{
  "type": "event",
  "event": {
    "event_type": "contract",
    "contract_id": "CABC123...",
    "topics": ["transfer"],
    "data": "100",
    "in_successful_contract_call": true
  }
}
```

### Log Message
```json
{
  "type": "log",
  "message": "Executing contract function..."
}
```

### Budget Update
```json
{
  "type": "budget_update",
  "cpu_instructions": 1000,
  "memory_bytes": 2048
}
```

### Completion
```json
{
  "type": "complete"
}
```

### Error
```json
{
  "type": "error",
  "message": "Contract execution failed: ..."
}
```

## Performance Considerations

- **Socket Overhead**: Minimal (~1-2% for typical simulations)
- **Buffering**: Messages are flushed immediately for real-time delivery
- **Fallback**: If socket creation fails, simulation continues without streaming
- **Cleanup**: Sockets are automatically cleaned up on completion

## Testing

Run the streaming tests:

```bash
go test -v ./internal/simulator -run TestSocket
go test -v ./internal/simulator -run TestStreamHandler
```

## Troubleshooting

### Socket Connection Failed

If the Rust simulator can't connect to the socket:
- Check that the socket path is accessible
- Verify no permission issues in `/tmp`
- Ensure the Go listener started before the simulator

### No Events Received

If events aren't being streamed:
- Verify `EnableStreaming` is set to `true` in the Runner
- Check logs for socket connection messages
- Ensure the simulation actually generates events

### Socket File Not Cleaned Up

If socket files persist in `/tmp`:
- Call `listener.Close()` explicitly
- Use `defer listener.Close()` to ensure cleanup
- Check for panics that might skip cleanup code

## Future Enhancements

- [ ] WebSocket support for remote debugging
- [ ] Event filtering at the socket level
- [ ] Compression for high-volume event streams
- [ ] Replay capability for streamed events
