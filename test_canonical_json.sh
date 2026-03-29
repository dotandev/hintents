#!/bin/bash
# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Test script for canonical JSON serialization in audit logs
# This verifies that audit payload hashing is deterministic across platforms

set -e

echo "Testing canonical JSON serialization for audit logs..."
echo ""

# Run canonical JSON tests
echo "1. Running canonical JSON unit tests..."
go test -v ./internal/cmd -run TestCanonicalJSON

# Run audit generation and verification tests
echo ""
echo "2. Running audit generation and verification tests..."
go test -v ./internal/cmd -run TestGenerate
go test -v ./internal/cmd -run TestVerify

# Run determinism tests
echo ""
echo "3. Running cross-platform determinism tests..."
go test -v ./internal/cmd -run TestCanonicalJSON_Deterministic
go test -v ./internal/cmd -run TestGenerate_DeterministicHash

echo ""
echo "All tests passed! Canonical JSON serialization is working correctly."
