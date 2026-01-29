# CI/CD Documentation

## GitHub Actions CI Workflow

The project uses GitHub Actions for continuous integration, ensuring code quality and correctness for both Go and Rust codebases.

### Workflow Overview

**File**: `.github/workflows/ci.yml`

The CI pipeline runs on:
- Push to `main` or `master` branches
- Pull requests targeting `main` or `master`

### Jobs

#### 1. License Headers Check
Validates that all Go and Rust files contain proper license headers.

#### 2. Go CI
**Matrix Strategy**: Tests across multiple Go versions (1.21, 1.22, 1.23) and OS platforms (Ubuntu, macOS, Windows)

**Steps**:
- Dependency verification (`go mod verify`)
- Code formatting check (`gofmt`)
- Static analysis (`go vet`)
- Linting with `golangci-lint` (blocks merge on failure)
- Race condition detection (`go test -race`)
- Build verification

**Caching**: Go build cache and module cache enabled

#### 3. Documentation Spellcheck
Validates markdown files for spelling errors using `misspell`.

#### 4. Rust CI
**Matrix Strategy**: Tests across multiple Rust versions (stable, 1.75, 1.76)

**Steps**:
- Code formatting check (`cargo fmt`)
- Linting with Clippy (blocks merge on warnings)
- Test execution (`cargo test`)
- Build verification

**Caching**: Cargo registry and target directory

### Linting Configuration

**Go** (`.golangci.yml`):
- unused
- ineffassign
- gofmt
- goimports
- govet
- errcheck

**Rust**: Clippy with `-D warnings` (all warnings treated as errors)

### Local Testing

Run CI checks locally before pushing:

```bash
./scripts/test-ci-locally.sh
```

This script runs:
- Go: formatting, vet, tests, build
- Rust: formatting, clippy, tests, build

### Testing CI Failure Detection

To verify CI properly catches failures:

1. **Go Test Failure**:
   ```bash
   # Edit internal/decoder/ci_test.go
   # Uncomment the t.Fail() line
   git add internal/decoder/ci_test.go
   git commit -m "test: verify CI failure detection"
   git push
   ```

2. **Go Formatting Failure**:
   ```bash
   # Add unformatted code
   echo "func bad(){return}" >> cmd/erst/main.go
   git add cmd/erst/main.go
   git commit -m "test: verify formatting check"
   git push
   ```

3. **Rust Clippy Failure**:
   ```bash
   # Add code that triggers clippy warning
   cd simulator
   # Edit a file to add unused variable
   git add .
   git commit -m "test: verify clippy check"
   git push
   ```

### Viewing CI Results

1. Navigate to the repository on GitHub
2. Click the "Actions" tab
3. Select the workflow run
4. View job results and logs

### Branch Protection

Recommended branch protection rules for `main`:
- Require status checks to pass before merging
- Required checks:
  - `Go CI (1.23, ubuntu-latest)`
  - `Rust CI (stable)`
  - `License Headers Check`
  - `Docs Spellcheck`

### Troubleshooting

**Go tests fail locally but pass in CI**:
- Check Go version: `go version`
- Ensure dependencies are up to date: `go mod tidy`

**Rust clippy warnings**:
- Run locally: `cd simulator && cargo clippy --all-targets --all-features -- -D warnings`
- Fix warnings before pushing

**Cache issues**:
- CI caches may be stale
- Clear cache by updating `Cargo.lock` or `go.sum`

### Performance

**Typical run times**:
- License check: ~10s
- Go CI (single matrix): ~2-3 min
- Rust CI (single matrix): ~3-5 min
- Docs spellcheck: ~15s

**Total matrix runs**: 9 Go jobs (3 versions × 3 OS) + 3 Rust jobs = 12 parallel jobs

### Maintenance

**Updating Go versions**:
Edit `.github/workflows/ci.yml`:
```yaml
matrix:
  go-version: ["1.22", "1.23", "1.24"]  # Update versions
```

**Updating Rust versions**:
```yaml
matrix:
  rust-version: [stable, "1.76", "1.77"]  # Update versions
```

**Adding new linters**:
- Go: Edit `.golangci.yml`
- Rust: Modify clippy args in workflow
