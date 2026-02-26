# CLI Reference

This document provides a reference for the `erst` command-line interface.

## erst

Erst is a specialized developer tool for the Stellar network, designed to solve the "black box" debugging experience on Soroban.

### Synopsis

Erst is a Soroban Error Decoder & Debugger.

```bash
erst [command]
```

### Options

```
  -h, --help   help for erst
```

---

## erst generate-bindings

Generate TypeScript bindings for a Soroban smart contract. Creates strongly-typed client libraries with erst integration.

### Usage

```bash
erst generate-bindings <wasm-file> [flags]
```

### Examples

```bash
# Basic usage
erst generate-bindings contract.wasm

# With custom output and package name
erst generate-bindings contract.wasm --output ./src/bindings --package my-contract

# With contract ID and network
erst generate-bindings contract.wasm \
  --contract-id CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2 \
  --network testnet
```

### Options

```
  -h, --help                help for generate-bindings
  -o, --output string       Output directory (defaults to current directory)
  -p, --package string      Package name (defaults to WASM filename)
      --contract-id string  Contract ID for network calls
  -n, --network string      Stellar network (testnet, mainnet, futurenet) (default "testnet")
```

### Arguments

| Argument | Description |
| :--- | :--- |
| `<wasm-file>` | Path to the compiled Soroban contract WASM file |

---

## erst debug

Debug a failed Soroban transaction. Fetches a transaction envelope from the Stellar network and prepares it for simulation.

### Usage

```bash
erst debug <transaction-hash> [flags]
```

### Examples

```bash
erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
erst debug --network testnet <tx-hash>
```

### Options

```
  -h, --help             help for debug
  -n, --network string   Stellar network to use (testnet, mainnet, futurenet) (default "mainnet")
      --rpc-url string   Custom Horizon RPC URL to use
```

### Arguments

| Argument | Description |
| :--- | :--- |
| `<transaction-hash>` | The hash of the transaction to debug. |

### Interrupt and Shutdown Behavior

When `erst` receives `Ctrl+C` (`SIGINT`) or `SIGTERM` during polling or simulation:

- Active command execution is canceled immediately.
- Shutdown hooks run once in deterministic order.
- RPC cache flush is attempted as best-effort.
- Active `erst-sim` child process groups are terminated gracefully (`SIGTERM`), then force-killed (`SIGKILL`) if they do not exit within the grace timeout.
- The CLI exits with code `130`.

Repeated interrupt handling:

- First interrupt starts graceful shutdown.
- A second interrupt during shutdown forces immediate exit with code `130`.

---

## erst generate-test

Generate regression tests from a recorded transaction trace. This creates test files that can be used to ensure bugs don't reoccur.

### Usage

```bash
erst generate-test <transaction-hash> [flags]
```

### Examples

```bash
# Generate both Go and Rust tests
erst generate-test 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab

# Generate only Go tests
erst generate-test --lang go <tx-hash>

# Generate with custom test name
erst generate-test --name my_regression_test <tx-hash>
```

### Options

```
  -h, --help             help for generate-test
  -l, --lang string      Target language (go, rust, or both) (default "both")
  -n, --network string   Stellar network to use (testnet, mainnet, futurenet) (default "mainnet")
      --name string      Custom test name (defaults to transaction hash)
  -o, --output string    Output directory (defaults to current directory)
      --rpc-url string   Custom Horizon RPC URL to use
```

### Arguments

| Argument | Description |
| :--- | :--- |
| `<transaction-hash>` | The hash of the transaction to generate tests from. |

### Output

Generated tests are written to:
- **Go tests**: `internal/simulator/regression_tests/regression_<name>_test.go`
- **Rust tests**: `simulator/tests/regression/regression_<name>.rs`
