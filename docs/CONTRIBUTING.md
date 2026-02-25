# Contributing to Erst

Thank you for your interest in contributing to Erst! We welcome contributions from the community to help make Stellar debugging better for everyone.

## Getting Started

1.  **Fork the repository** on GitHub.


2.  **Clone your fork** locally:
    ```bash
    git clone https://github.com/your-username/hintents.git
    cd hintents
    ```

3.  **Sync with the latest `main` branch**:
    ```bash
    git checkout main
    git pull origin main
    ```

4.  **Create a new branch** for your feature or bug fix (always branch from `main`):
    ```bash
    git checkout -b feature/my-new-feature
    ```

## Feature Requests & Voting

We believe the community should drive the project's priorities. Here's how you can participate:

### Requesting a New Feature

1.  **Check existing requests**: Before creating a new feature request, browse [GitHub Discussions](https://github.com/dotandev/hintents/discussions) (under the "Feature Requests" category) to see if someone has already suggested it.
2.  **Create a discussion**: If your idea is new, start a discussion in the **Feature Requests** category with:
    -   A clear, descriptive title
    -   The problem or use case you're trying to solve
    -   Your proposed solution or approach
    -   Any relevant examples or context
3.  **Engage with the community**: Respond to questions and feedback to help refine the idea.

> **Note**: If GitHub Discussions are not yet enabled, please open a regular [GitHub Issue](https://github.com/dotandev/hintents/issues) with the `feature-request` label instead.

### Voting on Features

-   **Use reactions**: Vote for features you'd like to see by adding a 👍 (thumbs up) reaction to the original discussion post.
-   **Avoid "+1" comments**: Please use reactions instead of comments to keep discussions focused.
-   **Priority ranking**: Features with the most 👍 reactions will be prioritized in our development roadmap.

### From Discussion to Implementation

1.  Popular feature requests will be reviewed by maintainers and converted into GitHub Issues when approved.
2.  Approved issues will be labeled with `feature` and `community-requested`.
3.  You're welcome to implement features yourself! Comment on the issue to let others know you're working on it.

### Tips for Great Feature Requests

-   **Be specific**: Vague requests are hard to implement.
-   **Explain the "why"**: Help us understand the problem you're solving.
-   **Consider scope**: Smaller, focused features are easier to review and merge.
-   **Think about compatibility**: How does this fit with Stellar/Soroban's ecosystem?

## Development Workflow

Erst consists of two parts that talk to each other:
1.  **Go CLI (`cmd/erst`)**: The user-facing tool.
2.  **Rust Simulator (`simulator/`)**: The core logic that replays transactions using `soroban-env-host`.

### Prerequisites

You will need the following installed:

*   **Go**: Version 1.21 or later. [Download Go](https://go.dev/dl/)
*   **Rust**: Standard Stable Toolchain. [Install Rust](https://www.rust-lang.org/tools/install)
*   **(Optional) Docker**: If you prefer building in a container.

### Detailed Local Development Setup

#### 1. Rust Simulator Setup

The simulator must be built first because the Go CLI depends on the binary being available (or in the path).

```bash
# From the project root
cd simulator

# Ensure the latest stable Rust toolchain is installed
rustup update stable

# Install required Rust components (if not already installed)
rustup component add rustfmt clippy

# Build the release binary (recommended for performance)
cargo build --release
```

**Note**: The binary will be located at `simulator/target/release/erst-sim`.

#### 2. Go CLI Setup

Once the simulator is built, you can build and run the Go CLI.

```bash
# Return to the project root
cd ..

# Download Go dependencies
go mod tidy
go mod download

# Build the CLI binary
go build -o erst cmd/erst/main.go
```

To run the CLI and have it find the simulator, you can either:
- Set `ERST_SIMULATOR_PATH` to the absolute path of the rust binary.
- Or rely on the automatic detection which looks in `simulator/target/release/erst-sim` (useful for dev).

```bash
# Verify it works
./erst --help
```

### Running Tests

**Go Tests:**
```bash
go test ./...
```

**Rust Tests:**
```bash
cd simulator
cargo test
```
> Always run both Go and Rust tests before submitting a pull request.


## Submitting a Pull Request

1.  **Run strict linting**: Ensure all linting checks pass before submitting.
    ```bash
    make lint-all-strict
    ```
    See [STRICT_LINTING.md](STRICT_LINTING.md) for details on our linting requirements.

2.  **Ensure all tests pass**: Run the full test suite.
    ```bash
    make test
    make rust-test
    ```

3.  **Update documentation** if you change functionality.

4.  **Submit your PR** to the `main` branch.

### Code Quality Requirements

All PRs must pass strict linting checks:

- **No unused variables, imports, or functions**
- **No dead code** (unless explicitly justified)
- **All warnings treated as errors** in CI
- **Proper formatting** (gofmt for Go, cargo fmt for Rust)

The CI pipeline will automatically fail if any linting issues are detected. To avoid CI failures:

- Run `make lint-all-strict` locally before pushing
- Install pre-commit hooks: `pip install pre-commit && pre-commit install`
- Fix all linting issues before requesting review

For details on suppressing false positives, see [STRICT_LINTING.md](STRICT_LINTING.md).

## License

By contributing, you agree that your contributions will be licensed under the Apache License, Version 2.0.
