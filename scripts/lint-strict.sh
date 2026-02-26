# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/usr/bin/env bash
# Copyright (c) 2025 ERST Contributors
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

echo "=========================================="
echo "Running Strict Linting Pipeline"
echo "=========================================="

EXIT_CODE=0

# Go linting
echo ""
echo "1. Running Go strict linting..."
echo "------------------------------------------"

if ! golangci-lint run --config=.golangci.yml --max-issues-per-linter=0 --max-same-issues=0; then
    echo "[FAIL] golangci-lint found issues"
    EXIT_CODE=1
else
    echo " golangci-lint passed"
fi

if ! go vet ./...; then
    echo "[FAIL] go vet found issues"
    EXIT_CODE=1
else
    echo " go vet passed"
fi

# Check for unused variables specifically
echo ""
echo "2. Checking for unused variables..."
echo "------------------------------------------"

if go vet ./... 2>&1 | grep -i "declared and not used\|unused variable\|unused parameter"; then
    echo "[FAIL] Unused variables detected"
    EXIT_CODE=1
else
    echo " No unused variables detected"
fi

# Rust linting
echo ""
echo "3. Running Rust strict linting..."
echo "------------------------------------------"

cd simulator

if ! cargo clippy --all-targets --all-features -- \
    -D warnings \
    -D clippy::all \
    -D unused-variables \
    -D unused-imports \
    -D unused-mut \
    -D dead-code \
    -D unused-assignments \
    -W clippy::pedantic \
    -W clippy::nursery; then
    echo "[FAIL] Clippy found issues"
    EXIT_CODE=1
else
    echo " Clippy passed"
fi

cd ..

# Summary
echo ""
echo "=========================================="
if [ $EXIT_CODE -eq 0 ]; then
    echo " All strict linting checks passed!"
else
    echo " Linting failed - please fix the issues above"
fi
echo "=========================================="

exit $EXIT_CODE
