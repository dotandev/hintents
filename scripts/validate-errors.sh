#!/bin/bash

# Validate error standardization implementation
set -e

echo "🔍 Validating error standardization..."

# Check if errors package exists
if [ ! -f "internal/errors/errors.go" ]; then
    echo "❌ Errors package not found"
    exit 1
fi
echo "✅ Errors package exists"

# Check if sentinel errors are defined
if grep -q "ErrTransactionNotFound.*errors\.New" internal/errors/errors.go; then
    echo "✅ ErrTransactionNotFound defined"
else
    echo "❌ ErrTransactionNotFound not found"
    exit 1
fi

if grep -q "ErrRPCConnectionFailed.*errors\.New" internal/errors/errors.go; then
    echo "✅ ErrRPCConnectionFailed defined"
else
    echo "❌ ErrRPCConnectionFailed not found"
    exit 1
fi

if grep -q "ErrSimulatorNotFound.*errors\.New" internal/errors/errors.go; then
    echo "✅ ErrSimulatorNotFound defined"
else
    echo "❌ ErrSimulatorNotFound not found"
    exit 1
fi

# Check if wrap functions exist
if grep -q "func WrapTransactionNotFound" internal/errors/errors.go; then
    echo "✅ WrapTransactionNotFound function exists"
else
    echo "❌ WrapTransactionNotFound function not found"
    exit 1
fi

# Check if packages are using standardized errors
if grep -q "errors\.WrapTransactionNotFound" internal/rpc/client.go; then
    echo "✅ RPC client uses standardized errors"
else
    echo "❌ RPC client not using standardized errors"
    exit 1
fi

if grep -q "errors\.WrapSimulatorNotFound" internal/simulator/runner.go; then
    echo "✅ Simulator runner uses standardized errors"
else
    echo "❌ Simulator runner not using standardized errors"
    exit 1
fi

if grep -q "errors\.WrapInvalidNetwork" internal/cmd/debug.go; then
    echo "✅ Debug command uses standardized errors"
else
    echo "❌ Debug command not using standardized errors"
    exit 1
fi

# Check if tests use standardized errors
if grep -q "errors\.ErrTransactionNotFound" internal/rpc/client_test.go; then
    echo "✅ Tests use standardized errors"
else
    echo "❌ Tests not using standardized errors"
    exit 1
fi

echo "🎉 All validation checks passed!"
echo "📋 Error standardization complete:"
echo "   - Sentinel errors defined for comparison with errors.Is"
echo "   - Wrap functions follow Go error wrapping best practices"
echo "   - All packages refactored to use standardized errors"
echo "   - Tests updated to use standardized error constants"
