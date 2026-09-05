# IPC Protocol — Go ↔ Simulator

This document specifies the inter-process communication (IPC) protocol between
the **erst** CLI (Go) and the **erst-sim** simulator (Rust). It is intended for
third-party tool authors who want to drive the simulator directly or consume
its output without going through the CLI.

Two transport modes exist:

1. **One-shot simulation**: a single JSON request on stdin, a single JSON
   response on stdout.
2. **Streaming NDJSON**: newline-delimited JSON frames emitted on stdout while
   the simulation runs (time-travel / interactive modes), plus bidirectional
   control commands on stdin.

All payloads are UTF-8 JSON, one document per line (NDJSON) on the streaming
channel. Source references point to the authoritative implementations:
`internal/bridge/` (Go side) and `simulator/src/ipc/` (Rust side).

---

## 1. One-shot simulation (request/response)

The Go CLI spawns the simulator subprocess and writes one JSON object to its
stdin; the simulator writes one JSON object to stdout.

### 1.1 Request (Go → simulator)

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

### 1.2 Optional request extensions

`internal/simulator/request.go` (`SimulationRequest`) defines additional
optional fields the simulator accepts:

| Field | Type | Purpose |
|-------|------|---------|
| `control_command` | String | Bridge control command (see §3), e.g. `ROLLBACK_AND_RESUME` |
| `rewind_step` | Int | Snapshot sequence to rewind to before resuming |
| `fork_params` | Map (String → String) | Parameters for a forked simulation run |
| `harness_reset` | Bool | Reset the regression harness state before running |
| `timestamp` | Int64 | Ledger timestamp to simulate |
| `ledger_sequence` | Uint32 | Ledger sequence to simulate |
| `wasm_path` | String (path) | Local WASM file to execute instead of the on-chain code |
| `no_cache` | Bool | Bypass the local ledger state cache |
| `mock_args` | Array (String) | Arguments for mocked host functions |
| `profile` | Bool | Enable profiler output |
| `protocol_version` | Uint32 | Negotiated protocol version |
| `mock_base_fee` / `mock_gas_price` | Uint32 / Uint64 | Override fee parameters for mocking |
| `memory_limit` | Uint64 | Per-instance memory limit |
| `enable_coverage` / `coverage_lcov_path` | Bool / String | Emit an LCOV coverage report |
| `enable_optimization_advisor` | Bool | Emit an optimization report |
| `restore_preamble` | Map | State to restore before execution |
| `auth_trace_opts` | Object | Auth-trace options (`enabled`, `trace_custom_contracts`, `capture_sig_details`, `max_event_depth`) |
| `custom_auth_config` | Map | Custom auth configuration |
| `resource_calibration` | Object | Host-function cost calibration (`sha256_fixed`, `sha256_per_byte`, `keccak256_fixed`, `keccak256_per_byte`, `ed25519_fixed`) |
| `sandbox_native_token_cap_stroops` | Uint64 | Native-token cap for sandbox mode |
| `contract_wasm` | String (Base64) | Inline WASM instead of `wasm_path` |
| `enable_asset_safety` | Bool | Enable asset-safety checks |

### 1.3 Response (simulator → Go)

```json
{
  "status": "success|error",
  "error": "error message",
  "events": ["event1", "event2"],
  "logs": ["log1", "log2"]
}
```

**Field Descriptions:**

| Field | Type | Purpose |
|-------|------|---------|
| `status` | String | Execution status: `"success"` or `"error"` |
| `error` | String \| Null | Error message if status is `"error"` |
| `events` | Array | Diagnostic events emitted during execution |
| `logs` | Array | Detailed execution logs for debugging |

`SimulationResponse` (Go: `internal/simulator/response.go`) additionally
carries optional structured payloads:

| Field | Type | Purpose |
|-------|------|---------|
| `error_code` | String | Structured error code (see §5) |
| `events` / `diagnostic_events` | Array | Diagnostic events (see §4) |
| `logs` | Array | Execution logs |
| `flamegraph` | String | Flamegraph payload |
| `auth_trace` | Object | Auth trace (signers, thresholds, SAC calls, replay warnings) |
| `optimization_report` | Object | Efficiency tips and budget breakdown |
| `budget_usage` | Object | CPU/memory budget consumption |
| `categorized_events` | Array | Events grouped by category |
| `protocol_version` | Uint32 | Protocol version used |
| `stack_trace` | Object | WASM stack trace (`trap_kind`, `raw_message`, `frames`, `soroban_wrapped`) |
| `source_location` | Object | DWARF source mapping (`file`, `line`, `column`, `column_end`) |
| `wasm_offset` | Uint64 | Offset of the trapping instruction |
| `linear_memory_dump` | String (Base64) | Linear memory dump at the failure point |
| `asset_anomalies` | Array | Asset-safety anomalies |
| `lcov_report` / `lcov_report_path` | String | Coverage report (inline or path) |

### 1.4 Structured diagnostic events

Diagnostic events (Go: `internal/simulator/events.go`, `DiagnosticEvent`):

| Field | Type | Purpose |
|-------|------|---------|
| `event_type` | String | Event discriminator |
| `contract_id` | String \| Null | Emitting contract ID |
| `topics` | Array (Base64) | XDR-encoded `ScVal` topics |
| `data` | String (Base64) | XDR-encoded `ScVal` payload |
| `in_successful_contract_call` | Bool | Whether the event fired inside a successful call |
| `wasm_instruction` | String \| Null | WASM instruction (budget ticks) |
| `cpu` / `mem` | Uint64 \| Null | Budget counters at emission time |

`data` and each `topics` element are Base64-encoded XDR `ScVal`s.

---

## 2. Streaming NDJSON protocol (simulator → Go)

Interactive and time-travel modes use a streaming protocol: the simulator
emits one JSON object per line on stdout. Each line is a **StreamFrame**
(Go: `internal/bridge/reader.go`; Rust: `simulator/src/ipc/types.rs`):

```json
{"type":"snapshot","seq":0,"data":{...}}
{"type":"chunk","seq":1,"total":3,"data":"..."}
{"type":"final","seq":2,"data":{...}}
```

| Field | Type | Purpose |
|-------|------|---------|
| `type` | String | Frame discriminator: `snapshot`, `final`, `chunk`, `fetch_response` |
| `seq` | Uint32 | Monotonically increasing sequence number (0-based) within a run |
| `total` | Uint32 | Expected frame count in a logical batch (`chunk` frames only) |
| `data` | JSON value | Frame payload |

**Frame semantics:**

- `snapshot` — intermediate ledger-snapshot frame produced while the
  simulation is still running; forwarded to the UI as it arrives.
- `final` — terminal frame; `data` contains the complete
  `SimulationResponse` JSON object.
- `chunk` — partial payload of a large response. Each chunk's `data` field is
  a **JSON-escaped string** fragment of the full payload; the consumer
  collects all `chunk` frames, sorts them by `seq`, JSON-parses each `data`
  string, and concatenates the fragments in `seq` order. `total` declares how
  many chunks to expect.
- `fetch_response` — answer to a `FETCH_SNAPSHOT` control command (§3).

**Compatibility:** a line with no recognised `type` field but a top-level
`status` key is treated as a legacy non-streaming final response and returned
as-is. Unknown frames are skipped for forward compatibility.

**Buffering:** the Go reader (`internal/bridge/reader.go`) allows individual
lines up to 16 MiB (large responses exceed the 64 KiB `bufio` default).

**Key advantages** (per `docs/ARCHITECTURE.md` §4.2):
- **Asymmetric processing**: the CLI can render the trace while the simulator
  is still executing WASM or fetching ledger state.
- **Low latency**: NDJSON is line-buffered, bypassing monolithic JSON parsing.
- **Reliability**: `seq` numbers let consumers detect dropped or out-of-order
  frames across the pipe.

---

## 3. Control commands (Go → simulator, stdin)

Control commands are single NDJSON lines written to the simulator's stdin.

### 3.1 `FETCH_SNAPSHOT`

Requests snapshot frames by sequence ID.

```json
{"op":"FETCH_SNAPSHOT","id":3,"batch_size":5}
```

| Field | Type | Purpose |
|-------|------|---------|
| `op` | String | Always `"FETCH_SNAPSHOT"` |
| `id` | Uint32 | Starting snapshot sequence ID |
| `batch_size` | Uint32 | Number of consecutive frames to return (default 1, capped at 5) |

The simulator responds with a `fetch_response` frame on stdout:

```json
{"type":"fetch_response","seq":3,"data":{"snapshots":[{"seq":3,"data":{...}}]}}
```

Go helpers: `FetchSnapshot(w, id)` and `FetchSnapshotBatch(w, id, count)`
(`internal/bridge/client.go`). The caller is responsible for reading the
corresponding `fetch_response` frame.

### 3.2 `ROLLBACK_AND_RESUME`

Injected into a `SimulationRequest` payload (not a separate frame) via
`bridge.WithRollbackAndResume(reqJSON, rewindStep, forkParams, harnessReset)`:

```json
{
  "control_command": "ROLLBACK_AND_RESUME",
  "rewind_step": 2,
  "fork_params": {"key": "value"},
  "harness_reset": false
}
```

| Field | Type | Purpose |
|-------|------|---------|
| `control_command` | String | `"ROLLBACK_AND_RESUME"` |
| `rewind_step` | Int ≥ 0 | Snapshot sequence to rewind to |
| `fork_params` | Map (String → String) | Optional parameters for the forked run (removed if empty) |
| `harness_reset` | Bool | Whether to reset the regression harness state |

---

## 4. Payload compression (Go → simulator)

`internal/bridge/compress.go` (`CompressRequest`) optionally replaces the
plain `ledger_entries` map with a Zstd-compressed, Base64-encoded blob:

```json
{
  "ledger_entries_zstd": "base64-of-zstd-compressed-json-map"
}
```

- If `ledger_entries` is absent or empty, the request is returned unchanged.
- The original `ledger_entries` key is removed when compression is applied.
- The Rust simulator detects `ledger_entries_zstd` and decompresses it
  automatically (see `simulator/src/ipc/decompress.rs`); this reduces IPC
  payload size by 60–80% in practice.

---

## 5. Error codes

The Rust simulator may emit plain message strings without structured codes;
the Go side (`internal/ipc/types.go`) first maps structured code strings via
`mapIPCCode`, then falls back to message-based heuristics via
`classifyByMessage`:

| Structured code | Classified as |
|-----------------|---------------|
| `SIMULATION_FAILED`, `EXECUTION_FAILED` | simulation execution failure |
| `WASM_TRAP`, `CONTRACT_TRAP` | simulator crash |
| `INVALID_INPUT`, `VALIDATION_ERROR` | validation failure |
| `PROTOCOL_UNSUPPORTED` | protocol unsupported |
| `ERR_MEMORY_LIMIT_EXCEEDED`, `MEMORY_LIMIT_EXCEEDED` | memory limit exceeded |

Message heuristics (fallback when no structured code is emitted):

| Message contains | Classified as |
|------------------|---------------|
| `decode Envelope` / `decode LedgerKey` / `decode LedgerEntry` / `decode WASM` | unmarshal failure |
| `Wasm Trap` / `unreachable` / `stack overflow` / `out of bounds` | simulator crash |
| `memory limit exceeded` | memory limit exceeded |
| `InvalidInput` | validation failure |
| anything else | simulation execution failure |

Transport-level failures (Go side): a simulator that exits without emitting a
final frame yields `simulator stdout closed without a final frame`; a
simulator that cannot bind its port maps to `IPC bridge could not bind`
(`simulator/src/ipc/types.rs`, `IpcError::PortBindingFailed`).

---

## 6. Transport notes for implementers

- **Encoding**: UTF-8 JSON; one document per line on the streaming channel.
- **Line buffering**: allow individual NDJSON lines up to 16 MiB
  (`internal/bridge/reader.go`); large simulation responses exceed the 64 KiB
  `bufio` default.
- **Cancellation**: readers check context cancellation before each blocking
  read; a cancelled context terminates frame collection.
- **Ordering**: `seq` numbers are monotonic within a run, but out-of-order
  delivery is possible; sort by `seq` when order matters.
- **Process model**: the CLI spawns the simulator as a subprocess and
  communicates over stdin/stdout (one-shot mode) or stdin commands + stdout
  NDJSON frames (streaming mode).

---

## 7. References

- `internal/bridge/client.go` — commands and request compression
- `internal/bridge/reader.go` — NDJSON frame parsing
- `internal/simulator/request.go` / `response.go` — request/response schemas
- `internal/ipc/types.go` — error classification
- `simulator/src/ipc/types.rs` — Rust-side frames and command handling
- `docs/ARCHITECTURE.md` — overall data flow and design decisions
