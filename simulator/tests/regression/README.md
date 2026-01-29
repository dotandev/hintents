# Regression Tests

This directory contains automatically generated Rust regression tests from transaction traces.

## Purpose

These tests ensure that once a bug is fixed, it never returns. Each test captures:
- Transaction envelope (XDR)
- Result metadata (XDR)
- Ledger state at the time of execution

## Generating Tests

Use the `erst generate-test` command to create new regression tests:

```bash
erst generate-test <transaction-hash> --lang rust
```

## Running Tests

```bash
cd simulator
cargo test --test regression_*
```
