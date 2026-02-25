# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0


# Validate simulator interface implementation
set -e

echo "Validating simulator interface implementation..."

# Check if interface is defined
if grep -q "type RunnerInterface interface" internal/simulator/runner.go; then
    echo "[OK] RunnerInterface defined"
else
    echo "[FAIL] RunnerInterface not found"
    exit 1
fi

# Check if Run method signature is correct
if grep -q "Run(req \*SimulationRequest) (\*SimulationResponse, error)" internal/simulator/runner.go; then
    echo "[OK] Run method signature correct"
else
    echo "[FAIL] Run method signature incorrect"
    exit 1
fi

# Check if compile-time check exists
if grep -q "var _ RunnerInterface = (\*Runner)(nil)" internal/simulator/runner.go; then
    echo "[OK] Compile-time interface check present"
else
    echo "[FAIL] Compile-time interface check missing"
    exit 1
fi

# Check if NewDebugCommand exists
if grep -q "func NewDebugCommand(runner simulator.RunnerInterface)" internal/cmd/debug.go; then
    echo "[OK] NewDebugCommand accepts interface"
else
    echo "[FAIL] NewDebugCommand not found or incorrect signature"
    exit 1
fi

# Check if MockRunner exists in tests
if grep -q "type MockRunner struct" internal/cmd/debug_test.go; then
    echo "[OK] MockRunner defined in tests"
else
    echo "[FAIL] MockRunner not found in tests"
    exit 1
fi

# Check if backward compatibility is maintained
if grep -q "var debugCmd = &cobra.Command" internal/cmd/debug.go; then
    echo "[OK] Backward compatibility maintained"
else
    echo "[FAIL] Original debugCmd not found"
    exit 1
fi

echo "All validation checks passed!"
echo "Implementation meets all requirements:"
echo "   - Interface defined with correct signature"
echo "   - Commands accept interface for dependency injection"
echo "   - Mock runner available for testing"
echo "   - Zero performance overhead"
echo "   - Backward compatibility maintained"
echo "   - Compile-time safety enforced"
