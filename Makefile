.PHONY: build test lint lint-strict lint-unused test-unused validate-ci validate-interface clean
.PHONY: rust-lint rust-lint-strict rust-test rust-build lint-all-strict
.PHONY: build test lint validate-errors clean bench bench-rpc bench-sim bench-profile
.PHONY: fmt fmt-go fmt-rust pre-commit

# Build variables
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_SHA?=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%d %H:%M:%S UTC")

# Go build flags
LDFLAGS=-ldflags "-X 'github.com/dotandev/hintents/internal/cmd.Version=$(VERSION)' \
                  -X 'github.com/dotandev/hintents/internal/cmd.CommitSHA=$(COMMIT_SHA)' \
                  -X 'github.com/dotandev/hintents/internal/cmd.BuildDate=$(BUILD_DATE)'"

# Build the main binary
build:
	go build $(LDFLAGS) -o bin/erst ./cmd/erst

# Build for release (optimized)
build-release:
	go build $(LDFLAGS) -ldflags "-s -w" -o bin/erst ./cmd/erst

# Run tests
test:
	go test ./...

# Run full linter suite
lint:
	golangci-lint run

# Run strict linting (fail on all warnings)
lint-strict:
	@echo "Running strict Go linting..."
	@golangci-lint run --config=.golangci.yml --max-issues-per-linter=0 --max-same-issues=0
	@go vet ./...
	@echo " Strict linting passed"

# Run unused code detection
lint-unused:
	./scripts/lint-unused.sh

# Test unused code detection setup
test-unused:
	./scripts/test-unused-detection.sh

# Validate CI/CD configuration
validate-ci:
	./scripts/validate-ci.sh
# Validate error standardization
validate-errors:
	./scripts/validate-errors.sh

# Validate interface implementation
validate-interface:
	./scripts/validate-interface.sh

# Clean build artifacts
clean:
	rm -rf bin/
	go clean -cache

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run benchmarks
bench:
	go test -bench=. -benchmem ./internal/rpc ./internal/simulator

# Run RPC benchmarks only
bench-rpc:
	go test -bench=. -benchmem ./internal/rpc

# Run simulator benchmarks only
bench-sim:
	go test -bench=. -benchmem ./internal/simulator

# Run benchmarks with CPU profiling
bench-profile:
	go test -bench=. -benchmem -cpuprofile=cpu.prof ./internal/rpc ./internal/simulator

# Rust simulator targets
.PHONY: rust-lint rust-lint-strict rust-test rust-build

# Run Rust linting
rust-lint:
	cd simulator && cargo clippy --all-targets --all-features

# Run strict Rust linting (fail on all warnings)
rust-lint-strict:
	@echo "Running strict Rust linting..."
	@cd simulator && cargo clippy --all-targets --all-features -- \
		-D warnings \
		-D clippy::all \
		-D unused-variables \
		-D unused-imports \
		-D unused-mut \
		-D dead-code \
		-D unused-assignments \
		-W clippy::pedantic \
		-W clippy::nursery
	@echo " Strict Rust linting passed"

# Run Rust tests
rust-test:
	cd simulator && cargo test --verbose

# Build Rust simulator
rust-build:
	cd simulator && cargo build --verbose

# Run all strict linting (Go + Rust)
lint-all-strict: lint-strict rust-lint-strict
	@echo " All strict linting passed"

# ──────────────────────────────────────────────
# Formatting targets
# ──────────────────────────────────────────────

# Format Go files (gofmt + goimports)
fmt-go:
	@echo "Formatting Go files..."
	@gofmt -w .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "⚠  goimports not found. Install: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi
	@echo "✓ Go formatting done"

# Format Rust files (cargo fmt)
fmt-rust:
	@echo "Formatting Rust files..."
	@cd simulator && cargo fmt
	@echo "✓ Rust formatting done"

# Format everything (Go + Rust)
fmt: fmt-go fmt-rust
	@echo "✓ All formatting done"

# ──────────────────────────────────────────────
# Pre-commit setup
# ──────────────────────────────────────────────

# Install pre-commit hooks
pre-commit:
	@echo "Setting up pre-commit hooks..."
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit install; \
		echo "✓ Pre-commit hooks installed"; \
	else \
		echo "⚠  pre-commit not found. Install: pip install pre-commit"; \
		exit 1; \
	fi
