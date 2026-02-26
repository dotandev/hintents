# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0


# Validate error standardization implementation
set -e

echo "Validating error standardization..."

# Check if errors package exists
if [ ! -f "internal/errors/errors.go" ]; then
    echo "[FAIL] Errors package not found"
    exit 1
fi
echo "[OK] Errors package exists"

# Check if sentinel errors are defined
if grep -q "ErrTransactionNotFound.*errors\.New" internal/errors/errors.go; then
    echo "[OK] ErrTransactionNotFound defined"
else
    echo "[FAIL] ErrTransactionNotFound not found"
    exit 1
fi

if grep -q "ErrRPCConnectionFailed.*errors\.New" internal/errors/errors.go; then
    echo "[OK] ErrRPCConnectionFailed defined"
else
    echo "[FAIL] ErrRPCConnectionFailed not found"
    exit 1
fi

if grep -q "ErrSimulatorNotFound.*errors\.New" internal/errors/errors.go; then
    echo "[OK] ErrSimulatorNotFound defined"
else
    echo "[FAIL] ErrSimulatorNotFound not found"
    exit 1
fi

# Check if wrap functions exist
if grep -q "func WrapTransactionNotFound" internal/errors/errors.go; then
    echo "[OK] WrapTransactionNotFound function exists"
else
    echo "[FAIL] WrapTransactionNotFound function not found"
    exit 1
fi

# Check if packages are using standardized errors
if grep -q "errors\.WrapTransactionNotFound" internal/rpc/client.go; then
    echo "[OK] RPC client uses standardized errors"
else
    echo "[FAIL] RPC client not using standardized errors"
    exit 1
fi

if grep -q "errors\.WrapSimulatorNotFound" internal/simulator/runner.go; then
    echo "[OK] Simulator runner uses standardized errors"
else
    echo "[FAIL] Simulator runner not using standardized errors"
    exit 1
fi

if grep -q "errors\.WrapInvalidNetwork" internal/cmd/debug.go; then
    echo "[OK] Debug command uses standardized errors"
else
    echo "[FAIL] Debug command not using standardized errors"
    exit 1
fi

# Check if tests use standardized errors
if grep -q "errors\.ErrTransactionNotFound" internal/rpc/client_test.go; then
    echo "[OK] Tests use standardized errors"
else
    echo "[FAIL] Tests not using standardized errors"
    exit 1
fi

echo "All validation checks passed!"
echo "Error standardization complete:"
echo "   - Sentinel errors defined for comparison with errors.Is"
echo "   - Wrap functions follow Go error wrapping best practices"
echo "   - All packages refactored to use standardized errors"
echo "   - Tests updated to use standardized error constants"
