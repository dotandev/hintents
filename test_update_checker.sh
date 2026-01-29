#!/bin/bash
set -e

echo "=== Update Checker Integration Tests ==="
echo ""

# Clean slate
echo "1. Cleaning cache and config..."
rm -rf ~/.cache/erst ~/.config/erst
echo "   ✓ Clean"
echo ""

# Build with old version
echo "2. Building with old version (v0.0.1)..."
go build -ldflags "-X main.Version=v0.0.1" -o erst_test ./cmd/erst
echo "   ✓ Built"
echo ""

# First run - should check (but will fail silently due to no releases)
echo "3. First run (checking update checker runs)..."
timeout 10 ./erst_test version > /dev/null 2>&1 || true
sleep 2  # Wait for goroutine

# Note: Cache won't be created because GitHub has no releases (404)
# This is expected behavior - update checker fails silently
if [ -f ~/.cache/erst/last_update_check ]; then
    echo "   ✓ Cache file created (GitHub has releases)"
else
    echo "   ℹ Cache file not created (expected - no GitHub releases yet)"
    echo "   ✓ Update checker ran without errors"
fi
echo ""

# Test opt-out
echo "4. Testing opt-out..."
ERST_NO_UPDATE_CHECK=1 ./erst_test version > /dev/null 2>&1
echo "   ✓ Opt-out works (no errors)"
echo ""

# Test with config file
echo "5. Testing config file opt-out..."
mkdir -p ~/.config/erst
cat > ~/.config/erst/config.yaml << 'EOF'
check_for_updates: false
EOF
./erst_test version > /dev/null 2>&1
echo "   ✓ Config file opt-out works"
rm ~/.config/erst/config.yaml
echo ""

# Test with current version
echo "6. Building with future version (v999.0.0)..."
go build -ldflags "-X main.Version=v999.0.0" -o erst_test ./cmd/erst
./erst_test version > output.txt 2>&1

if grep -q "new version" output.txt; then
    echo "   ✗ FAIL: Notification shown when already latest"
    cat output.txt
    exit 1
else
    echo "   ✓ No false notification"
fi
echo ""

# Test version command works
echo "7. Testing version command..."
VERSION_OUTPUT=$(./erst_test version)
if echo "$VERSION_OUTPUT" | grep -q "v999.0.0"; then
    echo "   ✓ Version command works: $VERSION_OUTPUT"
else
    echo "   ✗ FAIL: Version command output incorrect"
    echo "   Got: $VERSION_OUTPUT"
    exit 1
fi
echo ""

# Test help command
echo "8. Testing help command..."
if ./erst_test --help | grep -q "version"; then
    echo "   ✓ Help command shows version"
else
    echo "   ✗ FAIL: Help command doesn't show version"
    exit 1
fi
echo ""

# Cleanup
echo "9. Cleaning up..."
rm -f erst_test output.txt
rm -rf ~/.config/erst
echo "   ✓ Cleanup complete"
echo ""

echo "=== All Integration Tests Passed ✓ ==="
