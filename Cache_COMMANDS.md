# Command Reference - LedgerKey Cache Hashing Tests

## Quick Start

### Run All Tests

```bash
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -v
```

### Run Manual Verification

```bash
# Basic verification (1,000 iterations)
go run verify_hashing.go

# Comprehensive verification (4,000 iterations, 4 key types)
go run verify_hashing_comprehensive.go
```

---

## Test Commands

### Unit Tests

#### Run all cache tests

```bash
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -v
```

#### Run specific test

```bash
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -run TestLedgerKeyHashing_Deterministic -v
```

#### Run with race detection

```bash
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -race
```

#### Stability test (multiple runs)

```bash
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -count=10
```

#### Run entire package tests

```bash
go test ./internal/rpc -v
```

---

## Benchmark Commands

### Run all benchmarks

```bash
go test ./internal/rpc -bench=BenchmarkLedgerKeyHashing -benchmem
```

### Run specific benchmark

```bash
go test ./internal/rpc -bench=BenchmarkLedgerKeyHashing_ContractData -benchmem
```

---

## Coverage Commands

### Generate coverage report

```bash
go test ./internal/rpc -coverprofile=coverage.out
```

### View coverage in terminal

```bash
go tool cover -func=coverage.out
```

### View coverage in browser

```bash
go tool cover -html=coverage.out
```

### Coverage for cache.go only

```bash
go test ./internal/rpc -coverprofile=coverage.out
go tool cover -func=coverage.out | grep cache.go
```

---

## Verification Commands

### Basic verification (single key type)

```bash
go run verify_hashing.go
```

**Expected Output:**

```
SUCCESS: Consistent hash: 8b283a7411fb24e3540781a645517a01832265d055ba7d0ff3de20b6455c526b
```

### Comprehensive verification (multiple key types)

```bash
go run verify_hashing_comprehensive.go
```

**Expected Output:**

```
=== Comprehensive Hash Consistency Verification ===

 Account Key: SUCCESS
   Hash: 8b283a7411fb24e3540781a645517a01832265d055ba7d0ff3de20b6455c526b
 Trustline Key (USDC): SUCCESS
   Hash: 9a1e801b0c33aa25fd3a13d40f2ee71ac62bafc88c2d95e520e962115d7f0515
 Offer Key: SUCCESS
   Hash: 7f6843b658c09b807599243329053869de1a53bf629cb6086230526ccd026ab9
 Contract Data Key: SUCCESS
   Hash: 5440938817031b18a8f20ccf1cd25a15f20cccf9c9bf9488c2bce2c4652b6499

All tests passed! Hash consistency verified across 4,000 operations.
```

### Multiple verification runs

```bash
for i in {1..5}; do echo "=== Run $i ===" && go run verify_hashing.go; done
```

---

## Diagnostic Commands

### Check for compilation errors

```bash
go build ./internal/rpc
```

### Run tests with verbose output

```bash
go test ./internal/rpc -v -run TestLedgerKeyHashing 2>&1 | tee test_output.log
```

### Check test execution time

```bash
time go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go
```

### List all test functions

```bash
go test ./internal/rpc -list=Test
```

### List all benchmarks

```bash
go test ./internal/rpc -list=Benchmark
```

---

## CI/CD Commands

### Full test suite with coverage

```bash
go test ./internal/rpc -v -race -coverprofile=coverage.out
go tool cover -func=coverage.out
```

### Quick validation

```bash
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go && \
go run verify_hashing.go && \
echo " All validations passed"
```

### Complete validation pipeline

```bash
#!/bin/bash
set -e

echo "Running unit tests..."
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -v

echo "Running race detection..."
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -race

echo "Running benchmarks..."
go test ./internal/rpc -bench=BenchmarkLedgerKeyHashing -benchmem

echo "Running manual verification..."
go run verify_hashing.go

echo "Running comprehensive verification..."
go run verify_hashing_comprehensive.go

echo "Generating coverage report..."
go test ./internal/rpc -coverprofile=coverage.out
go tool cover -func=coverage.out | grep cache.go

echo " All validations completed successfully!"
```

---

## Troubleshooting Commands

### If tests fail to compile

```bash
# Check Go version
go version

# Download dependencies
go mod download

# Verify module integrity
go mod verify

# Clean build cache
go clean -cache
```

### If tests are slow

```bash
# Run with timeout
go test ./internal/rpc -timeout 30s

# Run specific test only
go test ./internal/rpc -run TestLedgerKeyHashing_Deterministic
```

### If getting "undefined" errors

```bash
# Make sure to include both files
go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -v

# Or run the entire package
go test ./internal/rpc -v
```

---

## Development Commands

### Watch mode (requires entr or similar)

```bash
# Install entr: sudo apt install entr (Linux) or brew install entr (macOS)
ls internal/rpc/*.go | entr -c go test ./internal/rpc -v
```

### Format code

```bash
go fmt ./internal/rpc/...
```

### Lint code (requires golangci-lint)

```bash
golangci-lint run ./internal/rpc/...
```

### Generate test coverage badge

```bash
go test ./internal/rpc -coverprofile=coverage.out
go tool cover -func=coverage.out | grep "total:" | awk '{print $3}'
```

---

## Quick Reference

| Task                       | Command                                                              |
| -------------------------- | -------------------------------------------------------------------- |
| Run all tests              | `go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -v`    |
| Run with race detection    | `go test ./internal/rpc/cache_test.go ./internal/rpc/cache.go -race` |
| Run benchmarks             | `go test ./internal/rpc -bench=. -benchmem`                          |
| Generate coverage          | `go test ./internal/rpc -coverprofile=coverage.out`                  |
| Manual verification        | `go run verify_hashing.go`                                           |
| Comprehensive verification | `go run verify_hashing_comprehensive.go`                             |

---

## Expected Results

All commands should complete successfully with:

-  PASS status
-  0 failures
-  0 race conditions
-  Consistent hash outputs
-  Execution time < 1 second

If any command fails, refer to the troubleshooting section or review the test output for specific error messages.
