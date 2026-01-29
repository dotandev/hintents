# Verification Plan for Erst Project

This document outlines the verification strategy for the Erst project, ensuring quality, correctness, and completeness of all components.

## Overview

The verification plan covers automated testing, manual verification, integration testing, and continuous monitoring to ensure the project meets its objectives.

## 1. Automated Tests

### 1.1 Unit Tests

**Scope**: Individual functions and components

**Coverage Requirements**:
- Minimum 80% code coverage for core packages
- 100% coverage for critical paths (transaction parsing, simulation)

**Test Categories**:
- RPC client functionality
- Simulator integration
- XDR parsing and decoding
- Error handling and edge cases

**Execution**:
```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out
```

### 1.2 Integration Tests

**Scope**: End-to-end workflows

**Test Scenarios**:
- Fetch transaction from Horizon (testnet/mainnet)
- Run simulation with real transaction data
- Parse and display diagnostic events
- Token flow analysis
- Session management

**Execution**:
```bash
# Run integration tests (requires network access)
go test ./... -tags=integration -v
```

### 1.3 Automated Issue Verification

**Purpose**: Verify GitHub issue creation and management

**Script**: `scripts/verify_issues.sh`

**Checks**:
- All 40 issues created successfully
- Correct labels applied (`new_for_wave`)
- Issue format matches standard template
- Proper milestone assignment
- Correct issue numbering

**Execution**:
```bash
# Verify issue creation
./scripts/verify_issues.sh

# Use GitHub API to verify
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/dotandev/hintents/issues?labels=new_for_wave
```

### 1.4 API Response Verification

**Purpose**: Validate GitHub API interactions

**Test Cases**:
- Issue creation returns 201 status
- Labels are correctly applied
- Issue body contains all required sections
- Milestone is set correctly

**Sample Verification**:
```bash
# Create test issue
curl -X POST \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Content-Type: application/json" \
  -d @test_issue.json \
  https://api.github.com/repos/dotandev/hintents/issues

# Verify response
# Expected: 201 Created with issue number
```

## 2. Manual Verification

### 2.1 Repository Issue Verification

**Checklist**:
- [ ] All 40 issues appear on `dotandev/hintents`
- [ ] Labels `new_for_wave` are applied to all issues
- [ ] Issue format matches the standard template
- [ ] Issues are properly categorized (Feature, Refactor, UX, etc.)
- [ ] Each issue has clear acceptance criteria
- [ ] Suggested execution steps are included
- [ ] Example commit messages are provided

**Verification Steps**:
1. Navigate to https://github.com/dotandev/hintents/issues
2. Filter by label: `new_for_wave`
3. Verify count matches expected (40 issues)
4. Spot-check 5-10 issues for format compliance
5. Verify issue numbers are sequential
6. Check that no duplicate issues exist

### 2.2 Issue Format Verification

**Required Sections** (per issue):
- Title with prefix: `[Category] Brief description`
- Description section
- Requirements and Context
  - Background
  - Success Criteria (Done)
- Suggested Execution
  - Fork/Branch
  - Implementation steps
- Test and Commit
  - Testing instructions
  - PR Inclusions
  - Example Commit Message
- Guidelines
  - Must-haves

**Sample Issue Format**:
```markdown
#123 [Feature] Add transaction replay capability

Description
Enable replay of historical Stellar transactions...

Requirements and Context
Background: Users need to debug failed transactions...
Success Criteria (Done):
- Transaction can be replayed locally
- Diagnostic events are captured

Suggested Execution
Fork/Branch: feature/transaction-replay
Implementation:
1. Fetch transaction from Horizon
2. Set up local simulation environment
...

Test and Commit
Testing:
- Test with known failed transaction
PR Inclusions:
- Replay implementation
- Unit tests
Example Commit Message
feat(replay): add transaction replay capability

Guidelines
Must-haves: Error handling for network failures
```

### 2.3 CLI Functionality Verification

**Test Scenarios**:
1. **Help Command**
   ```bash
   ./erst --help
   # Verify: Commands listed alphabetically
   ```

2. **Version Command**
   ```bash
   ./erst version
   # Verify: Version displayed correctly
   ```

3. **Debug Command**
   ```bash
   ./erst debug <tx-hash> --network testnet
   # Verify: Transaction fetched and analyzed
   ```

4. **Error Handling**
   ```bash
   ./erst debug invalid-hash
   # Verify: Clear error message displayed
   ```

### 2.4 Documentation Verification

**Checklist**:
- [ ] README.md is up to date
- [ ] CONTRIBUTING.md includes all contribution guidelines
- [ ] API documentation is complete
- [ ] Architecture docs reflect current design
- [ ] Examples are working and tested
- [ ] Links are not broken

**Tools**:
```bash
# Check for broken links
markdown-link-check docs/**/*.md

# Verify code examples compile
go test -run Example
```

## 3. Performance Verification

### 3.1 Benchmarks

**Metrics to Track**:
- Transaction fetch time
- Simulation execution time
- Memory usage
- XDR parsing performance

**Execution**:
```bash
# Run benchmarks
go test -bench=. -benchmem ./...

# Profile CPU usage
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof
```

### 3.2 Load Testing

**Scenarios**:
- Multiple concurrent transaction fetches
- Large transaction processing
- Bulk simulation requests

## 4. Security Verification

### 4.1 Dependency Scanning

**Tools**:
```bash
# Check for known vulnerabilities
go list -json -m all | nancy sleuth

# Update dependencies
go get -u ./...
go mod tidy
```

### 4.2 Code Security

**Checks**:
- No hardcoded credentials
- Proper error handling (no sensitive data in errors)
- Input validation for all user inputs
- Safe XDR parsing

## 5. Continuous Integration

### 5.1 CI Pipeline Checks

**On Every PR**:
- [ ] All tests pass
- [ ] Code coverage meets threshold
- [ ] Linting passes (`golangci-lint`)
- [ ] Formatting is correct (`go fmt`)
- [ ] No security vulnerabilities
- [ ] Documentation builds successfully

### 5.2 Pre-Release Checklist

**Before Each Release**:
- [ ] All tests passing on main branch
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version bumped correctly
- [ ] Release notes prepared
- [ ] Binary builds for all platforms
- [ ] Docker image builds successfully

## 6. Issue Tracking Verification

### 6.1 GitHub Issues

**Automated Checks**:
```bash
# Count issues with label
gh issue list --label "new_for_wave" --json number --jq 'length'

# Verify all issues have required fields
gh issue list --label "new_for_wave" --json number,title,labels,body
```

**Manual Checks**:
- Issue titles follow naming convention
- All issues are properly labeled
- Milestones are assigned
- No duplicate issues

### 6.2 Issue Quality

**Review Criteria**:
- Clear and concise description
- Actionable acceptance criteria
- Realistic implementation suggestions
- Appropriate complexity/priority labels

## 7. Verification Schedule

### Daily
- Automated test runs on CI
- Dependency vulnerability scans

### Weekly
- Manual spot-checks of new issues
- Documentation review
- Performance benchmark comparison

### Before Each Release
- Full manual verification checklist
- Integration test suite
- Security audit
- Documentation completeness check

## 8. Reporting

### 8.1 Test Results

**Format**: JUnit XML for CI integration
**Storage**: Artifacts in CI/CD pipeline
**Retention**: 30 days

### 8.2 Coverage Reports

**Format**: HTML coverage report
**Publishing**: GitHub Pages or CI artifacts
**Threshold**: 80% minimum

### 8.3 Issue Verification Report

**Template**:
```markdown
## Issue Verification Report

Date: YYYY-MM-DD
Verifier: [Name]

### Summary
- Total issues created: X/40
- Issues with correct labels: X/40
- Issues with correct format: X/40

### Issues Found
- [List any problems]

### Action Items
- [Required fixes]

### Sign-off
- [ ] All issues verified
- [ ] Ready for development
```

## 9. Tools and Scripts

### 9.1 Verification Scripts

**Location**: `scripts/`

**Available Scripts**:
- `verify_issues.sh` - Verify GitHub issues
- `run_tests.sh` - Run full test suite
- `check_coverage.sh` - Generate coverage report
- `lint.sh` - Run linters
- `verify_docs.sh` - Check documentation

### 9.2 GitHub CLI Commands

```bash
# List all issues with label
gh issue list --label "new_for_wave"

# View specific issue
gh issue view 123

# Check issue count
gh issue list --label "new_for_wave" --json number --jq 'length'

# Export issues to JSON
gh issue list --label "new_for_wave" --json number,title,labels,body > issues.json
```

## 10. Success Criteria

### Project-Level
- [ ] All 40 issues created and verified
- [ ] Test coverage ≥ 80%
- [ ] All CI checks passing
- [ ] Documentation complete and accurate
- [ ] No critical security vulnerabilities
- [ ] Performance benchmarks meet targets

### Issue-Level
- [ ] Correct label (`new_for_wave`) applied
- [ ] Format matches standard template
- [ ] All required sections present
- [ ] Clear acceptance criteria
- [ ] Actionable implementation steps

## Appendix

### A. Standard Issue Template

See `ISSUE_TEMPLATE.md` for the complete template.

### B. Verification Checklist

Printable checklist available in `docs/VERIFICATION_CHECKLIST.md`.

### C. Contact

For questions about verification:
- Create an issue with label `question`
- Contact maintainers via GitHub Discussions
