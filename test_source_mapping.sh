#!/bin/bash

# Test script for source mapping functionality
# This script tests the simulator with a minimal valid request

echo "Testing source mapping functionality..."

# Create a minimal test request
TEST_REQUEST='{"envelope_xdr":"","result_meta_xdr":"","ledger_entries":{},"contract_wasm":"AGFzbQEAAAABBAFgAAADAgEABQMBAAEGCAF/AEGAgAQLBwkBBWhlbGxvAAAKBAECAAv="}'

# Run the simulator
echo "Running simulator with test request..."
cd /workspaces/hintents/simulator
echo "$TEST_REQUEST" | cargo run --bin erst-sim

echo "Test completed!"
