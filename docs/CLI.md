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
