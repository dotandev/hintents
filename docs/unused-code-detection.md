# Unused Code Detection Setup

This document describes the unused code detection setup for the Erst project.

## Configuration

The project uses `golangci-lint` with the `unused` linter enabled to detect and remove unused functions, variables, and constants.

### Configuration File: `.golangci.yml`

The configuration includes:
- **unused linter**: Detects unused code elements
- **Additional linters**: ineffassign, misspell, gofmt, goimports, govet, errcheck
- **Exclusions**: Test files are excluded from unused checks for interface implementations

### Key Settings

```yaml
linters-settings:
  unused:
    check-exported: false    # Don't report unused exported identifiers (may be used externally)
    check-arguments: true    # Check function arguments
    check-fields: true       # Check struct fields
```

## Usage

### Manual Execution

```bash
# Run only unused linter
golangci-lint run --enable unused --disable-all

# Run full linter suite
golangci-lint run
```

### Using Make Targets

```bash
# Run unused code detection
make lint-unused

# Run full linter suite
make lint
```

### Using the Script

```bash
./scripts/lint-unused.sh
```

## Current Status

**Note**: Due to a Go toolchain version mismatch (go1.25.4 vs go1.25.5), the linters cannot currently run. This needs to be resolved before the unused code detection can be executed.

### Resolving the Toolchain Issue

1. Ensure Go version consistency:
   ```bash
   go version  # Should match the toolchain version
   ```

2. Clean and rebuild:
   ```bash
   go clean -cache
   go mod tidy
   ```

3. If the issue persists, reinstall Go or update the toolchain.

## Manual Code Analysis

Based on manual analysis of the codebase, the following observations were made:

### Potentially Unused Elements

1. **Network Constants**: All network types (Testnet, Mainnet, Futurenet) are used in the CLI
2. **RPC Functions**: All client functions are used or part of the public API
3. **Test Helpers**: Mock implementations in test files are necessary for testing

### Code Quality

The codebase is already quite lean with minimal unused code. Most functions serve specific purposes:

- `main.go`: Entry point
- `debug.go`: CLI command implementation
- `client.go`: RPC client functionality
- `simulator/`: Simulation logic
- `decoder/`: XDR decoding utilities
- `logger/`: Logging infrastructure

## Integration with CI/CD

Once the toolchain issue is resolved, add the following to your CI pipeline:

```yaml
- name: Run unused code detection
  run: make lint-unused
```

## Best Practices

1. **Regular Checks**: Run unused code detection before each commit
2. **Careful Removal**: Ensure non-exported fields intended for future use aren't deleted
3. **Test Coverage**: Maintain test coverage when removing unused code
4. **Documentation**: Update documentation when removing public APIs

## Troubleshooting

### Common Issues

1. **Go Version Mismatch**: Ensure consistent Go version across toolchain
2. **False Positives**: Use exclusion rules for legitimate unused code
3. **Interface Implementations**: Test mocks may appear unused but are necessary

### Exclusion Patterns

Add to `.golangci.yml` if needed:

```yaml
issues:
  exclude-rules:
    - text: "specific pattern to exclude"
      linters:
        - unused
```
