# Verbose Debug Mode

## Overview

The `--verbose` flag provides detailed technical information during debug operations for the ERST/Hintents CLI.

## Usage

### Standard Mode (Default)

```bash
erst debug <transaction_hash>
```

**Output:** Clean, minimal information suitable for regular users.

### Verbose Mode

```bash
erst debug <transaction_hash> --verbose
```

**Output:** Detailed technical information including RPC calls, payload sizes, timing, and performance metrics.

## What's Logged in Verbose Mode

### RPC Operations
- Full request URLs
- HTTP methods
- Request payload sizes
- Response status codes
- Response payload sizes
- Request duration

### Data Processing
- Parsing steps
- Field extraction (Ledger, Source Account)

### Performance Metrics
- Total execution time
- Memory usage (heap used)
- Peak memory (heap total)

## Examples

### Standard Output

```
🔍 Debugging transaction: a1b2c3d4...
✅ Transaction fetched successfully
📊 Operations: 3
✅ Debug complete
```

### Verbose Output

```
🔍 Debugging transaction: a1b2c3d4e5f6...

[00:00.123] [INFO] Configuration
[00:00.124] [INFO]   RPC URL: https://horizon-testnet.stellar.org
[00:00.125] [INFO]   Transaction hash: a1b2c3d4e5f6...
[00:00.126] [INFO]   Verbose mode: enabled

[00:00.127] [RPC] Initiating transaction fetch...
[00:00.128] [RPC] → GET /transactions/a1b2c3d4e5f6...
[00:00.129] [RPC]   Endpoint: https://horizon-testnet.stellar.org
[00:00.130] [RPC]   Request size: 0 bytes
[00:00.345] [RPC] ← Response received (212ms)
[00:00.346] [RPC]   Status: 200 OK
[00:00.347] [RPC]   Response size: 15.2 KB

[00:00.348] [DATA] Parsing transaction response...
[00:00.349] [DATA]   Ledger: 12345678
[00:00.350] [DATA]   Source: GABC...XYZ

✅ Transaction fetched successfully
📊 Operations: 3

[00:00.506] [PERF] Performance metrics
[00:00.507] [PERF]   Total execution time: 507ms
[00:00.508] [PERF]   Memory usage: 24.5 MB
[00:00.509] [PERF]   Peak memory: 28.2 MB

✅ Debug complete
```

## Log Categories

| Category | Description | Color |
|----------|-------------|-------|
| `RPC` | RPC/HTTP operations | Blue |
| `DATA` | Data parsing/processing | Cyan |
| `SIM` | Simulation steps | Magenta |
| `PERF` | Performance metrics | Yellow |
| `ERROR` | Error messages | Red |
| `INFO` | General information | White |

## Troubleshooting

### Verbose logs not showing
Ensure you are using the `--verbose` flag correctly: `erst debug <tx> --verbose`.

### Request fails with 400
The transaction ID might be invalid for the selected network (e.g., testnet vs mainnet). Use `--rpc` to specify the correct endpoint.
