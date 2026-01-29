# Erst Technical Architecture

## Overview

**Erst** is a Soroban debugging and error decoding tool for the Stellar network. It bridges the gap between opaque XDR error codes and developer-friendly transaction analysis through transaction replay and local simulation.

The architecture consists of three core components:
1. **Go CLI** (`erst`): Command-line interface for user interaction
2. **RPC Client**: Stellar network data fetching via Horizon API and JSON-RPC
3. **Rust Simulator** (`erst-sim`): Soroban transaction execution and diagnostics

---

## System Architecture Diagram

```mermaid
graph TB
    subgraph User["User Environment"]
        CLI["Go CLI<br/>(erst)"]
    end
    
    subgraph StellarNetwork["Stellar Network"]
        Horizon["Horizon REST API<br/>(State & Ledger Data)"]
        RPC["JSON-RPC<br/>(Advanced Queries)"]
    end
    
    subgraph LocalProcessing["Local Processing"]
        GoClient["RPC Client<br/>(Go)"]
        Runner["Simulator Runner<br/>(Process Manager)"]
    end
    
    subgraph Simulator["Rust Simulator<br/>(erst-sim)"]
        XDRDecoder["XDR Decoder"]
        Host["Soroban Host<br/>(VM)"]
        DiagnosticEngine["Diagnostic Engine"]
    end
    
    subgraph Output["Output & Analysis"]
        Events["Execution Events"]
        Logs["Diagnostic Logs"]
        ErrorTrace["Error Trace"]
    end
    
    CLI -->|User Input<br/>tx-hash| GoClient
    GoClient -->|Fetch EnvelopeXDR<br/>ResultMetaXDR<br/>LedgerEntries| Horizon
    GoClient -->|Advanced Queries| RPC
    GoClient -->|JSON Request<br/>stdin| Runner
    Runner -->|Spawn Process| Simulator
    XDRDecoder -->|Base64 Decode| Host
    Host -->|Execute WASM| DiagnosticEngine
    DiagnosticEngine --> Events
    DiagnosticEngine --> Logs
    DiagnosticEngine --> ErrorTrace
    Simulator -->|JSON Response<br/>stdout| Runner
    Runner -->|Parse Results| GoClient
    GoClient -->|Display| CLI
    CLI -->|Output| Output
```

---

## Data Flow: Transaction Debugging Workflow

```mermaid
sequenceDiagram
    participant User as Developer
    participant CLI as Go CLI
    participant Network as Stellar Network
    participant RPC as RPC Client
    participant Runner as Simulator Runner
    participant Sim as Rust Simulator
    
    User->>CLI: erst debug <tx-hash>
    activate CLI
    
    CLI->>RPC: Fetch transaction envelope
    activate RPC
    RPC->>Network: GET /transactions/<tx-hash>
    Network-->>RPC: TransactionEnvelope XDR
    RPC->>Network: Get result meta & ledger state
    Network-->>RPC: ResultMetaXDR, LedgerEntries
    RPC-->>CLI: Complete transaction context
    deactivate RPC
    
    CLI->>Runner: Create SimulationRequest
    Note over CLI,Runner: envelope_xdr, result_meta_xdr,<br/>ledger_entries (key→entry map)
    
    Runner->>Runner: Serialize to JSON
    Runner->>Sim: Pass JSON via stdin
    activate Sim
    
    Sim->>Sim: Decode EnvelopeXDR (Base64)
    Sim->>Sim: Decode ResultMetaXDR
    Sim->>Sim: Load LedgerEntries into Host Storage
    Sim->>Sim: Initialize Soroban Host VM
    Sim->>Sim: Execute Transaction
    
    Sim->>Sim: Capture diagnostic_events
    Sim->>Sim: Capture execution logs
    Sim->>Sim: Build execution trace
    
    Sim->>Runner: Output JSON (events, logs, status)
    deactivate Sim
    
    Runner->>CLI: Parse SimulationResponse
    CLI->>User: Display results & errors
    deactivate CLI
```

---

## IPC Protocol: Go ↔ Rust Communication

### Communication Method: stdin/stdout JSON Serialization

The Go CLI and Rust simulator communicate through a simple but effective protocol:

#### Request Format (Go → Rust)

```json
{
  "envelope_xdr": "base64-encoded-transaction-envelope",
  "result_meta_xdr": "base64-encoded-transaction-result-meta",
  "ledger_entries": {
    "base64-key-1": "base64-ledger-entry-1",
    "base64-key-2": "base64-ledger-entry-2"
  }
}
```

**Field Descriptions:**

| Field | Type | Purpose |
|-------|------|---------|
| `envelope_xdr` | String (Base64) | Complete signed transaction envelope ready for execution |
| `result_meta_xdr` | String (Base64) | Transaction result metadata from the blockchain (optional) |
| `ledger_entries` | Map (Base64 → Base64) | Read/write set of ledger entries at transaction time |

#### Response Format (Rust → Go)

```json
{
  "status": "success|error",
  "error": null or "error message",
  "events": ["event1", "event2"],
  "logs": ["log1", "log2"]
}
```

**Field Descriptions:**

| Field | Type | Purpose |
|-------|------|---------|
| `status` | String | Execution status: "success" or "error" |
| `error` | String \| Null | Error message if status is "error" |
| `events` | Array | Diagnostic events emitted during execution |
| `logs` | Array | Detailed execution logs for debugging |

### Process Flow

```mermaid
graph LR
    A["Go Client<br/>Serialize"] -->|stdin| B["Rust Process<br/>std input"]
    B -->|Deserialize| C["Parse JSON"]
    C -->|Validate| D["Prepare Simulation"]
    D -->|Execute| E["Run VM"]
    E -->|Collect| F["Events & Logs"]
    F -->|Serialize| G["Format JSON"]
    G -->|stdout| H["Go Client<br/>Receives"]
    H -->|Deserialize| I["Parse Results"]
    I -->|Display| J["User Output"]
    
    style A fill:#4A90E2
    style B fill:#E74C3C
    style I fill:#4A90E2
    style J fill:#2ECC71
```

---

## Component Details

### 1. Go RPC Client

**Location:** `internal/rpc/client.go`

**Responsibilities:**
- Establish connections to Stellar Horizon API
- Fetch transaction envelopes and metadata
- Query ledger state at specific transaction points
- Support multiple networks (Mainnet, Testnet, Futurenet)

**Key Functions:**

```go
// Client manages Stellar network interactions
type Client struct {
    Horizon horizonclient.ClientInterface
    Network Network
}

// NewClient creates network-specific RPC client
func NewClient(net Network) *Client

// Fetch transaction context
func (c *Client) GetTransaction(ctx context.Context, txHash string) (*TransactionResponse, error)

// Fetch ledger entries for simulation
func (c *Client) GetLedgerEntries(ctx context.Context, keys []string) (map[string]string, error)
```

**Network Support:**

```mermaid
graph TD
    Client["RPC Client"]
    Client -->|Mainnet| HorizonMain["https://horizon.stellar.org"]
    Client -->|Testnet| HorizonTest["https://horizon-testnet.stellar.org"]
    Client -->|Futurenet| HorizonFuture["https://horizon-futurenet.stellar.org"]
    
    Client -->|JSON-RPC| RPCAPI["Stellar RPC API<br/>(Future Enhancement)"]
```

### 2. Simulator Runner

**Location:** `internal/simulator/runner.go`

**Responsibilities:**
- Locate and execute the `erst-sim` Rust binary
- Manage subprocess lifecycle
- Handle IPC communication via stdin/stdout
- Deserialize simulation results

**Key Functions:**

```go
// Runner manages simulator subprocess execution
type Runner struct {
    BinaryPath string
}

// NewRunner creates runner with binary discovery
func NewRunner() (*Runner, error)

// Run executes simulation with request
func (r *Runner) Run(req *SimulationRequest) (*SimulationResponse, error)
```

**Binary Discovery Priority:**

```
1. ERST_SIMULATOR_PATH environment variable
2. ./erst-sim (current working directory)
3. ./simulator/target/release/erst-sim (development)
4. erst-sim (global PATH)
```

### 3. Rust Simulator (erst-sim)

**Location:** `simulator/src/main.rs`

**Responsibilities:**
- Decode XDR structures from Base64
- Initialize Soroban Host VM with ledger state
- Execute transaction and capture diagnostics
- Generate execution trace and error information

**Execution Pipeline:**

```mermaid
graph TD
    A["Read stdin<br/>JSON"] -->|Deserialize| B["SimulationRequest"]
    B -->|Extract| C["envelope_xdr<br/>result_meta_xdr<br/>ledger_entries"]
    C -->|Base64 Decode| D["TransactionEnvelope<br/>TransactionResultMeta"]
    D -->|Initialize| E["Soroban Host"]
    F["ledger_entries Map"] -->|Load| E
    E -->|Set DiagnosticLevel| G["Debug Mode"]
    G -->|Execute| H["Transaction VM"]
    H -->|Emit Events| I["diagnostic_events"]
    H -->|Emit Logs| J["execution_logs"]
    I --> K["SimulationResponse"]
    J --> K
    H -->|Capture| L["Status & Errors"]
    L --> K
    K -->|Serialize| M["JSON to stdout"]
```

---

## JSON-RPC vs Horizon API Usage

### When to Use Horizon API

**Strengths:**
- ✅ Simple REST interface with standard HTTP
- ✅ Broad ecosystem support and documentation
- ✅ Well-established for common queries

**Use Cases in Erst:**
- Fetching transaction details and envelopes
- Querying account information
- Accessing transaction history

**Endpoints:**
- `GET /transactions/{id}` — Get transaction by hash
- `GET /ledger_entries` — Query ledger state
- `GET /accounts/{account}` — Account details

### When to Use JSON-RPC (Future Enhancement)

**Strengths:**
- ✅ More granular access to ledger state
- ✅ Better support for historical queries
- ✅ Optimized for simulator needs

**Planned Use Cases:**
- Direct ledger entry queries by key
- Snapshot state at specific ledger close time
- Reduced payload sizes with selective field queries

**Implementation Plan:**
```
Phase 1: Current — Horizon API (stable, mature)
Phase 2: Hybrid — JSON-RPC for ledger queries
Phase 3: Future — JSON-RPC as primary data source
```

---

## State Fetching & Management

### Ledger State Snapshot

When debugging a transaction, Erst must capture the exact ledger state at the point of execution:

```mermaid
graph TB
    subgraph Blockchain["Blockchain State"]
        LC1["Ledger Close 1"]
        LC2["Ledger Close 2"]
        TxBlock["Ledger Close N<br/>(Transaction Block)"]
        LC3["Ledger Close N+1"]
    end
    
    subgraph Capture["State Capture"]
        TxHash["Transaction Hash"]
        LedgerSeq["Ledger Sequence"]
        ReadSet["Read Set<br/>(account, contract, storage)"]
        WriteSet["Write Set<br/>(post-transaction state)"]
    end
    
    TxBlock -->|Extract| TxHash
    TxBlock -->|Extract| LedgerSeq
    TxBlock -->|Extract| ReadSet
    TxBlock -->|Extract| WriteSet
    
    TxHash -->|Pass to Rust Sim| ReadSet
    ReadSet -->|Load into Host| WriteSet
```

### State Consistency Requirements

| State Element | Source | Purpose |
|---------------|--------|---------|
| Account Balance | Horizon API | Verify sender has funds |
| Contract State | Ledger Entries | Execute contract logic |
| Sequence Numbers | Account Query | Validate transaction ordering |
| Fee Pool | Ledger Query | Calculate fee impacts |

---

## Event Correlation & Error Tracing

### Diagnostic Events Flow

The Soroban Host emits structured diagnostic events during execution. Erst correlates these with transaction failures:

```mermaid
graph LR
    A["Transaction<br/>Execution"] -->|Event 1| B["Function Entry"]
    A -->|Event 2| C["Memory Access"]
    A -->|Event 3| D["Contract Call"]
    A -->|Event 4| E["Trap/Error"]
    A -->|Event 5| F["Host Rejection"]
    
    B --> G["Diagnostic<br/>Pipeline"]
    C --> G
    D --> G
    E --> G
    F --> G
    
    G -->|Filter| H["Relevant Events"]
    H -->|Trace| I["Error Cause"]
    I -->|Map to| J["Source Code"]
    J -->|Display| K["Developer"]
    
    style E fill:#FF6B6B
    style F fill:#FF6B6B
    style I fill:#FFE066
    style K fill:#51CF66
```

### Error Classification

```
┌─ Execution Errors
│  ├─ Trap/Panic
│  ├─ Assertion Failure
│  └─ Out of Bounds
│
├─ Logic Errors
│  ├─ Invalid State Transition
│  ├─ Permission Denied
│  └─ Contract Violation
│
├─ Resource Errors
│  ├─ Out of Gas
│  ├─ Memory Exceeded
│  └─ Instruction Limit
│
└─ Network Errors
   ├─ Invalid Signature
   ├─ Bad Sequence
   └─ Insufficient Balance
```

---

## Data Flow: Detailed Examples

### Example 1: Simple Contract Call

```mermaid
sequenceDiagram
    participant User
    participant Horizon as Horizon API
    participant CLI as Erst CLI
    participant Sim as Rust Simulator
    
    User->>CLI: erst debug abc123def
    
    Note over CLI: Fetch transaction
    CLI->>Horizon: GET /transactions/abc123def
    Horizon-->>CLI: TransactionEnvelope
    
    Note over CLI: Fetch ledger entries
    CLI->>Horizon: GET /ledger_entries?keys=[...]
    Horizon-->>CLI: LedgerEntries (accounts, contracts, storage)
    
    Note over CLI: Prepare simulation
    CLI->>Sim: {envelope_xdr, result_meta_xdr, ledger_entries}
    
    activate Sim
    Note over Sim: Decode XDR structures
    Note over Sim: Load state into Host VM
    Note over Sim: Execute contract WASM
    Note over Sim: Emit diagnostic events
    deactivate Sim
    
    Sim-->>CLI: {status, events, logs}
    
    CLI->>User: Display: ✓ Success<br/>Events: [...]<br/>Logs: [...]
```

### Example 2: Failed Transaction with Error

```mermaid
sequenceDiagram
    participant User
    participant CLI as Erst CLI
    participant Sim as Rust Simulator
    
    User->>CLI: erst debug failed_tx_hash
    
    CLI->>Sim: Execute simulation request
    
    activate Sim
    Note over Sim: Load state
    Note over Sim: Execute WASM bytecode
    Note over Sim: Hit: Assertion failed (user balance < amount)
    Note over Sim: Emit: "assertion_failure" event
    Note over Sim: Capture stack trace
    deactivate Sim
    
    Sim-->>CLI: {<br/>  status: "error",<br/>  error: "assertion at line 42",<br/>  events: [...],<br/>  logs: [...traceback...]<br/>}
    
    CLI->>User: Display error with:<br/>- Failure reason<br/>- Stack trace<br/>- Contract state at failure
```

---

## Integration Points & Extensibility

### Current Integration Points

```mermaid
graph TB
    Erst["Erst Core"]
    
    Erst -->|Horizon Client| HorizonAPI["Stellar Horizon API"]
    Erst -->|spf13/cobra| CLI["Command Framework"]
    Erst -->|zap| Logger["Logging System"]
    Erst -->|serde_json| JSONSerde["JSON Serialization"]
    Erst -->|soroban-env-host| Soroban["Soroban Host VM"]
    Erst -->|base64| Encoding["XDR Encoding"]
    
    style Erst fill:#4A90E2
    style HorizonAPI fill:#E74C3C
    style Soroban fill:#F39C12
```

### Future Integration Points

| Component | Purpose | Status |
|-----------|---------|--------|
| JSON-RPC Client | Direct ledger queries | Planned |
| WebAssembly Inspector | WASM-level debugging | Proposed |
| Source Map Integration | Map to Rust source | Phase 3 |
| Event Database | Persistent event logging | Future |
| Dashboard/Web UI | Visual debugging interface | Backlog |

---

## Performance Considerations

### Optimization Strategies

```mermaid
graph TD
    A["Optimization Goals"]
    
    A --> B["Minimize State Transfer"]
    B -->|Strategy| B1["Only fetch read/write set"]
    B -->|Strategy| B2["Use JSON-RPC pagination"]
    
    A --> C["Fast Local Execution"]
    C -->|Strategy| C1["Pre-compile WASM if needed"]
    C -->|Strategy| C2["Cache Host state"]
    
    A --> D["Efficient Data Format"]
    D -->|Strategy| D1["Base64 encoding (portable)"]
    D -->|Strategy| D2["Compressed XDR for large states"]
    
    style A fill:#FFE066
    style B fill:#51CF66
    style C fill:#51CF66
    style D fill:#51CF66
```

### Benchmarking Metrics

- **State Fetch Time**: Horizon latency + payload size
- **Simulation Time**: WASM execution + event collection
- **Memory Usage**: Host VM state + ledger cache
- **End-to-End**: User request → result display

---

## Development Setup

### Building the Project

```bash
# Clone and navigate
git clone https://github.com/dotandev/hintents.git
cd hintents

# Build Rust simulator
cd simulator
cargo build --release
cd ..

# Build Go CLI
go build -o erst ./cmd/erst

# Run tests
go test ./...
```

### Environment Variables

```bash
# Simulator binary location (optional)
export ERST_SIMULATOR_PATH=/path/to/erst-sim

# Network selection
export STELLAR_NETWORK=testnet  # or mainnet, futurenet
```

---

## Testing Architecture

### Unit Testing

- **RPC Client Tests**: Mock Horizon API responses
- **Runner Tests**: Test subprocess execution and IPC
- **Serialization Tests**: Validate JSON encoding/decoding

### Integration Testing

```mermaid
graph LR
    A["Integration Tests"]
    
    A -->|Live Network| B["Testnet Transactions"]
    B -->|Fetch| C["Real Horizon Data"]
    C -->|Simulate| D["Rust VM Execution"]
    D -->|Verify| E["Expected Outputs"]
    
    A -->|Test Fixtures| F["Mocked Data"]
    F -->|Simulate| D
    D -->|Verify| E
```

---

## Troubleshooting Guide

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| `erst-sim not found` | Binary not built or incorrect path | Run `cargo build --release` in simulator/ |
| `Invalid JSON` from simulator | Malformed request | Check XDR encoding is Base64-valid |
| `Ledger entry decode error` | Incorrect ledger state snapshot | Verify transaction block number |
| `Horizon API timeout` | Network latency or endpoint down | Use `--network futurenet` or check Horizon status |

---

## Future Enhancements

1. **Phase 2**: Source-level debugging with line mapping
2. **Phase 3**: Web dashboard for visual inspection
3. **Phase 4**: Machine learning for error pattern detection
4. **Phase 5**: Integration with IDEs and development tools

---

## Contributing

For architecture questions or to propose changes, see [CONTRIBUTING.md](../CONTRIBUTING.md).
# Architecture

Erst follows a split-architecture design to leverage the best tools for each job:
1.  **Go CLI (`cmd/erst`)**: Provides a user-friendly interface, handles networking (Horizon RPC), and manages the user experience.
2.  **Rust Simulator (`simulator/`)**: Uses `soroban-env-host` to replay transactions with high fidelity.

## Inter-Process Communication (IPC)

The core validation and debugging logic happens in the Rust simulator. Since `soroban-env-host` is a Rust library, we wrap it in a standalone binary (`erst-sim`) and communicate with it from the Go CLI via standard input/output (Stdin/Stdout).

### Data Flow

```mermaid
sequenceDiagram
    participant User
    participant GoCLI as Go CLI (erst)
    participant RustSim as Rust Simulator (erst-sim)

    User->>GoCLI: erst debug <tx_hash>
    Note over GoCLI: Fetch Tx Envelope & ResultMeta from RPC
    GoCLI->>GoCLI: Construct SimulationRequest (JSON)
    GoCLI->>RustSim: Spawn Subprocess
    GoCLI->>RustSim: Write JSON to Stdin
    Note over RustSim: Parse Request
    Note over RustSim: Initialize soroban-env-host
    Note over RustSim: Replay Transaction
    RustSim->>GoCLI: Write SimulationResponse (JSON) to Stdout
    Note over GoCLI: Parse Response
    GoCLI->>User: Display Debug Info / Errors
```

### JSON Protocol

#### Request (Go -> Rust)

The Go CLI sends a JSON object containing the XDR envelopes required to reconstruct the transaction state.

```json
{
  "envelope_xdr": "AAAA...",
  "result_meta_xdr": "AAAA...",
  "ledger_entries": {
    "key_xdr_1": "entry_xdr_1"
  }
}
```

#### Response (Rust -> Go)

The Rust simulator returns a JSON object with the execution status, logs, and any events captured during the replay.

```json
{
  "status": "success",
  "error": null,
  "events": ["Contract Event 1", "Diagnostic Event 2"],
  "logs": ["Host Initialized", "Charged 100 fee"]
}
```
