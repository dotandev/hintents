#!/bin/bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0
#
# Test CI checks locally before pushing
# This script runs the same checks as GitHub Actions CI

set -e

echo "🔍 Running CI checks locally..."
echo "=================================="
echo ""

# ============================================
# License Header Check
# ============================================
echo "📄 Checking license headers..."
if ! ./scripts/check-license-headers.sh; then
  echo "❌ License header check failed"
  exit 1
fi
echo ""

# ============================================
# Go CLI - Lint, Build & Test
# ============================================
echo "📦 Go: Verifying dependencies..."
go mod verify
echo "✅ Dependencies verified"
echo ""

echo "🎨 Go: Checking formatting..."
if [ -n "$(gofmt -l .)" ]; then
  echo "❌ Go files are not formatted. Run 'go fmt ./...' to fix."
  gofmt -d .
  exit 1
fi
echo "✅ Go files are properly formatted"
echo ""

echo "🔎 Go: Running go vet..."
go vet ./...
echo "✅ go vet passed"
echo ""

# Check if golangci-lint is installed
if command -v golangci-lint &> /dev/null; then
  echo "🔍 Go: Running golangci-lint..."
  golangci-lint run --timeout=5m
  echo "✅ golangci-lint passed"
  echo ""
else
  echo "⚠️  golangci-lint not installed (skipping)"
  echo "   Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  echo ""
fi

echo "🧪 Go: Running tests with race detector..."
go test -v -race ./...
echo "✅ Go tests passed"
echo ""

echo "🏗️  Go: Building..."
go build -v ./...
echo "✅ Go build succeeded"
echo ""

# ============================================
# Rust Simulator - Lint, Build & Test
# ============================================
echo "🦀 Rust: Checking formatting..."
cd simulator
if ! cargo fmt --check; then
  echo "❌ Rust files are not formatted. Run 'cargo fmt' to fix."
  exit 1
fi
echo "✅ Rust files are properly formatted"
echo ""

echo "📎 Rust: Running Clippy..."
cargo clippy --all-targets --all-features -- -D warnings
echo "✅ Clippy passed"
echo ""

echo "🧪 Rust: Running tests..."
cargo test --verbose
echo "✅ Rust tests passed"
echo ""

echo "🏗️  Rust: Building..."
cargo build --verbose
echo "✅ Rust build succeeded"
echo ""

cd ..

# ============================================
# Docs - Spell Check (optional)
# ============================================
if command -v misspell &> /dev/null; then
  echo "📝 Docs: Running spellcheck..."
  IGNORE_WORDS=$(paste -sd, .github/spelling/allow.txt 2>/dev/null || echo "")
  if [ -n "$IGNORE_WORDS" ]; then
    find . -name '*.md' -print0 | xargs -0 misspell -error -i "$IGNORE_WORDS" || {
      echo "❌ Spellcheck failed"
      exit 1
    }
  else
    find . -name '*.md' -print0 | xargs -0 misspell -error || {
      echo "❌ Spellcheck failed"
      exit 1
    }
  fi
  echo "✅ Spellcheck passed"
  echo ""
else
  echo "⚠️  misspell not installed (skipping spellcheck)"
  echo "   Install with: go install github.com/client9/misspell/cmd/misspell@latest"
  echo ""
fi

# ============================================
# Summary
# ============================================
echo "=================================="
echo "✅ All CI checks passed! Safe to push."
echo ""
echo "💡 Tip: Run this before every push to avoid CI failures"