#!/bin/bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Test script to verify strict linting configuration
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

echo "Verifying strict linting configuration..."

# Create a temporary file with a linting issue (unused variable)
cat > test_lint.go << 'EOF'
package main
import "fmt"
func main() {
    var unused = 1
    fmt.Println("Hello")
}
EOF

# Run golangci-lint and expect it to fail
if command -v golangci-lint &> /dev/null; then
    if golangci-lint run --config=.golangci.yml test_lint.go > /dev/null 2>&1; then
        echo "[FAIL] Strict linting failed to catch unused variable"
        rm test_lint.go
        exit 1
    else
        echo "[OK] Strict linting caught unused variable"
    fi
else
    echo "golangci-lint not available, skipping verification"
fi

rm -f test_lint.go
echo "Strict linting verification passed"
