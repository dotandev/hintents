#!/bin/bash
# Copyright 2025 Erst Users
# SPDX-License-Identifier: Apache-2.0

# Verification script for GitHub issues
# Verifies that all issues are created correctly with proper labels and format

set -e

REPO="dotandev/hintents"
LABEL="new_for_wave"
EXPECTED_COUNT=40

echo "========================================="
echo "GitHub Issues Verification Script"
echo "========================================="
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "❌ Error: GitHub CLI (gh) is not installed"
    echo "Install it from: https://cli.github.com/"
    exit 1
fi

echo "✓ GitHub CLI found"
echo ""

# Check authentication
if ! gh auth status &> /dev/null; then
    echo "❌ Error: Not authenticated with GitHub"
    echo "Run: gh auth login"
    exit 1
fi

echo "✓ Authenticated with GitHub"
echo ""

# Count issues with the label
echo "Checking issues with label '$LABEL'..."
ISSUE_COUNT=$(gh issue list --repo "$REPO" --label "$LABEL" --json number --jq 'length')

echo "Found: $ISSUE_COUNT issues"
echo "Expected: $EXPECTED_COUNT issues"
echo ""

if [ "$ISSUE_COUNT" -eq "$EXPECTED_COUNT" ]; then
    echo "✓ Issue count matches expected"
else
    echo "❌ Issue count mismatch!"
    echo "   Expected: $EXPECTED_COUNT"
    echo "   Found: $ISSUE_COUNT"
    exit 1
fi

# Fetch all issues with the label
echo ""
echo "Fetching issue details..."
ISSUES_JSON=$(gh issue list --repo "$REPO" --label "$LABEL" --limit 100 --json number,title,labels,body)

# Check if all issues have the required label
echo ""
echo "Verifying labels..."
LABEL_COUNT=$(echo "$ISSUES_JSON" | jq "[.[] | select(.labels | map(.name) | contains([\"$LABEL\"]))] | length")

if [ "$LABEL_COUNT" -eq "$ISSUE_COUNT" ]; then
    echo "✓ All issues have the '$LABEL' label"
else
    echo "❌ Some issues are missing the '$LABEL' label"
    exit 1
fi

# Check issue format (spot check first 5 issues)
echo ""
echo "Spot-checking issue format (first 5 issues)..."
FORMAT_ERRORS=0

for i in {0..4}; do
    ISSUE_BODY=$(echo "$ISSUES_JSON" | jq -r ".[$i].body // empty")
    ISSUE_NUMBER=$(echo "$ISSUES_JSON" | jq -r ".[$i].number // empty")
    
    if [ -z "$ISSUE_BODY" ]; then
        continue
    fi
    
    # Check for required sections
    if ! echo "$ISSUE_BODY" | grep -q "Requirements and Context"; then
        echo "❌ Issue #$ISSUE_NUMBER missing 'Requirements and Context' section"
        FORMAT_ERRORS=$((FORMAT_ERRORS + 1))
    fi
    
    if ! echo "$ISSUE_BODY" | grep -q "Success Criteria"; then
        echo "❌ Issue #$ISSUE_NUMBER missing 'Success Criteria' section"
        FORMAT_ERRORS=$((FORMAT_ERRORS + 1))
    fi
    
    if ! echo "$ISSUE_BODY" | grep -q "Suggested Execution"; then
        echo "❌ Issue #$ISSUE_NUMBER missing 'Suggested Execution' section"
        FORMAT_ERRORS=$((FORMAT_ERRORS + 1))
    fi
done

if [ "$FORMAT_ERRORS" -eq 0 ]; then
    echo "✓ All spot-checked issues have correct format"
else
    echo "❌ Found $FORMAT_ERRORS format errors in spot-checked issues"
    exit 1
fi

# Summary
echo ""
echo "========================================="
echo "Verification Summary"
echo "========================================="
echo "✓ Issue count: $ISSUE_COUNT/$EXPECTED_COUNT"
echo "✓ Labels applied correctly"
echo "✓ Format checks passed"
echo ""
echo "✅ All verifications passed!"
echo ""

# Optional: Export issues to JSON file
if [ "$1" == "--export" ]; then
    OUTPUT_FILE="issues_export_$(date +%Y%m%d_%H%M%S).json"
    echo "$ISSUES_JSON" > "$OUTPUT_FILE"
    echo "📄 Issues exported to: $OUTPUT_FILE"
fi

exit 0
