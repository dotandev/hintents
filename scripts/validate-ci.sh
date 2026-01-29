#!/bin/bash

# Validate CI/CD configuration
set -e

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

# Validate go.mod version matches CI
GO_VERSION=$(grep "^go " go.mod | awk '{print $2}')
CI_GO_VERSION=$(grep "go-version:" .github/workflows/ci.yml | head -1 | sed "s/.*go-version: *'\([^']*\)'.*/\1/")

if [ "$GO_VERSION" != "$CI_GO_VERSION" ]; then
    echo " Go version mismatch: go.mod=$GO_VERSION, CI=$CI_GO_VERSION"
    exit 1
fi

echo " Go versions match: $GO_VERSION"

# Check if golangci-lint config is valid YAML
if command -v yamllint &> /dev/null; then
    yamllint .golangci.yml
    echo " golangci-lint config is valid YAML"
else
    echo "  yamllint not available, skipping YAML validation"
fi

echo " CI/CD configuration validation passed"
