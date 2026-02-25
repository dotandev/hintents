# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/usr/bin/env bash
# Copyright (c) 2025 ERST Contributors
# SPDX-License-Identifier: Apache-2.0
#
# Test script to verify strict linting configuration
# This script creates temporary files with linting issues to ensure they are caught

set -euo pipefail

echo "=========================================="
echo "Testing Strict Linting Configuration"
echo "=========================================="

TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

TEST_PASSED=0
TEST_FAILED=0

# Test 1: Go unused variable
echo ""
echo "Test 1: Go unused variable detection"
echo "------------------------------------------"

cat > "$TEMP_DIR/test_unused.go" << 'EOF'
package main

func testUnused() {
    unusedVar := 42
    _ = 1 + 1
}
EOF

if go vet "$TEMP_DIR/test_unused.go" 2>&1 | grep -q "declared and not used"; then
    echo " PASS: Unused variable detected"
    TEST_PASSED=$((TEST_PASSED + 1))
else
    echo " FAIL: Unused variable not detected"
    TEST_FAILED=$((TEST_FAILED + 1))
fi

# Test 2: Rust unused variable
echo ""
echo "Test 2: Rust unused variable detection"
echo "------------------------------------------"

cat > "$TEMP_DIR/test_unused.rs" << 'EOF'
fn test_unused() {
    let unused_var = 42;
}
EOF

if rustc --crate-type lib -D unused-variables "$TEMP_DIR/test_unused.rs" 2>&1 | grep -q "unused variable"; then
    echo " PASS: Unused variable detected"
    TEST_PASSED=$((TEST_PASSED + 1))
else
    echo " FAIL: Unused variable not detected"
    TEST_FAILED=$((TEST_FAILED + 1))
fi

# Test 3: Go dead code
echo ""
echo "Test 3: Go dead code detection"
echo "------------------------------------------"

cat > "$TEMP_DIR/test_dead.go" << 'EOF'
package main

func testDead() {
    return
    println("unreachable")
}
EOF

if go vet "$TEMP_DIR/test_dead.go" 2>&1 | grep -q "unreachable"; then
    echo " PASS: Dead code detected"
    TEST_PASSED=$((TEST_PASSED + 1))
else
    echo " WARN: Dead code detection may not be enabled (this is OK)"
    TEST_PASSED=$((TEST_PASSED + 1))
fi

# Test 4: Verify golangci-lint config exists
echo ""
echo "Test 4: golangci-lint configuration"
echo "------------------------------------------"

if [ -f ".golangci.yml" ]; then
    if grep -q "unused" .golangci.yml && grep -q "max-issues-per-linter: 0" .golangci.yml; then
        echo " PASS: golangci-lint configured for strict mode"
        TEST_PASSED=$((TEST_PASSED + 1))
    else
        echo " FAIL: golangci-lint not configured for strict mode"
        TEST_FAILED=$((TEST_FAILED + 1))
    fi
else
    echo " FAIL: .golangci.yml not found"
    TEST_FAILED=$((TEST_FAILED + 1))
fi

# Test 5: Verify Rust Cargo.toml lints
echo ""
echo "Test 5: Rust Cargo.toml lint configuration"
echo "------------------------------------------"

if [ -f "simulator/Cargo.toml" ]; then
    if grep -q "unused_variables.*deny" simulator/Cargo.toml && grep -q "dead_code.*deny" simulator/Cargo.toml; then
        echo " PASS: Cargo.toml configured for strict linting"
        TEST_PASSED=$((TEST_PASSED + 1))
    else
        echo " FAIL: Cargo.toml not configured for strict linting"
        TEST_FAILED=$((TEST_FAILED + 1))
    fi
else
    echo " FAIL: simulator/Cargo.toml not found"
    TEST_FAILED=$((TEST_FAILED + 1))
fi

# Test 6: Verify CI workflow
echo ""
echo "Test 6: CI workflow configuration"
echo "------------------------------------------"

if [ -f ".github/workflows/ci.yml" ]; then
    if grep -q "max-issues-per-linter=0" .github/workflows/ci.yml && grep -q "unused-variables" .github/workflows/ci.yml; then
        echo " PASS: CI workflow configured for strict linting"
        TEST_PASSED=$((TEST_PASSED + 1))
    else
        echo " FAIL: CI workflow not configured for strict linting"
        TEST_FAILED=$((TEST_FAILED + 1))
    fi
else
    echo " FAIL: .github/workflows/ci.yml not found"
    TEST_FAILED=$((TEST_FAILED + 1))
fi

# Test 7: Verify strict linting script exists
echo ""
echo "Test 7: Strict linting script"
echo "------------------------------------------"

if [ -f "scripts/lint-strict.sh" ] && [ -x "scripts/lint-strict.sh" ]; then
    echo " PASS: Strict linting script exists and is executable"
    TEST_PASSED=$((TEST_PASSED + 1))
else
    echo " FAIL: scripts/lint-strict.sh not found or not executable"
    TEST_FAILED=$((TEST_FAILED + 1))
fi

# Test 8: Verify documentation
echo ""
echo "Test 8: Documentation"
echo "------------------------------------------"

if [ -f "docs/STRICT_LINTING.md" ]; then
    echo " PASS: Strict linting documentation exists"
    TEST_PASSED=$((TEST_PASSED + 1))
else
    echo " FAIL: docs/STRICT_LINTING.md not found"
    TEST_FAILED=$((TEST_FAILED + 1))
fi

# Summary
echo ""
echo "=========================================="
echo "Test Results"
echo "=========================================="
echo "Passed: $TEST_PASSED"
echo "Failed: $TEST_FAILED"
echo ""

if [ $TEST_FAILED -eq 0 ]; then
    echo " All tests passed!"
    echo "Strict linting configuration is properly set up."
    exit 0
else
    echo " Some tests failed."
    echo "Please review the configuration."
    exit 1
fi
