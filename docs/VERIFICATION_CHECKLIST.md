# Verification Checklist

Quick reference checklist for verifying the Erst project components.

## Issue Verification

### Automated Checks
- [ ] Run issue verification script: `./scripts/verify_issues.sh`
- [ ] Verify GitHub API responses with curl
- [ ] Check issue count: `gh issue list --label "new_for_wave" --json number --jq 'length'`
- [ ] Expected count: 40 issues

### Manual Checks
- [ ] Navigate to https://github.com/dotandev/hintents/issues
- [ ] Filter by label: `new_for_wave`
- [ ] Verify all 40 issues are present
- [ ] Spot-check 5-10 issues for format compliance
- [ ] Verify labels are correctly applied
- [ ] Check issue numbering is sequential
- [ ] Confirm no duplicate issues

### Format Verification (per issue)
- [ ] Title has correct prefix: `[Category] Description`
- [ ] Description section present
- [ ] Requirements and Context section present
  - [ ] Background included
  - [ ] Success Criteria (Done) listed
- [ ] Suggested Execution section present
  - [ ] Fork/Branch specified
  - [ ] Implementation steps provided
- [ ] Test and Commit section present
  - [ ] Testing instructions included
  - [ ] PR Inclusions listed
  - [ ] Example Commit Message provided
- [ ] Guidelines section present
  - [ ] Must-haves specified

## Code Verification

### Build and Test
- [ ] Code compiles: `go build ./...`
- [ ] All tests pass: `go test ./...`
- [ ] Coverage ≥ 80%: `go test -cover ./...`
- [ ] No linting errors: `golangci-lint run`
- [ ] Code formatted: `go fmt ./...`

### CLI Functionality
- [ ] Help command works: `./erst --help`
- [ ] Version command works: `./erst version`
- [ ] Debug command works: `./erst debug <tx-hash>`
- [ ] Error handling is clear

## Documentation Verification

- [ ] README.md is up to date
- [ ] CONTRIBUTING.md is complete
- [ ] All links work (no 404s)
- [ ] Code examples compile
- [ ] API documentation is current
- [ ] Architecture docs reflect current design

## Security Verification

- [ ] No hardcoded credentials
- [ ] Dependencies scanned for vulnerabilities
- [ ] Input validation in place
- [ ] Error messages don't leak sensitive data

## Performance Verification

- [ ] Benchmarks run successfully: `go test -bench=.`
- [ ] No performance regressions
- [ ] Memory usage is acceptable

## Pre-Release Checklist

- [ ] All tests passing
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version bumped
- [ ] Release notes prepared
- [ ] Binaries build for all platforms

## Sign-off

Date: _______________

Verifier: _______________

Notes:
_________________________________________________________________
_________________________________________________________________
_________________________________________________________________

Status:
- [ ] All checks passed
- [ ] Ready for next phase
- [ ] Issues found (see notes)
