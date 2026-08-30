#!/bin/bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Run strict Go linting
set -euo pipefail

# Ensure we are in the project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." &>/dev/null && pwd)"
cd "${REPO_ROOT}" || { echo "Failed to change directory to project root: ${REPO_ROOT}"; exit 1; }

echo "Running strict Go linting..."
echo "Project root: ${REPO_ROOT}"
if command -v golangci-lint &> /dev/null; then
    golangci-lint run --config=.golangci.yml --timeout=5m --max-issues-per-linter=0 --max-same-issues=0
else
    echo "Warning: golangci-lint not found, running go vet only"
fi
go vet ./...
echo "Strict linting passed"
