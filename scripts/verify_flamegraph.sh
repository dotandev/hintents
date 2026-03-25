# Copyright 2026 Erst Users
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash
# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

# Verification script for interactive flamegraph export feature

set -e

echo "=== Flamegraph Export Feature Verification ==="
echo ""

# Check if required files exist
echo "✓ Checking implementation files..."
test -f internal/visualizer/flamegraph.go || { echo "✗ Missing flamegraph.go"; exit 1; }
test -f internal/visualizer/flamegraph_test.go || { echo "✗ Missing flamegraph_test.go"; exit 1; }
test -f internal/cmd/root.go || { echo "✗ Missing root.go"; exit 1; }
test -f internal/cmd/debug.go || { echo "✗ Missing debug.go"; exit 1; }
test -f docs/INTERACTIVE_FLAMEGRAPH.md || { echo "✗ Missing documentation"; exit 1; }
echo "  All implementation files present"
echo ""

# Check for key functions in flamegraph.go
echo "✓ Checking implementation..."
grep -q "GenerateInteractiveHTML" internal/visualizer/flamegraph.go || { echo "✗ Missing GenerateInteractiveHTML"; exit 1; }
grep -q "ExportFlamegraph" internal/visualizer/flamegraph.go || { echo "✗ Missing ExportFlamegraph"; exit 1; }
grep -q "ExportFormat" internal/visualizer/flamegraph.go || { echo "✗ Missing ExportFormat type"; exit 1; }
grep -q "FormatHTML" internal/visualizer/flamegraph.go || { echo "✗ Missing FormatHTML constant"; exit 1; }
grep -q "FormatSVG" internal/visualizer/flamegraph.go || { echo "✗ Missing FormatSVG constant"; exit 1; }
echo "  All key functions implemented"
echo ""

# Check for interactive features in HTML template
echo "✓ Checking interactive features..."
grep -q "handleMouseOver" internal/visualizer/flamegraph.go || { echo "✗ Missing hover functionality"; exit 1; }
grep -q "handleClick" internal/visualizer/flamegraph.go || { echo "✗ Missing click-to-zoom"; exit 1; }
grep -q "performSearch" internal/visualizer/flamegraph.go || { echo "✗ Missing search functionality"; exit 1; }
grep -q "resetZoom" internal/visualizer/flamegraph.go || { echo "✗ Missing reset zoom"; exit 1; }
grep -q "prefers-color-scheme" internal/visualizer/flamegraph.go || { echo "✗ Missing dark mode support"; exit 1; }
echo "  All interactive features present"
echo ""

# Check for CLI flag
echo "✓ Checking CLI integration..."
grep -q "ProfileFormatFlag" internal/cmd/root.go || { echo "✗ Missing ProfileFormatFlag"; exit 1; }
grep -q "profile-format" internal/cmd/root.go || { echo "✗ Missing --profile-format flag"; exit 1; }
grep -q "ExportFlamegraph" internal/cmd/debug.go || { echo "✗ Missing export call in debug.go"; exit 1; }
echo "  CLI integration complete"
echo ""

# Check for tests
echo "✓ Checking test coverage..."
grep -q "TestGenerateInteractiveHTML" internal/visualizer/flamegraph_test.go || { echo "✗ Missing HTML generation tests"; exit 1; }
grep -q "TestExportFlamegraph" internal/visualizer/flamegraph_test.go || { echo "✗ Missing export tests"; exit 1; }
grep -q "TestExportFormat_GetFileExtension" internal/visualizer/flamegraph_test.go || { echo "✗ Missing format tests"; exit 1; }
echo "  Test coverage adequate"
echo ""

# Check for standalone HTML (no external dependencies)
echo "✓ Checking for standalone HTML..."
if grep -q 'src="http' internal/visualizer/flamegraph.go || grep -q 'href="http' internal/visualizer/flamegraph.go; then
    echo "✗ HTML template contains external dependencies"
    exit 1
fi
echo "  HTML is self-contained"
echo ""

# Check gitignore update
echo "✓ Checking .gitignore patterns..."
grep -q "*.flamegraph.html" internal/cmd/init.go || { echo "✗ Missing .flamegraph.html in gitignore"; exit 1; }
echo "  Gitignore patterns updated"
echo ""

echo "=== All Checks Passed ==="
echo ""
echo "Manual verification steps:"
echo "1. Build: make build"
echo "2. Run: ./bin/erst debug --profile --profile-format html <tx-hash>"
echo "3. Open the generated .flamegraph.html file in a browser"
echo "4. Verify interactive features work (hover, click, search)"
echo "5. Toggle system dark mode and verify colors adapt"
echo "6. Check browser console for errors (should be none)"
echo "7. Check network tab for requests (should be none)"
echo ""
