# Contributing to ERST

Thank you for your interest in contributing to the **Stellar ERST** (Execution Replay & Simulation Tool)! We welcome community contributions, from bug fixes and new features to documentation improvements.

## 1. Prerequisites

To build and run ERST locally, you will need:
- **Go** (1.25.0 or later)
- **Rust** (1.87 or later, with Cargo)
- **Node.js** (LTS version, required if working on the UI or WebAssembly plugins)

## 2. Setting Up Your Environment

1. **Clone the repository**:
   ```bash
   git clone https://github.com/dotandev/hintents.git
   cd hintents
   ```

2. **Build the Rust Simulator**:
   The `erst` CLI relies on the Rust-based WASM simulator.
   ```bash
   cd simulator
   cargo build
   ```

3. **Build the Go CLI**:
   Return to the project root and build the CLI binary.
   ```bash
   cd ..
   go build -o erst ./cmd/erst
   ```

## 3. Coding Guidelines

### Go
- Use `go fmt ./...` to format all code before committing.
- Run `go vet ./...` to catch common mistakes.
- We strictly adhere to `golangci-lint`. You can run it locally with `golangci-lint run --config=.golangci.yml`.

### Rust
- Use `cargo fmt` to format Rust code.
- Run `cargo clippy --all-targets --all-features -- -D warnings` to catch linting errors.

## 4. Testing

Please ensure all tests pass before submitting a Pull Request:
- **Go Tests**: `go test -v -race ./...`
- **Rust Tests**: `cargo test` inside the `simulator/` directory.

If you are adding a new feature, please add test coverage. We require tests for any new functionality to maintain stability.

## 5. Pull Request Process

1. **Create a branch**: `git checkout -b feat/your-feature-name` or `fix/your-fix-name`.
2. **Follow Conventional Commits**: We use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) for our commit messages to automatically generate Changelogs.
   - Good: `feat: add support for dynamic budget`
   - Good: `fix: correct crash on empty payload`
   - Bad: `fixed bug`
3. **Submit a PR**: Use the provided GitHub Pull Request Template. Ensure you link to any relevant open issues.
4. **CI Checks**: Wait for the CI pipeline (GitHub Actions) to run. If it fails, please investigate and fix the issues.
5. **Code Review**: At least one maintainer must review and approve your PR before it is merged into `main`.

Thank you for making ERST better!
