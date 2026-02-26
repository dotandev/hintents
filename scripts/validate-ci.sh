# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0


# Validate CI/CD configuration
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

echo "Validating CI/CD configuration..."

# Check if required files exist
if [ ! -f ".github/workflows/ci.yml" ]; then
    echo " CI workflow file missing"
    exit 1
fi

if [ ! -f ".golangci.yml" ]; then
    echo " golangci-lint config missing"
    exit 1
fi

if [ ! -f "go.mod" ]; then
    echo " go.mod missing"
    exit 1
fi

# Validate go.mod version is represented in CI matrix (best-effort, non-fatal)
GO_VERSION=$(grep "^go " go.mod | awk '{print $2}')
if grep -q "go-version:" .github/workflows/ci.yml; then
    if grep -q "go-version: \"${GO_VERSION}\"" .github/workflows/ci.yml; then
        echo " Go version ${GO_VERSION} is present in CI matrix"
    else
        echo " Warning: Go version mismatch between go.mod (${GO_VERSION}) and CI matrix"
    fi
else
    echo " Warning: No go-version entries found in .github/workflows/ci.yml"
fi

# Check if golangci-lint config is valid YAML
if command -v yamllint &> /dev/null; then
    yamllint .golangci.yml
    echo " golangci-lint config is valid YAML"
else
    echo "  yamllint not available, skipping YAML validation"
fi

echo " CI/CD configuration validation passed"
