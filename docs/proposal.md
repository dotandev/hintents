# Proposal 1: Erst - Soroban Error Decoder & Debugger

**Targeting**: Infrastructure / Developer Tooling
**Problem**: "Subpar Error Reporting" is the #1 developer pain point on Stellar.

## Executive Summary
Developers building on Soroban (Stellar's smart contract platform) currently face a "black box" debugging experience. When a transaction fails on mainnet, the error returned is often a generic XDR code (e.g., `HostError`). Tracing *why* it failed requires complex setups, running local standalone nodes, and manually trying to reproduce the state.

`erst` proposes to be the **missing link**: a CLI tool that takes a failed transaction hash, automatically fetches the historical ledger state, and "replays" the transaction locally with verbose debug tracing enabled, mapping errors back to the original Rust source code.

## The Problem in Detail
1.  **Opaque Errors**: `tx_failed` responses often contain binary XDR blobs that, when decoded, result in generic messages like `Error(WasmVm, InvalidAction)`.
2.  **State Dependency**: Contracts fail because of specific state (e.g., "Balance too low", "TTL expired"). Reproducing this locally is hard because developers' local tests don't have the mainnet state.
3.  **Adoption Blocker**: New developers coming from Ethereum (Hardhat/Foundry) expect stack traces. The lack of them on Stellar is a major friction point.

## Proposed Solution: `erst debug <tx-hash>`

The `erst` tool will provide a simple workflow:

1.  **Input**: Developer runs `erst debug 5c0a...` (Transaction Hash).
2.  **Fetch**: `erst` queries a Horizon/RPC archive node to get:
    *   The transaction envelope (inputs, args).
    *   The ledger number it failed in.
    *   The Read/Write set (state keys) required for that transaction.
3.  **Replay**: `erst` spins up a lightweight, embedded `soroban-env-host` instance (via Rust FFI or native Go implementation if feasible, likely Rust FFI).
4.  **Trace**: It executes the transaction against that forked state with `debug_mode=true`.
5.  **Report**: It outputs a human-readable trace:
    ```
    Replaying Transaction 5c0a... locally...
    [INFO] Call Contract A::deposit()
    [INFO]   > Call Contract B::transfer()
    [ERROR]  < Contract B failed: "Insufficient Allowance" at line 45 (token.rs)
    ```

## Value Proposition
*   **For Developers**: Saves hours of debugging time.
*   **For the Ecosystem**: Removes a primary complaint hindering Soroban adoption.
*   **For Maintainers**: Positions `erst` as a critical infrastructure piece, eligible for SCF Infrastructure Grants.
