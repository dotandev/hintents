# Local WASM Replay Feature

## Overview

The local WASM replay feature allows developers to test and debug Stellar smart contracts locally without needing to deploy to any network or fetch mainnet data. This is particularly useful during rapid contract development.

## Usage

### Basic Local Replay

```bash
erst debug --wasm ./contract.wasm
```

### With Arguments

```bash
erst debug --wasm ./contract.wasm --args "arg1" --args "arg2" --args "arg3"
```

### With Verbose Output

```bash
erst debug --wasm ./contract.wasm --args "hello" --verbose
```

## Features

-  Load WASM files from local filesystem
-  Mock state provider (no network data required)
-  Support for mock arguments (Integer and Symbol/String)
-  Diagnostic logging and event capture
-  Clear warnings about mock state usage
-  Full WASM execution

## Warning

When using the `--wasm` flag, the execution uses **Mock State** and not mainnet data. This is clearly indicated in the output:

```
[WARN]  WARNING: Using Mock State (not mainnet data)
```

This mode is intended for:
- Rapid contract development
- Local testing before deployment
- Debugging contract logic without network overhead

## Architecture

### CLI Layer (Go)
- `internal/cmd/debug.go`: Handles the `--wasm` flag and coordinates local replay
- `internal/simulator/schema.go`: Extended to support `wasm_path` and `mock_args`

### Simulator Layer (Rust)
- `simulator/src/main.rs`: Contains `run_local_wasm_replay()` function
- Loads WASM files from disk
- Initializes Soroban Host
- Deploys contract to host
- Parses arguments (supports Integers and Symbols)
- Invokes contract function
- Captures diagnostic events and logs

### Soroban Compatibility & Determinism
- The simulator enforces Soroban VM compatibility by rejecting WASM binaries
  that contain floating-point instructions. This prevents non-deterministic
  execution traces during local replay and keeps behavior aligned with on-chain
  Soroban restrictions.

## Example Output

```
[WARN]  WARNING: Using Mock State (not mainnet data)

[TOOL] Local WASM Replay Mode
WASM File: ./contract.wasm
Arguments: [hello world]

[OK] Initialized Host with diagnostic level: Debug
[OK] Contract registered at: Contract(0000...)
▶ Invoking function: hello

[OK] Execution successful

[LIST] Logs:
  Host Budget: [budget details]
  Result: Symbol(world)
```

## Future Enhancements

1. **Complex Types**: Support for parsing more complex ScVal types (Maps, Vectors, etc.) via JSON
2. **State Persistence**: Allow saving/loading mock state
3. **Interactive Mode**: REPL-like interface for invoking multiple functions

## Testing

To test the feature:

```bash
# Create a simple test WASM file
echo "test" > /tmp/test.wasm

# Run local replay
./erst debug --wasm /tmp/test.wasm --args "test1" --args "test2"
```

