# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

# Integration test for source mapping functionality
# Tests the complete flow from Go CLI to Rust simulator

set -e

echo "Running source mapping integration test..."

# Build the simulator
echo "Building simulator..."
cd simulator
cargo build --release
cd ..

# Test 1: Request without WASM (should work normally)
echo "Test 1: Request without contract WASM"
TEST_REQUEST='{"envelope_xdr":"","result_meta_xdr":"","ledger_entries":{}}'
RESULT=$(echo "$TEST_REQUEST" | ./simulator/target/release/erst-sim)
echo "Result: $RESULT"

# Test 2: Request with WASM but no debug symbols
echo "Test 2: Request with WASM (no debug symbols)"
TEST_REQUEST='{"envelope_xdr":"","result_meta_xdr":"","ledger_entries":{},"contract_wasm":"AGFzbQEAAAABBAFgAAADAgEABQMBAAEGCAF/AEGAgAQLBwkBBWhlbGxvAAAKBAECAAv="}'
RESULT=$(echo "$TEST_REQUEST" | ./simulator/target/release/erst-sim)
echo "Result: $RESULT"

# Test 3: Invalid JSON
echo "Test 3: Invalid JSON"
RESULT=$(echo "invalid json" | ./simulator/target/release/erst-sim)
echo "Result: $RESULT"

echo "Integration tests completed successfully!"
