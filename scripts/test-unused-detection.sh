#!/bin/bash

# Test script to verify unused code detection setup
# This creates a temporary file with unused code to test the linter

set -e

echo "Testing unused code detection setup..."

# Create a temporary Go file with unused code
cat > test_unused.go << 'EOF'
package main

import "fmt"

// This variable should be detected as unused
var unusedVariable = "test"

// This function should be detected as unused
func unusedFunction() {
    fmt.Println("This function is not used")
}

// This constant should be detected as unused
const unusedConstant = 42

func main() {
    fmt.Println("Hello, World!")
}
EOF

echo "Created test file with unused code..."

# Try to run golangci-lint on the test file
if command -v golangci-lint &> /dev/null; then
    echo "Running golangci-lint on test file..."
    if golangci-lint run --enable unused --disable-all test_unused.go; then
        echo "WARNING: No unused code detected in test file (this might indicate linter issues)"
    else
        echo "SUCCESS: Unused code detected as expected"
    fi
else
    echo "golangci-lint not available, skipping test"
fi

# Clean up
rm -f test_unused.go

echo "Test completed."
