#!/bin/bash

# Test script for local WASM replay functionality
# This script tests the erst debug --wasm feature

set -e

echo "========================================="
echo "Testing Local WASM Replay Feature"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create a test WASM file
TEST_WASM="/tmp/test_contract.wasm"
echo "Creating test WASM file at $TEST_WASM..."
echo "test wasm content" > "$TEST_WASM"

echo ""
echo "${YELLOW}Test 1: Basic local replay without arguments${NC}"
echo "Command: ./erst debug --wasm $TEST_WASM"
echo "---"
./erst debug --wasm "$TEST_WASM"
echo ""

echo "${YELLOW}Test 2: Local replay with arguments${NC}"
echo "Command: ./erst debug --wasm $TEST_WASM --args hello --args world"
echo "---"
./erst debug --wasm "$TEST_WASM" --args "hello" --args "world"
echo ""

echo "${YELLOW}Test 3: Local replay with verbose output${NC}"
echo "Command: ./erst debug --wasm $TEST_WASM --args test --verbose"
echo "---"
./erst debug --wasm "$TEST_WASM" --args "test" --verbose
echo ""

echo "${YELLOW}Test 4: Error handling - non-existent WASM file${NC}"
echo "Command: ./erst debug --wasm /tmp/nonexistent.wasm"
echo "---"
if ./erst debug --wasm "/tmp/nonexistent.wasm" 2>&1; then
    echo "ERROR: Should have failed with non-existent file"
    exit 1
else
    echo "${GREEN}✓ Correctly handled non-existent file${NC}"
fi
echo ""

# Cleanup
rm -f "$TEST_WASM"

echo "========================================="
echo "${GREEN}All tests passed!${NC}"
echo "========================================="
