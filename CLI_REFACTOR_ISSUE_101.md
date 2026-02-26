# CLI Refactoring - Issue #101

## Overview
Standardize the command line architecture for better help text formatting and nested subcommands by organizing commands into logical groups.

## Current State
- Project already uses spf13/cobra v1.7.0
- 38 command files in `internal/cmd/`
- Commands are flat (no grouping)

## Proposed Command Groups

### Core Debugging Commands (GroupID: "core")
- `debug` - Debug a failed Soroban transaction
- `explain` - Summarize why a transaction failed in plain English
- `trace` - Display execution trace with interactive navigation
- `auth-debug` - Debug multi-signature and threshold-based authorization failures

### Testing & Validation Commands (GroupID: "testing")
- `dry-run` - Pre-submission dry run to estimate transaction cost
- `fuzz` - Fuzz test XDR inputs against Soroban contracts
- `regression-test` - Run protocol regression tests against historic transactions
- `generate-test` - Generate regression tests from a transaction
- `compare` - Compare replay: local WASM vs on-chain WASM side-by-side

### Session & Cache Management (GroupID: "management")
- `session` - Manage debugging sessions (list, save, load, delete)
- `cache` - Manage transaction and simulation cache (status, clear, prune)
- `search` - Search saved sessions by criteria

### Development Tools (GroupID: "development")
- `init` - Scaffold a local Erst debugging workspace
- `doctor` - Diagnose development environment setup
- `shell` - Interactive REPL for contract testing
- `daemon` - Start JSON-RPC server for remote debugging
- `wizard` - Interactive setup wizard

### Utility Commands (GroupID: "utility")
- `xdr` - Decode and inspect XDR data structures
- `abi` - Decompile and display a Soroban contract ABI
- `version` - Display version information
- `completion` - Generate completion script for your shell
- `upgrade` - Upgrade contract code
- `export` - Export data from the current session
- `report` - Generate detailed analysis reports
- `profile` - Export trace as pprof profile for gas-to-function mapping
- `stats` - Display statistics about sessions and cache
- `rpc` - RPC client utilities

## Implementation Plan

1. [OK] Add command groups to root.go
2. [OK] Update debug.go to use GroupID
3. Update remaining commands to assign GroupID
4. Test help output formatting
5. Update documentation

## Benefits

- Better discoverability of commands
- Logical organization in help text
- Easier for new users to find relevant commands
- Maintains backward compatibility (all commands still work the same)

## Testing

```bash
# Test help output
erst --help

# Test individual command groups
erst debug --help
erst session --help
erst cache --help
```
