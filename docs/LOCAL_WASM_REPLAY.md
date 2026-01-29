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

- ✅ Load WASM files from local filesystem
- ✅ Mock state provider (no network data required)
- ✅ Support for mock arguments
- ✅ Diagnostic logging and event capture
- ✅ Clear warnings about mock state usage
- ⚠️ Full WASM execution (coming soon)

## Warning

When using the `--wasm` flag, the execution uses **Mock State** and not mainnet data. This is clearly indicated in the output:

```
⚠️  WARNING: Using Mock State (not mainnet data)
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
- Initializes Soroban Host with mock state
- Captures diagnostic events and logs

## Example Output

```
⚠️  WARNING: Using Mock State (not mainnet data)

🔧 Local WASM Replay Mode
WASM File: ./contract.wasm
Arguments: [hello world]

▶ Executing contract locally...

✓ Execution completed successfully

📋 Logs:
  Host Initialized with Budget: [budget details]
  WASM file loaded: 1234 bytes
  Mock State Provider: Active
  Mock Arguments provided: 2 args
    Arg[0]: hello
    Arg[1]: world
```

## Future Enhancements

1. **Full WASM Execution**: Currently, the infrastructure is in place but full contract execution is not yet implemented
2. **Contract Deployment**: Automatically deploy the WASM to the mock host
3. **Argument Parsing**: Parse string arguments into proper Soroban ScVal types
4. **Function Invocation**: Call specific contract functions with parsed arguments
5. **Result Capture**: Return actual execution results and contract state changes

## Testing

To test the feature:

```bash
# Create a simple test WASM file
echo "test" > /tmp/test.wasm

# Run local replay
./erst debug --wasm /tmp/test.wasm --args "test1" --args "test2"
```


