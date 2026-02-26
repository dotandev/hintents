# Strict Linting Pipeline

This document describes the strict linting configuration for the ERST project, designed to maintain a pristine codebase by failing CI immediately on unused variables and heavy warnings.

## Overview

The project enforces strict linting rules for both Go and Rust code:

- **Zero tolerance** for unused variables, imports, and dead code
- **Fail fast** on any linting warnings in CI
- **Consistent enforcement** across all environments (local, CI, pre-commit)

## Go Linting (golangci-lint)

### Configuration

The Go linting configuration is defined in `.golangci.yml` with the following strict rules:

- `unused`: Detects unused variables, functions, constants, and types
- `ineffassign`: Detects ineffectual assignments
- `govet`: Comprehensive static analysis including shadow variable detection
- `staticcheck`: Advanced static analysis
- `gosimple`: Suggests code simplifications
- `deadcode`: Detects unreachable code
- `varcheck`: Finds unused global variables and constants
- `structcheck`: Finds unused struct fields

### Running Locally

```bash
# Standard linting
make lint

# Strict linting (recommended before committing)
make lint-strict

# Or directly with golangci-lint
golangci-lint run --config=.golangci.yml --max-issues-per-linter=0 --max-same-issues=0
```

### CI Enforcement

The CI pipeline runs strict linting on:
- Ubuntu with Go 1.23 (primary enforcement)
- All linting issues cause immediate CI failure
- No suppression of unused variable warnings

## Rust Linting (Clippy)

### Configuration

Rust linting is configured in `simulator/Cargo.toml` with strict lint levels:

```toml
[lints.rust]
unused_variables = "deny"
unused_imports = "deny"
unused_mut = "deny"
dead_code = "deny"
unused_assignments = "deny"

[lints.clippy]
all = "deny"
pedantic = "warn"
nursery = "warn"
```

### Running Locally

```bash
# Standard Rust linting
make rust-lint

# Strict Rust linting (recommended before committing)
make rust-lint-strict

# Or directly with cargo
cd simulator
cargo clippy --all-targets --all-features -- \
  -D warnings \
  -D clippy::all \
  -D unused-variables \
  -D unused-imports \
  -D unused-mut \
  -D dead-code \
  -D unused-assignments \
  -W clippy::pedantic \
  -W clippy::nursery
```

### CI Enforcement

The CI pipeline runs strict Clippy checks on:
- Stable Rust toolchain
- All warnings treated as errors
- Pedantic and nursery lints enabled as warnings

## Combined Linting

Run all strict linting checks at once:

```bash
make lint-all-strict
```

Or use the dedicated script:

```bash
./scripts/lint-strict.sh
```

## Pre-commit Hooks

Install pre-commit hooks to catch issues before committing:

```bash
# Install pre-commit (if not already installed)
pip install pre-commit

# Install the hooks
pre-commit install

# Run manually on all files
pre-commit run --all-files
```

The pre-commit configuration (`.pre-commit-config.yaml`) runs:
- golangci-lint with strict settings
- go vet
- cargo clippy with strict settings
- cargo fmt check
- go fmt check

## Suppressing False Positives

### When to Suppress

Lints should **only** be suppressed when they are objectively false positives. Examples:

- Generated code that cannot be modified
- External dependencies with unavoidable warnings
- Legitimate cases where the lint rule doesn't apply

### How to Suppress

#### Go

Use `//nolint` comments sparingly and always with justification:

```go
//nolint:unused // Kept for future API compatibility
func futureFunction() {}
```

Or add specific exclusions in `.golangci.yml`:

```yaml
issues:
  exclude-rules:
    - path: path/to/file.go
      linters:
        - unused
      text: "specific pattern to exclude"
```

#### Rust

Use `#[allow]` attributes with clear justification:

```rust
#[allow(dead_code)] // Required for FFI interface
fn internal_function() {}
```

Or configure in `Cargo.toml`:

```toml
[lints.rust]
specific_lint = { level = "allow", priority = 1 }
```

## CI Workflow

The strict linting pipeline runs in the following order:

1. **License header check** - Ensures all files have proper headers
2. **Go linting** (parallel with Rust)
   - Format check (`gofmt`)
   - `go vet` analysis
   - `golangci-lint` with strict settings
   - Unused variable detection
3. **Rust linting** (parallel with Go)
   - Format check (`cargo fmt`)
   - Clippy with strict settings
4. **Tests** - Only run if linting passes

Any failure in steps 1-3 causes immediate CI failure without running subsequent steps.

## Troubleshooting

### Common Issues

**Unused variable in test file:**
```go
// Bad
func TestSomething(t *testing.T) {
    result := doSomething()
    // result not used
}

// Good
func TestSomething(t *testing.T) {
    _ = doSomething() // Explicitly ignore
}
```

**Unused import:**
```go
// Remove unused imports or use goimports
import (
    "fmt" // Remove if not used
)
```

**Dead code in Rust:**
```rust
// Either use the code or remove it
// Don't suppress unless there's a valid reason
fn unused_function() {} // Remove this
```

### Getting Help

If you encounter a lint error that seems incorrect:

1. Read the lint documentation to understand the rule
2. Check if it's a genuine issue that should be fixed
3. If it's a false positive, document why in the suppression comment
4. Discuss in PR review if unsure

## Benefits

This strict linting approach provides:

- **Early detection** of bugs and code quality issues
- **Consistent code quality** across the entire codebase
- **Reduced technical debt** by preventing accumulation of warnings
- **Better maintainability** through cleaner, more intentional code
- **Faster reviews** by automating quality checks

## References

- [golangci-lint documentation](https://golangci-lint.run/)
- [Clippy lint list](https://rust-lang.github.io/rust-clippy/master/)
- [Go vet documentation](https://pkg.go.dev/cmd/vet)
- [Rust lint levels](https://doc.rust-lang.org/rustc/lints/levels.html)
