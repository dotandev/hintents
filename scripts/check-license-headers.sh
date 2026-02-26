# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Check for license headers in Go and Rust files
# Exit with status 1 if any files are missing headers
set -e

MISSING_HEADERS=0
EXPECTED_HEADER="Copyright 2025 Erst Users"

echo "Checking for license headers in Go and Rust files..."

# Check Go files
echo ""
echo "Checking Go files (.go)..."
while IFS= read -r file; do
    if ! head -1 "$file" | grep -q "$EXPECTED_HEADER"; then
        echo "  [FAIL] Missing license header: $file"
        MISSING_HEADERS=$((MISSING_HEADERS + 1))
    else
        echo "  [OK] $file"
    fi
done < <(find . -type d \( -name "target" -o -name "vendor" \) -prune -o -name "*.go" -type f -print)

# Check Rust files
echo ""
echo "Checking Rust files (.rs)..."
while IFS= read -r file; do
    if ! head -1 "$file" | grep -q "$EXPECTED_HEADER"; then
        echo "  [FAIL] Missing license header: $file"
        MISSING_HEADERS=$((MISSING_HEADERS + 1))
    else
        echo "  [OK] $file"
    fi
done < <(find . -type d \( -name "target" -o -name "vendor" \) -prune -o -name "*.rs" -type f -print)

echo ""
if [ $MISSING_HEADERS -eq 0 ]; then
    echo "[OK] All files have proper license headers"
    exit 0
else
    echo "[FAIL] Found $MISSING_HEADERS file(s) missing license headers"
    exit 1
fi
