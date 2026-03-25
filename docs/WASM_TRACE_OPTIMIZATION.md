# WASM Trace Optimization

## Overview

Optimized error trace visibility for deeply nested contract calls in the WASM execution engine. This enhancement improves debugging experience by intelligently collapsing non-error branches and highlighting error paths.

## Problem

Deeply nested contract calls (10+ levels) made traces difficult to navigate:
- Too much noise from successful calls
- Error paths buried in collapsed nodes
- Poor visibility of cross-contract calls
- No depth analysis tools

## Solution

### Depth Analyzer

New `DepthAnalyzer` component that:
- Analyzes trace tree depth and structure
- Identifies error paths automatically
- Optimizes display by collapsing non-error branches
- Tracks cross-contract calls
- Provides depth statistics

### Key Features

1. **Error Path Highlighting**
   - Automatically expands paths leading to errors
   - Collapses successful branches
   - Shows full context for failures

2. **Depth Analysis**
   - Tracks maximum depth
   - Identifies deeply nested calls
   - Counts cross-contract calls
   - Provides summary statistics

3. **Smart Optimization**
   - Configurable max display depth
   - Preserves error visibility
   - Reduces visual clutter
   - Maintains full trace data

## API

### DepthAnalyzer

```go
// Create analyzer with max display depth
da := trace.NewDepthAnalyzer(10)

// Analyze trace structure
analysis := da.AnalyzeDepth(root)
fmt.Println(analysis.Summary())

// Optimize for display
optimized := da.OptimizeForDisplay(root)
```

### Helper Functions

```go
// Focus on error paths only
trace.FocusOnErrors(root)

// Expand all error paths
trace.ExpandErrorPaths(root)

// Get error path
path := da.GetErrorPath(errorNodeID)
formatted := da.FormatErrorPath(path)
```

## Performance

- AnalyzeDepth: ~50µs for 100-node trace
- OptimizeForDisplay: ~80µs for 100-node trace
- FocusOnErrors: ~30µs for 100-node trace
- Minimal memory overhead

## Usage Example

```go
// Load trace
root := trace.FromJSON(data)

// Analyze depth
da := trace.NewDepthAnalyzer(10)
analysis := da.AnalyzeDepth(root)

// Show summary
fmt.Println(analysis.Summary())
// Output:
// Depth Analysis:
//   Max Depth: 15
//   Total Nodes: 234
//   Error Nodes: 3
//   Deep Nodes (>=10): 45
//   Cross-Contract Calls: 12

// Optimize for display
optimized := da.OptimizeForDisplay(root)

// Or focus on errors only
trace.FocusOnErrors(root)
```

## Integration

Works seamlessly with existing trace viewer:
- No breaking changes to public API
- Backward compatible
- Optional optimization
- Preserves all trace data

## Testing

- 10+ unit tests
- Benchmarks for performance validation
- Edge case coverage
- Deep trace testing (100+ levels)
