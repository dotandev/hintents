#!/bin/bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Test CI checks locally before pushing
set -euo pipefail

# Ensure we are in the project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." &>/dev/null && pwd)"
cd "${REPO_ROOT}" || { echo "Failed to change directory to project root: ${REPO_ROOT}"; exit 1; }

echo "Running CI checks locally..."
echo "Project root: ${REPO_ROOT}"
echo ""

# Check for required tools
for tool in go gofmt cargo; do
  if ! command -v "$tool" &>/dev/null; then
    echo "[ERROR] $tool is not installed or not in PATH"
    exit 1
  fi
done

# Go checks
echo "Go: Verifying dependencies..."
go mod verify

echo "Go: Checking formatting (excluding simulator/target and .gocache)..."
# Use find to exclude directories and run gofmt
FORMAT_ISSUES=$(find . -name "*.go" -not -path "./simulator/target/*" -not -path "./.gocache/*" -not -path "./vendor/*" -exec gofmt -l {} +)
if [ -n "${FORMAT_ISSUES}" ]; then
  echo "[FAIL] The following Go files are not formatted:"
  echo "${FORMAT_ISSUES}"
  echo "Run 'go fmt ./...' to fix."
  exit 1
fi
echo "[OK] Go files are properly formatted"

echo "Go: Running go vet..."
go vet ./...

echo "Go: Building..."
go build -v ./...

echo "Go: Building erst binary for integration tests..."
go build -o erst ./cmd/erst

echo "Go: Running tests..."
go test -v -race ./...

# Rust checks
echo ""
echo "Rust: Checking formatting..."
if [ -d "simulator" ]; then
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
fi

echo ""
echo "[OK] All CI checks passed! Safe to push."
