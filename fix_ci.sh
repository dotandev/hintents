# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash
# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

# Script to fix CI/CD failures related to go.mod/go.sum

set -e

echo "=== Fixing CI/CD Issues ==="
echo

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "âŒ Go is not installed"
    echo
    echo "Please install Go to fix the CI issues:"
    echo "  - Ubuntu/Debian: sudo apt install golang-go"
    echo "  - macOS: brew install go"
    echo "  - Or download from: https://golang.org/dl/"
    echo
    echo "After installing Go, run this script again."
    exit 1
fi

echo "âœ“ Go is installed: $(go version)"
echo

# Run go mod tidy
echo "Running go mod tidy..."
if go mod tidy; then
    echo "âœ“ go mod tidy completed successfully"
else
    echo "âŒ go mod tidy failed"
    exit 1
fi
echo

# Verify dependencies
echo "Verifying dependencies..."
if go mod verify; then
    echo "âœ“ Dependencies verified successfully"
else
    echo "âŒ Dependency verification failed"
    exit 1
fi
echo

# Check if go.sum was updated
if git diff --quiet go.sum; then
    echo "âš ï¸  go.sum was not modified"
    echo "   This might mean it was already up to date"
else
    echo "âœ“ go.sum was updated"
fi
echo

# Run tests
echo "Running metrics package tests..."
if go test ./internal/metrics -v; then
    echo "âœ“ Tests passed"
else
    echo "âŒ Tests failed"
    exit 1
fi
echo

# Check formatting
echo "Checking code formatting..."
UNFORMATTED=$(gofmt -l internal/metrics/)
if [ -z "$UNFORMATTED" ]; then
    echo "âœ“ All files are properly formatted"
else
    echo "âš ï¸  The following files need formatting:"
    echo "$UNFORMATTED"
    echo
    echo "Run: gofmt -w internal/metrics/"
fi
echo

# Run go vet
echo "Running go vet..."
if go vet ./internal/metrics/...; then
    echo "âœ“ go vet passed"
else
    echo "âŒ go vet found issues"
    exit 1
fi
echo

# Show what changed
echo "=== Changes Made ==="
echo
echo "Modified files:"
git status --short go.mod go.sum
echo

# Show summary
echo "=== Summary ==="
echo
echo "âœ“ go.mod and go.sum are now in sync"
echo "âœ“ All dependencies verified"
echo "âœ“ Tests pass"
echo "âœ“ Code is properly formatted"
echo "âœ“ No vet issues"
echo
echo "Next steps:"
echo "1. Review the changes: git diff go.mod go.sum"
echo "2. Commit the changes:"
echo "   git add go.mod go.sum"
echo "   git commit -m 'fix(deps): update go.sum for prometheus dependency'"
echo "3. Push to trigger CI: git push"
echo
echo "The CI should now pass! âœ¨"
