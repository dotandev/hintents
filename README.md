# Erst By Hintents

**Erst** is a specialized developer tool for the Stellar network, designed to solve the "black box" debugging experience on Soroban.

> **Status**: Active Development (Pre-Alpha)
> **Focus**: Soroban Error Decoding & Transaction Replay

## Scope & Objective

The primary goal of `erst` is to clarify **why** a Stellar smart contract transaction failed.

Currently, when a Soroban transaction fails on mainnet, developers receive a generic XDR error code. `erst` aims to bridge the gap between this opaque network error and the developer's source code.

**Core Features (Planned):**

1.  **Transaction Replay**: Fetch a failed transaction's envelope and ledger state from an RPC provider.
2.  **Local Simulation**: Re-execute the transaction logically in a local environment.
3.  **Trace decoding**: Map execution steps and failures back to readable instructions or Rust source lines.

## Usage (MVP)

### Debugging a Transaction

Fetches a transaction envelope from the Stellar Public network and prints its XDR size (Simulation pending).

```bash
./erst debug <transaction-hash> --network testnet
```

### Interactive Trace Viewer

Launch an interactive terminal UI to explore transaction execution traces with search functionality.

```bash
./erst debug <transaction-hash> --interactive
# or
./erst debug <transaction-hash> -i
```

**Features:**

- **Search**: Press `/` to search through traces (contract IDs, functions, errors)
- **Tree Navigation**: Expand/collapse nodes, navigate with arrow keys
- **Syntax Highlighting**: Color-coded contract IDs, functions, and errors
- **Fast Navigation**: Jump between search matches with `n`/`N`
- **Match Counter**: See "Match 2 of 5" status while searching

See [internal/trace/README.md](internal/trace/README.md) for detailed documentation.

### Audit log signing (software / HSM)

`erst` includes a small utility command to generate a deterministic, signed audit log from a JSON payload.

#### Software signing (Ed25519 private key)

Provide a PKCS#8 PEM Ed25519 private key via env or CLI:

- Env: `ERST_AUDIT_PRIVATE_KEY_PEM`
- CLI: `--software-private-key <pem>`

Example:

```bash
node dist/index.js audit:sign \
  --payload '{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}' \
  --software-private-key "$(cat ./ed25519-private-key.pem)"
```

#### PKCS#11 HSM signing

Select the PKCS#11 provider with `--hsm-provider pkcs11` and configure the module/token/key via env vars.

Required env vars:

- `ERST_PKCS11_MODULE` (path to the PKCS#11 module `.so`)
- `ERST_PKCS11_PIN`
- `ERST_PKCS11_KEY_LABEL` **or** `ERST_PKCS11_KEY_ID` (hex)
- `ERST_PKCS11_PUBLIC_KEY_PEM` (SPKI PEM public key for verification/audit metadata)

Optional:

- `ERST_PKCS11_SLOT` (numeric index into the slot list)
- `ERST_PKCS11_TOKEN_LABEL`

Example:

```bash
export ERST_PKCS11_MODULE=/usr/lib/softhsm/libsofthsm2.so
export ERST_PKCS11_PIN=1234
export ERST_PKCS11_KEY_LABEL=erst-audit-ed25519
export ERST_PKCS11_PUBLIC_KEY_PEM="$(cat ./ed25519-public-key-spki.pem)"

node dist/index.js audit:sign \
  --hsm-provider pkcs11 \
  --payload '{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}'
```

The command prints the signed audit log JSON to stdout so it can be redirected to a file.

## Documentation

- **[Architecture Overview](docs/architecture.md)**: Deep dive into how the Go CLI communicates with the Rust simulator, including data flow, IPC mechanisms, and design decisions.
- **[Project Proposal](docs/proposal.md)**: Detailed project proposal and roadmap.

## Technical Analysis

### The Challenge

Stellar's `soroban-env-host` executes WASM. When it traps (crashes), the specific reason is often sanitized or lost in the XDR result to keep the ledger size small.

### The Solution Architecture

`erst` operates by:

1.  **Fetching Data**: Using the Stellar RPC to get the `TransactionEnvelope` and `LedgerFootprint` (read/write set) for the block where the tx failed.
2.  **Simulation Environment**: A Rust binary (`erst-sim`) that integrates with `soroban-env-host` to replay transactions.
3.  **Execution**: Feeding the inputs into the VM and capturing `diagnostic_events`.

For a detailed explanation of the architecture, see [docs/architecture.md](docs/architecture.md).

## How to Contribute

We are building this open-source to help the entire Stellar community.

### Prerequisites

- Go 1.21+
- Rust (for building the simulator binary)
- Stellar CLI (for comparing results)

### Getting Started

1.  Clone the repo:
    ```bash
    git clone https://github.com/dotandev/hintents.git
    cd hintents
    ```
2.  Build the Rust simulator:
    ```bash
    cd simulator
    cargo build --release
    cd ..
    ```
3.  Run tests:
    ```bash
    go test ./...
    ```

### Development Roadmap

See [docs/proposal.md](docs/proposal.md) for the detailed proposal.

1.  [ ] **Phase 1**: Research RPC endpoints for fetching historical ledger keys.
2.  [ ] **Phase 2**: Build a basic "Replay Harness" that can execute a loaded WASM file.
3.  [ ] **Phase 3**: Connect the harness to live mainnet data.

---

_Erst is an open-source initiative. Contributions, PRs, and Issues are welcome._
