#!/bin/bash

# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0

# CI License Header Check Wrapper
# This wrapper ensures the license header check works in CI environments
# where execute permissions might not be available

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LICENSE_SCRIPT="$SCRIPT_DIR/check-license-headers.sh"

# Check if the script exists
if [ ! -f "$LICENSE_SCRIPT" ]; then
    echo "ERROR: License header script not found at $LICENSE_SCRIPT"
    exit 1
fi

# Execute the script with bash to avoid permission issues
echo "Running license header check..."
exec bash "$LICENSE_SCRIPT"
