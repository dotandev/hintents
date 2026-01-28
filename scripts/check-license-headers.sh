#!/bin/bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Check for license headers in Go and Rust files
# Exit with status 1 if any files are missing headers
set -e

MISSING_HEADERS=0
EXPECTED_HEADER="Copyright 2025 Erst Users"

echo "🔍 Checking for license headers in Go and Rust files..."

# Check Go files
echo ""
echo "Checking Go files (.go)..."
while IFS= read -r file; do
    if ! head -1 "$file" | grep -q "$EXPECTED_HEADER"; then
        echo "  ❌ Missing license header: $file"
        MISSING_HEADERS=$((MISSING_HEADERS + 1))
    else
        echo "  ✅ $file"
    fi
done < <(find . -name "*.go" -type f)

# Check Rust files
echo ""
echo "Checking Rust files (.rs)..."
while IFS= read -r file; do
    if ! head -1 "$file" | grep -q "$EXPECTED_HEADER"; then
        echo "  ❌ Missing license header: $file"
        MISSING_HEADERS=$((MISSING_HEADERS + 1))
    else
        echo "  ✅ $file"
    fi
done < <(find . -name "*.rs" -type f)

echo ""
if [ $MISSING_HEADERS -eq 0 ]; then
    echo "✅ All files have proper license headers"
    exit 0
else
    echo "❌ Found $MISSING_HEADERS file(s) missing license headers"
    exit 1
fi