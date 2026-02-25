# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash
# Verify that erst debug produces no ANSI escape characters when piped
# Usage: ./scripts/verify_no_ansi.sh
set -e

cd "$(dirname "$0")/.."
BIN="./erst"

# Build if needed
if [ ! -x "$BIN" ] && ! command -v erst &>/dev/null; then
  echo "Building erst..."
  go build -o erst ./cmd/erst/
fi
if [ -x "$BIN" ]; then
  : # use ./erst
elif command -v erst &>/dev/null; then
  BIN="erst"
else
  echo "Error: erst binary not found. Run: go build -o erst ./cmd/erst/"
  exit 1
fi

# Use a valid-length hash (64 hex chars) - will fail to fetch but we get initial output
HASH="0"$(printf '0%.0s' {1..62})

echo "Testing: $BIN debug $HASH 2>&1 | cat"
OUTPUT=$($BIN debug "$HASH" 2>&1 | cat)

if echo "$OUTPUT" | grep -q $'\033'; then
  echo "FAIL: Output contains ANSI escape sequences"
  echo "$OUTPUT" | cat -A
  exit 1
fi

echo "PASS: No ANSI escape characters in piped output"

# Also verify NO_COLOR disables colors
echo ""
echo "Testing: NO_COLOR=1 $BIN debug $HASH 2>&1 | cat"
OUTPUT2=$(NO_COLOR=1 $BIN debug "$HASH" 2>&1 | cat)
if echo "$OUTPUT2" | grep -q $'\033'; then
  echo "FAIL: NO_COLOR=1 still produced ANSI escape sequences"
  exit 1
fi
echo "PASS: NO_COLOR honored"
