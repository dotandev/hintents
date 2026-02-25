#!/usr/bin/env bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0

# License Header Check - Make executable and run
# This script ensures the main script is executable and then runs it

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAIN_SCRIPT="$SCRIPT_DIR/check-license-headers.sh"

# Make the script executable
chmod +x "$MAIN_SCRIPT" 2>/dev/null || true

# Run the script
exec "$MAIN_SCRIPT" "$@"
