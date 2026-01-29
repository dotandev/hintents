.PHONY: build test lint lint-unused test-unused validate-ci validate-interface clean
.PHONY: build test lint lint-unused test-unused validate-ci clean
.PHONY: build test lint validate-errors clean

# Build the main binary
build:
	go build -o bin/erst ./cmd/erst

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
