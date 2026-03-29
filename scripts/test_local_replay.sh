#!/bin/bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Test script for local WASM replay functionality
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

echo "Testing local WASM replay functionality..."

# Ensure we have a built binary
if [ ! -f "./erst" ] && [ ! -f "./erst.exe" ]; then
    echo "Building erst binary..."
    go build -o erst ./cmd/erst
fi

BIN="./erst"
if [ -f "./erst.exe" ]; then
    BIN="./erst.exe"
fi

# Run a help command to verify it works
$BIN debug --help | grep -q "--wasm"
echo "[OK] debug --wasm flag present"

echo "WASM replay smoke test passed"
