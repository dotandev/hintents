# Scripts Directory

This directory contains utility scripts for testing, verification, and automation.

## Available Scripts

### `verify_issues.sh`

Automated verification script for GitHub issues.

**Purpose**: Verifies that all issues are created correctly with proper labels and format.

**Prerequisites**:
1. Install GitHub CLI: https://cli.github.com/
   ```bash
   # macOS
   brew install gh
   
   # Linux
   sudo apt install gh
   ```

2. Authenticate with GitHub:
   ```bash
   gh auth login
   ```

**Usage**:

```bash
# Basic verification
./scripts/verify_issues.sh

# Verify and export issues to JSON
./scripts/verify_issues.sh --export
```

**What it checks**:
- ✅ Issue count (expects 40 issues)
- ✅ All issues have `new_for_wave` label
- ✅ Issue format compliance (spot-checks first 5 issues)
- ✅ Required sections present:
  - Requirements and Context
  - Success Criteria
  - Suggested Execution

**Output**:
```
=========================================
GitHub Issues Verification Script
=========================================

✓ GitHub CLI found
✓ Authenticated with GitHub

Checking issues with label 'new_for_wave'...
Found: 40 issues
Expected: 40 issues

✓ Issue count matches expected

Fetching issue details...

Verifying labels...
✓ All issues have the 'new_for_wave' label

Spot-checking issue format (first 5 issues)...
✓ All spot-checked issues have correct format

=========================================
Verification Summary
=========================================
✓ Issue count: 40/40
✓ Labels applied correctly
✓ Format checks passed

✅ All verifications passed!
```

**Troubleshooting**:

- **Error: GitHub CLI not installed**
  ```
  Install from: https://cli.github.com/
  ```

- **Error: Not authenticated**
  ```bash
  gh auth login
  ```

- **Error: Issue count mismatch**
  - Check if all issues were created
  - Verify the label name is correct
  - Check repository name

## Manual Verification

If you prefer manual verification using GitHub API:

```bash
# Check issue count
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/dotandev/hintents/issues?labels=new_for_wave&per_page=100" \
  | jq 'length'

# Get all issues with label
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/dotandev/hintents/issues?labels=new_for_wave&per_page=100" \
  | jq '.[] | {number, title, labels: [.labels[].name]}'

# Export to file
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/dotandev/hintents/issues?labels=new_for_wave&per_page=100" \
  > issues.json
```

## Using GitHub CLI Directly

```bash
# List all issues with label
gh issue list --repo dotandev/hintents --label new_for_wave

# Count issues
gh issue list --repo dotandev/hintents --label new_for_wave --json number --jq 'length'

# View specific issue
gh issue view 123 --repo dotandev/hintents

# Export issues to JSON
gh issue list --repo dotandev/hintents --label new_for_wave --limit 100 \
  --json number,title,labels,body > issues_export.json
```

## Quick Start

1. **Install GitHub CLI** (if not already installed):
   ```bash
   brew install gh  # macOS
   ```

2. **Authenticate**:
   ```bash
   gh auth login
   ```

3. **Run verification**:
   ```bash
   cd /path/to/hintents
   ./scripts/verify_issues.sh
   ```

4. **Check results**:
   - Green checkmarks (✓) = passed
   - Red X (❌) = failed
   - Script exits with code 0 on success, 1 on failure

## CI/CD Integration

To use this script in CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Verify Issues
  run: |
    ./scripts/verify_issues.sh
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Adding New Scripts

When adding new scripts to this directory:

1. Make them executable:
   ```bash
   chmod +x scripts/your_script.sh
   ```

2. Add a shebang line:
   ```bash
   #!/bin/bash
   ```

3. Add copyright header:
   ```bash
   # Copyright 2025 Erst Users
   # SPDX-License-Identifier: Apache-2.0
   ```

4. Update this README with usage instructions

5. Add error handling:
   ```bash
   set -e  # Exit on error
   ```

## Support

For issues with scripts:
1. Check prerequisites are installed
2. Verify authentication
3. Check script permissions (`chmod +x`)
4. Review error messages
5. Open an issue on GitHub if problems persist
