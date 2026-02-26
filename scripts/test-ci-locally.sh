# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

# Test CI checks locally before pushing

set -e

echo "Running CI checks locally..."
echo ""

# Go checks
echo "Go: Verifying dependencies..."
go mod verify

echo "Go: Checking formatting..."
if [ -n "$(gofmt -l .)" ]; then
  echo "[FAIL] Go files are not formatted. Run 'go fmt ./...' to fix."
  gofmt -d .
  exit 1
fi
echo "[OK] Go files are properly formatted"

echo "Go: Running go vet..."
go vet ./...

echo "Go: Running tests..."
go test -v -race ./...

echo "Go: Building..."
go build -v ./...

# Rust checks
echo ""
echo "Rust: Checking formatting..."
cd simulator
if ! cargo fmt --check; then
  echo "[FAIL] Rust files are not formatted. Run 'cargo fmt' to fix."
  exit 1
fi
echo "[OK] Rust files are properly formatted"

echo "Rust: Running Clippy..."
cargo clippy --all-targets --all-features -- -D warnings

echo "Rust: Running tests..."
cargo test --verbose

echo "Rust: Building..."
cargo build --verbose

cd ..

echo ""
echo "[OK] All CI checks passed! Safe to push."
