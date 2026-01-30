.PHONY: build test lint lint-unused test-unused validate-ci validate-interface clean
.PHONY: build test lint lint-unused test-unused validate-ci clean
.PHONY: build test lint validate-errors clean

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
