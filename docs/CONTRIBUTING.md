# Contributing to Erst

Thank you for your interest in contributing to Erst! We welcome contributions from the community to help make Stellar debugging better for everyone.

## Getting Started

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** locally:
    ```bash
    git clone https://github.com/your-username/erst.git
    cd erst
    ```
3.  **Create a branch** for your feature or bug fix:
    ```bash
    git checkout -b feature/my-new-feature
    ```

## Development Workflow

Erst consists of two parts that talk to each other:
1.  **Go CLI (`cmd/erst`)**: The user-facing tool.
2.  **Rust Simulator (`simulator/`)**: The core logic that replays transactions using `soroban-env-host`.

### Prerequisites

You will need the following installed:

*   **Go**: Version 1.21 or later. [Download Go](https://go.dev/dl/)
*   **Rust**: Standard Stable Toolchain. [Install Rust](https://www.rust-lang.org/tools/install)
*   **(Optional) Docker**: If you prefer building in a container.

### detailed Setup

#### 1. Rust Simulator Setup

The simulator must be built first because the Go CLI depends on the binary being available (or in the path).

```bash
cd simulator
# Ensure you have the latest stable toolchain
rustup update stable
# Build the release binary (release is recommended for performance)
cargo build --release
```

**Note**: The binary will be located at `simulator/target/release/erst-sim`.

#### 2. Go CLI Setup

Once the simulator is built, you can build and run the Go CLI.

```bash
# From the project root
go mod download
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

## Submitting a Pull Request

1.  Ensure all tests pass.
2.  Update documentation if you change functionality.
3.  Submit your PR to the `main` branch.

## License

By contributing, you agree that your contributions will be licensed under the Apache License, Version 2.0.
