# Bi-Directional Trace Navigation

The ERST trace navigation system provides interactive debugging with the ability to step backward through execution traces, enabling developers to examine previous states and understand how errors occurred.

## Features

### Bi-Directional Navigation
- **Step Forward**: Move to the next execution step
- **Step Backward**: Move to the previous execution step  
- **Jump to Step**: Navigate directly to any specific step
- **State Reconstruction**: View complete state at any point in time

### Memory-Efficient Snapshotting
- **Automatic Snapshots**: Creates state snapshots at configurable intervals
- **Incremental Updates**: Only stores state changes between steps
- **Efficient Reconstruction**: Uses nearest snapshot + incremental changes

### Interactive Viewer
- **Terminal UI**: Responsive command-line interface
- **Real-time Navigation**: Instant step-by-step debugging
- **State Inspection**: View memory, host state, and execution context

## Usage

### Generate Trace Files

```bash
# Debug with trace generation
./erst debug --generate-trace --trace-output my_trace.json <tx-hash>

# Generate sample trace for testing
go run test/generate_sample_trace.go sample.json
```

### Interactive Navigation

```bash
# Launch interactive viewer
./erst trace sample.json
```

### Navigation Commands

```
Navigation:
  n, next, forward     - Step forward
  p, prev, back        - Step backward  
  j, jump <step>       - Jump to specific step

Display:
  s, show, state       - Show current state
  r, reconstruct [step] - Reconstruct state
  l, list [count]      - List steps (default: 10)
  i, info              - Show navigation info

Other:
  h, help              - Show help
  q, quit, exit        - Exit viewer
```

## Example Session

```
🔍 ERST Interactive Trace Viewer
=================================
Transaction: sample-tx-hash-12345
Total Steps: 7

📍 Current State
================
Step: 0/6
Time: 15:04:05.123
Operation: contract_init
Contract: CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2
Function: initialize

> n
➡️  Stepped forward to step 1

📍 Current State  
================
Step: 1/6
Operation: contract_call
Function: mint
Return: true

> p
⬅️  Stepped backward to step 0

> j 3
🎯 Jumped to step 3

> r
🔧 Reconstructed State at Step 3
==================================
Host State:
  balance: 400000
  total_supply: 500000
Memory:
  from_balance: 500000
  to_balance: 100000
```

## Technical Implementation

### State Snapshots
```go
type StateSnapshot struct {
    Step      int                    `json:"step"`
    Timestamp time.Time              `json:"timestamp"`
    HostState map[string]interface{} `json:"host_state"`
    Memory    map[string]interface{} `json:"memory"`
    CallStack []string               `json:"call_stack"`
}
```

### Execution States
```go
type ExecutionState struct {
    Step        int                    `json:"step"`
    Operation   string                 `json:"operation"`
    ContractID  string                 `json:"contract_id,omitempty"`
    Function    string                 `json:"function,omitempty"`
    Arguments   []interface{}          `json:"arguments,omitempty"`
    ReturnValue interface{}            `json:"return_value,omitempty"`
    Error       string                 `json:"error,omitempty"`
    HostState   map[string]interface{} `json:"host_state,omitempty"`
    Memory      map[string]interface{} `json:"memory,omitempty"`
}
```

### Configuration
- **Snapshot Interval**: Configurable (default: every 5 steps)
- **Memory Efficiency**: Only stores state changes, not full state
- **JSON Serialization**: Traces can be saved/loaded from files

## Performance Characteristics

- **Memory Usage**: O(n + s) where n = steps, s = snapshots
- **Navigation Speed**: O(1) for forward/backward, O(k) for reconstruction where k = steps since last snapshot
- **Snapshot Overhead**: Minimal - only creates snapshots at intervals
- **Reconstruction Time**: Fast due to incremental state application

## Integration

The trace navigation system integrates with:
- **Debug Command**: `--generate-trace` flag
- **Simulator**: Automatic trace generation during execution
- **JSON-RPC**: Trace data available via API
- **OpenTelemetry**: Distributed tracing correlation

## Use Cases

1. **Error Investigation**: Step back from error to see what led to failure
2. **State Analysis**: Examine memory/storage changes over time
3. **Performance Debugging**: Identify expensive operations
4. **Contract Auditing**: Verify execution flow and state transitions
5. **Educational**: Understand smart contract execution step-by-step
