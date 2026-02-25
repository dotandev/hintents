# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0


# Script to run unused code detection and cleanup
# This script should be run after resolving the Go toolchain version mismatch

set -e

echo "Running unused code detection..."

# Check if golangci-lint is available
if ! command -v golangci-lint &> /dev/null; then
    echo "golangci-lint not found. Installing..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2
fi

# Run golangci-lint with unused linter
echo "Running golangci-lint with unused linter..."
golangci-lint run --enable unused --disable-all

# If no issues found, run full linter suite
if [ $? -eq 0 ]; then
    echo "No unused code found. Running full linter suite..."
    golangci-lint run
else
    echo "Unused code detected. Please review and remove before proceeding."
    exit 1
fi

echo "Linting completed successfully!"
