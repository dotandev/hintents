# WASM Local Variables Display in VS Code Debug Hover

## Overview

This feature extends the ERST VS Code extension to display WASM local variables during debug sessions. Users can now hover over source code or expand the "WASM Locals" scope in the debug panel to inspect local variable values at each trace step.

## Implementation Details

### 1. Data Structure Extensions

#### TraceStep Interface (`erstClient.ts`)
Extended the `TraceStep` interface to include WASM-specific data:

```typescript
export interface WasmLocal {
    name: string;           // Variable name
    type: string;          // Type information (i32, i64, struct, etc.)
    location: string;      // Where the variable is stored (local[0], memory[256:512], etc.)
    value?: any;           // Current value
    startLine?: number;    // Source line where variable is in scope
    endLine?: number;      // Source line where variable goes out of scope
}

export interface TraceStep {
    // ... existing fields ...
    wasm_locals?: WasmLocal[];  // Array of local variables at this step
    wasm_offset?: number;       // Instruction offset for correlation
}
```

### 2. WASM Locals Extractor Module

Created `wasmLocalsExtractor.ts` to manage extraction and caching of local variables:

**Key Features:**
- Detects DWARF debug symbols in WASM binaries
- Caches extracted locals by instruction offset
- Extracts locals from trace step data
- Provides source information for hover tooltips

**Class: WasmLocalsExtractor**
- `setWasmData(wasmBytes)` - Sets WASM binary data
- `hasDebugInfo()` - Checks if DWARF symbols are present
- `extractLocalsAtInstruction(instruction, contractId, functionName)` - Extracts locals at a specific instruction
- `extractLocalsFromStepData(stepData)` - Extracts locals from trace step data
- `clearCache()` - Clears the extraction cache

### 3. DAP Adapter Integration

Modified `adapter.ts` to expose WASM locals through the Debug Adapter Protocol:

#### Scopes Handler Enhancement
Added "WASM Locals" scope that appears alongside existing scopes:

```
Scopes presented to VS Code:
├── Locals (operation details)
├── WASM Locals (if wasm_locals data is present)
├── Arguments (if present)
├── Host State (if present)
├── Memory (if present)
└── Budget (CPU/memory usage)
```

#### Variable Display
- WASM locals are displayed with format: `localName: type`
- Nested objects are expandable
- Values are cached for efficient repeated access
- Special handling in `formatValue()` to distinguish WasmLocal objects from regular values

#### Helper Methods
- `extractWasmLocals(step)` - Extracts WASM locals from trace step
- Updated `getChildVariables()` - Handles WasmLocal array items specially
- Updated `formatValue()` - Formats WasmLocal values with type information

## Usage

### For Developers Using the Debugger

1. Start a debug session with a transaction that has WASM debug symbols
2. Navigate through trace steps using the debugger
3. Look for the "WASM Locals" scope in the Variables panel
4. Expand individual locals to see their values
5. Hover over variables in the source view to see type information

### For Backend/Simulator Integration

The simulator should populate `TraceStep.wasm_locals` with data extracted from DWARF debug information:

```typescript
const step: TraceStep = {
    step: 42,
    operation: 'LocalSet',
    timestamp: '2024-01-01T00:00:00Z',
    contract_id: 'CA7QM...',
    wasm_locals: [
        {
            name: 'count',
            type: 'i32',
            location: 'local[0]',
            value: 42,
            startLine: 15,
            endLine: 25
        },
        {
            name: 'max_val',
            type: 'i32',
            location: 'local[1]',
            value: 1000
        }
    ],
    wasm_offset: 12345
};
```

## Architecture

```
┌─────────────────────┐
│   ERST Simulator    │
│  (Go/Rust)          │
└──────────┬──────────┘
           │ Populates TraceStep.wasm_locals
           │ using DWARF parsing
           ▼
┌─────────────────────────────────────┐
│   erstClient.ts                     │
│   - WasmLocal interface             │
│   - Extended TraceStep interface    │
└──────────┬──────────────────────────┘
           │
           ▼
┌─────────────────────────────────────┐
│   ERSTDebugSession (adapter.ts)     │
│   - extractWasmLocals()             │
│   - handleScopes() - includes       │
│     WASM Locals scope               │
│   - getChildVariables() - handles   │
│     WasmLocal objects               │
└──────────┬──────────────────────────┘
           │ DAP Protocol
           ▼
┌─────────────────────┐
│   VS Code Debugger  │
│   - Variables Panel │
│   - Hover Tooltips  │
└─────────────────────┘
```

## File Changes

### New Files
- `erst_extension/src/dap/wasmLocalsExtractor.ts` - WASM locals extraction module
- `erst_extension/src/dap/wasmLocalsExtractor.test.ts` - Unit tests
- `erst_extension/src/dap/adapter.test.ts` - Integration tests

### Modified Files
- `erst_extension/src/erstClient.ts` - Extended TraceStep and added WasmLocal interfaces
- `erst_extension/src/dap/adapter.ts` - Integrated WASM locals into DAP adapter

## Testing

### Unit Tests
- `WasmLocalsExtractor` initialization and configuration
- Debug symbol detection
- Locals extraction from step data
- Caching behavior
- Edge cases and error handling

### Integration Tests
- `TraceStep` backward compatibility
- WASM locals scope presentation
- Hover functionality
- Nested object expansion
- Complex type handling

### Manual Testing
1. Start a debug session with a WASM contract that has debug symbols
2. Set breakpoints and step through the trace
3. Verify "WASM Locals" scope appears in Variables panel
4. Expand locals and verify values are displayed correctly
5. Hover over variable names in source code to see tooltips

## Future Enhancements

1. **Full DWARF Parsing** - Currently uses placeholder parsing; implement complete DWARF debug info extraction
2. **Source Code Mapping** - Map locals to specific source lines for precise hover positions
3. **Type Resolution** - Resolve complex types like generics, enums, and structs
4. **Memory Inspection** - Drill down into memory-mapped locals with hex viewer
5. **Watch Expressions** - Allow users to add custom watch expressions for locals
6. **Performance** - Optimize caching for large traces with thousands of steps

## Backward Compatibility

All changes are backward compatible:
- `TraceStep` fields are optional
- Existing adapter functionality is unchanged
- If `wasm_locals` is not provided, the scope simply doesn't appear
- No breaking changes to the DAP protocol

## Documentation References

- [ERST Architecture](../docs/architecture.md)
- [DWARF Debug Symbols](../docs/debug-symbols-guide.md)
- [Source Mapping](../docs/source-mapping.md)
- [DAP Specification](https://microsoft.github.io/debug-adapter-protocol/)
