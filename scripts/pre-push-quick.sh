#!/bin/bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0
#
# Quick pre-push checks (fast feedback loop)
# Run this frequently while coding

set -e

echo "⚡ Quick pre-push checks..."
echo ""

# Fast checks only
echo "🎨 Checking Go formatting..."
if [ -n "$(gofmt -l .)" ]; then
  echo "❌ Go files not formatted. Run 'go fmt ./...'"
  exit 1
fi

echo "🔎 Running go vet..."
go vet ./... || exit 1

echo "🦀 Checking Rust formatting..."
cd simulator
cargo fmt --check || {
  echo "❌ Rust files not formatted. Run 'cargo fmt'"
  exit 1
}
cd ..

echo "✅ Quick checks passed! (Run ./scripts/test-ci-locally.sh for full CI checks)"