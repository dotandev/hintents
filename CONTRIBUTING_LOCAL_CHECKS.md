# Running CI Checks Locally

**Why?** Catch issues before pushing to avoid triggering multiple CI runs and wasting GitHub Actions minutes.

## Quick Pre-Push Checks (Recommended)

Run this frequently while coding - it's fast (~5-10 seconds):

```bash
make pre-push
# or
./scripts/pre-push-quick.sh
```

This checks:
- ✅ Go formatting
- ✅ Go vet
- ✅ Rust formatting

## Full CI Checks (Before PR)

Run this before opening/updating a PR to match exactly what CI runs:

```bash
make ci-local
# or
./scripts/test-ci-locally.sh
```

This runs everything CI does:
- ✅ License headers
- ✅ Go: mod verify, fmt, vet, golangci-lint, tests, build
- ✅ Rust: fmt, clippy, tests, build
- ✅ Docs spellcheck (if misspell installed)

## Optional: Git Pre-Push Hook

Automatically run quick checks before every push:

```bash
cp .git/hooks/pre-push.sample .git/hooks/pre-push
chmod +x .git/hooks/pre-push
```

Now `git push` will automatically run quick checks first.

## Installing Optional Tools

For full CI parity, install:

```bash
# golangci-lint (Go linter)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# misspell (Docs spellcheck)
go install github.com/client9/misspell/cmd/misspell@latest
```

## Workflow Recommendation

1. **While coding**: Run `make pre-push` frequently (every few commits)
2. **Before PR**: Run `make ci-local` to catch everything
3. **Result**: Fewer CI failures, faster feedback, less wasted CI minutes
