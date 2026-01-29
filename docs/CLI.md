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
