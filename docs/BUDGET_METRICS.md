# Budget Consumption Metrics in Trace Tree

## Overview

The trace tree now displays inline CPU and memory delta metrics beside each function call node, providing immediate visibility into resource consumption at each step of contract execution.

## Features

### Inline Budget Display

Each trace node can now track and display:
- **CPU Delta**: Number of CPU instructions consumed by that specific operation
- **Memory Delta**: Number of memory bytes consumed by that specific operation

### Visual Representation

Budget metrics are displayed inline in the trace tree:

```
0: contract_init (initialize)
1: contract_call (mint) [CPU: 125.00K, Mem: 2.00KB]
2: contract_call (transfer) [CPU: 180.00K, Mem: 3.00KB]
3: balance_check (get_balance) [CPU: 75.00K, Mem: 1.00KB]
```

### Format Specifications

**CPU Instructions:**
- Values >= 1,000,000: Displayed as `X.XXM` (e.g., `1.25M`)
- Values >= 1,000: Displayed as `X.XXK` (e.g., `125.00K`)
- Values < 1,000: Displayed as raw number

**Memory Bytes:**
- Values >= 1,048,576: Displayed as `X.XXMB` (e.g., `2.50MB`)
- Values >= 1,024: Displayed as `X.XXKB` (e.g., `3.00KB`)
- Values < 1,024: Displayed as `XB` (e.g., `512B`)

## Data Structure

### Go (TraceNode)

```go
type TraceNode struct {
    // ... existing fields ...
    CPUDelta    *uint64  // CPU instructions consumed (nil if not tracked)
    MemoryDelta *uint64  // Memory bytes consumed (nil if not tracked)
}
```

### TypeScript (TraceStep)

```typescript
export interface TraceStep {
    // ... existing fields ...
    cpu_delta?: number;      // CPU instructions consumed
    memory_delta?: number;   // Memory bytes consumed
}
```

## Usage

### In VSCode Extension

Budget metrics are automatically displayed in the trace tree view when available:

```typescript
export class TraceItem extends vscode.TreeItem {
    constructor(public readonly step: TraceStep) {
        // Budget info is formatted and displayed in description
        const budgetParts: string[] = [];
        if (step.cpu_delta !== undefined && step.cpu_delta > 0) {
            budgetParts.push(`CPU: ${this.formatNumber(step.cpu_delta)}`);
        }
        if (step.memory_delta !== undefined && step.memory_delta > 0) {
            budgetParts.push(`Mem: ${this.formatBytes(step.memory_delta)}`);
        }
        const budgetInfo = budgetParts.length > 0 ? ` [${budgetParts.join(', ')}]` : '';
        this.description = step.error ? `Error: ${step.error}` : budgetInfo;
    }
}
```

### In Mock Traces

Sample budget metrics can be added to test traces:

```go
call := NewTraceNode("call-1", "contract_call")
cpuDelta := uint64(150000)
memDelta := uint64(2048)
call.CPUDelta = &cpuDelta
call.MemoryDelta = &memDelta
```

## Benefits

1. **Immediate Visibility**: See resource consumption at a glance without drilling into details
2. **Performance Analysis**: Quickly identify expensive operations in the call tree
3. **Budget Planning**: Understand cumulative costs across nested function calls
4. **Optimization**: Spot optimization opportunities by comparing delta values

## Implementation Details

### Optional Fields

Budget metrics are optional (`*uint64` in Go, `number?` in TypeScript) to:
- Support traces without budget tracking
- Allow backward compatibility
- Reduce memory overhead for nodes that don't need tracking

### Zero Values

A value of `0` is treated as "no consumption" and not displayed, keeping the UI clean for operations with negligible resource usage.

### Nested Consumption

Budget deltas represent the consumption of that specific node only, not cumulative consumption including children. This allows developers to:
- See exact costs per operation
- Calculate total costs by summing deltas
- Identify which level of nesting contributes most to consumption

## Testing

Tests are provided in `internal/trace/budget_test.go`:

```bash
go test ./internal/trace/... -v -run TestBudget
```

## Future Enhancements

Potential improvements:
- Cumulative budget display (node + all children)
- Percentage of total budget consumed
- Budget limit warnings
- Color coding for high consumption nodes
- Budget comparison between different executions
