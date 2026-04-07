# Time Travel Guide

The Magic Rewind feature lets you re-simulate any Soroban transaction at an arbitrary
point in time, replay it across a sliding window, and save the entire session to disk
so that teammates can reproduce your findings without a network connection.

> **Full documentation**: [https://dotandev-hintents-75.mintlify.app/](https://dotandev-hintents-75.mintlify.app/)

## Contents

- [How It Works](#how-it-works)
- [Flag Reference](#flag-reference)
- [Keyboard Shortcuts](#keyboard-shortcuts)
- [Tutorial: Finding the Bug That Caused a Storage Leak](#tutorial-finding-the-bug-that-caused-a-storage-leak)
- [Snapshot Persistence](#snapshot-persistence)
- [Combining with Snapshot Diff](#combining-with-snapshot-diff)
- [Troubleshooting](#troubleshooting)

---

## How It Works

When a Soroban transaction runs on-chain, the ledger header carries a timestamp
that contracts can read via `env.ledger().timestamp()`. Time-dependent logic —
lock expiries, vesting schedules, rate-limiting windows — behaves differently
depending on that value.

Magic Rewind intercepts the timestamp before it reaches the WASM execution
environment and replaces it with a value you control. The rest of the simulation
pipeline (ledger state, envelope, resource budget) is unchanged, so only the
time-sensitive paths are affected.

```
                    ┌─────────────────────────────────┐
  Transaction  ───► │  erst debug --timestamp <epoch>  │
  (mainnet XDR)     │                                  │
                    │  Ledger state:  unchanged         │
                    │  Envelope:      unchanged         │
                    │  Timestamp:     YOUR VALUE        │
                    └──────────────┬──────────────────┘
                                   │
                                   ▼
                           soroban-env-host
                           (local simulation)
```

When you add `--window`, erst runs the simulation five times, stepping
the timestamp evenly across the specified range. This lets you observe how
contract behavior changes as time advances without running five separate
commands.

---

## Flag Reference

All flags below are passed to `erst debug`.

### `--timestamp <epoch>`

Override the ledger timestamp for a single simulation run.

`<epoch>` is a Unix timestamp in seconds (e.g. `1735689600`).

```bash
erst debug <tx-hash> --timestamp 1735689600
```

Use `date -d "2026-01-01 00:00:00 UTC" +%s` (Linux) or
`date -jf "%Y-%m-%d %H:%M:%S" "2026-01-01 00:00:00" +%s` (macOS) to
convert a human-readable date to an epoch value.

### `--window <seconds>`

Run five simulations distributed evenly across a time range starting at
`--timestamp` and ending at `--timestamp + window`.

```bash
# Simulate the transaction at T, T+15m, T+30m, T+45m, and T+60m
erst debug <tx-hash> --timestamp 1735689600 --window 3600
```

Both `--timestamp` and `--window` are persistent root flags; they work with any
subcommand that accepts a transaction hash.

### `--mock-time <epoch>`

A debug-specific alternative to `--timestamp`. Functionally identical for
single-run simulations; prefer `--timestamp` for consistency.

```bash
erst debug <tx-hash> --mock-time 1735689600
```

### `--save-snapshots <path>`

After a successful simulation run, save the ledger entries and transaction data
to a compressed snapshot registry file (conventionally `.erstsnap`). The file
can be replayed offline with `--load-snapshots`.

```bash
erst debug <tx-hash> --timestamp 1735689600 --save-snapshots ./session.erstsnap
```

When used with `--window`, one entry is written per simulated timestamp so the
full time-travel session is preserved.

### `--load-snapshots <path>`

Replay a previously saved snapshot registry without any network connectivity.
No transaction hash argument is required; the hash is read from the file.

```bash
erst debug --load-snapshots ./session.erstsnap
```

The simulation runs against the exact ledger state that was captured when the
file was saved, making results fully reproducible.

### `--snapshot <path>`

Load a single soroban-cli-compatible snapshot file as the ledger state source.
Unlike `--load-snapshots`, this does not replace the transaction envelope or
network metadata — it only substitutes the ledger entries.

```bash
erst debug <tx-hash> --snapshot ./before.json
```

---

## Keyboard Shortcuts

The interactive trace viewer is launched with the `--interactive` (or `-i`) flag.
All shortcuts are available once the viewer is open.

### Navigation

| Key                  | Action                          |
| -------------------- | ------------------------------- |
| `↑` / `k`            | Move up one row                 |
| `↓` / `j`            | Move down one row               |
| `PgUp`               | Scroll up one page              |
| `PgDn`               | Scroll down one page            |
| `Home` / `g`         | Jump to the first row           |
| `End` / `G`          | Jump to the last row            |
| `Enter` / `Space`    | Toggle expand / collapse a node |

### Search

| Key     | Action                               |
| ------- | ------------------------------------ |
| `/`     | Open the search prompt               |
| `Enter` | Execute the search query             |
| `n`     | Jump to the next match               |
| `N`     | Jump to the previous match           |
| `ESC`   | Clear the current search and dismiss |

Search is case-insensitive and matches against contract IDs, function names,
error messages, event topics, and log lines simultaneously.

### Tree Operations

| Key | Action                  |
| --- | ----------------------- |
| `e` | Expand all nodes        |
| `c` | Collapse all nodes      |
| `S` | Toggle Rust core traces |

### Miscellaneous

| Key            | Action                       |
| -------------- | ---------------------------- |
| `?` / `h`      | Show the keyboard help panel |
| `q` / `Ctrl+C` | Quit the viewer              |

---

## Tutorial: Finding the Bug That Caused a Storage Leak

This walkthrough demonstrates Magic Rewind on a realistic scenario: a token
contract that accumulates unbounded persistent storage entries over time,
eventually causing transactions to fail due to resource exhaustion.

### Background

Soroban contracts pay rent on persistent storage entries. A contract that creates
a new entry per invocation (for example, recording each transfer in its own key)
will grow the ledger footprint indefinitely. Symptoms appear gradually:

- Early transactions succeed.
- Later transactions fail with `WasmVm` or resource exhaustion errors.
- The failure cannot be reproduced by inspecting just the failing transaction,
  because the root cause was written by earlier transactions.

Time-travel lets you step back through history and observe the storage growth
that led to the failure.

### Prerequisites

- `erst` installed and on your `PATH`
- A Stellar RPC token set in `ERST_RPC_TOKEN` or passed via `--rpc-token`
- The hash of a transaction that is failing with a resource-related error

### Step 1: Confirm the failure

Start by reproducing the failure at the current time so you have a baseline.

```bash
erst debug <failing-tx-hash> --network mainnet --verbose
```

Expected output (abbreviated):

```
Debugging transaction: abc123...
Network: mainnet

--- Result for mainnet ---
Status: error
Error: WasmVm

Resource Usage:
  CPU Instructions: 99847 / 100000 (99.85%)  [!]  CRITICAL
  Memory Bytes: 8388480 / 8388608 (99.99%)   [!]  CRITICAL
  Operations: 42

Diagnostic Events: 3
  [1] Type: contract, Contract: CAAAA...
      Topics: ["storage_write", "entry_count"]
      Data: 4097
```

The `entry_count` of 4097 confirms that the contract has written far more
entries than intended. The resource budget is nearly exhausted.

### Step 2: Locate the origin timestamp

Identify a transaction hash from an earlier ledger — one where the contract
was still operating normally. The Stellar Explorer or the `stellar-horizon`
API can give you the timestamp of any historical ledger.

For this example, assume the contract was deployed at epoch `1735689600`
(2026-01-01 00:00:00 UTC) and started failing around `1736121600`
(2026-01-06 00:00:00 UTC).

### Step 3: Simulate at the deployment timestamp

```bash
erst debug <failing-tx-hash> \
  --network mainnet \
  --timestamp 1735689600
```

Output:

```
--- Simulating at Timestamp: 1735689600 ---
Running simulation on mainnet...

--- Result for mainnet ---
Status: success

Resource Usage:
  CPU Instructions: 3420 / 100000 (3.42%)
  Memory Bytes: 65536 / 8388608 (0.78%)
  Operations: 1

Diagnostic Events: 1
  [1] Type: contract
      Topics: ["storage_write", "entry_count"]
      Data: 1
```

The same transaction succeeds at T=0 with only one storage entry. This
confirms the storage grew between deployment and the failure date.

### Step 4: Observe growth across the window

Use `--window` to step through five points in the five-day failure period
and watch storage grow:

```bash
erst debug <failing-tx-hash> \
  --network mainnet \
  --timestamp 1735689600 \
  --window 432000          # 5 days in seconds
```

Output (abbreviated):

```
--- Simulating at Timestamp: 1735689600 ---  # Day 0
  entry_count: 1   Memory: 0.78%

--- Simulating at Timestamp: 1735797600 ---  # Day 1.25
  entry_count: 864   Memory: 20.51%

--- Simulating at Timestamp: 1735905600 ---  # Day 2.5
  entry_count: 1792   Memory: 42.61%

--- Simulating at Timestamp: 1736013600 ---  # Day 3.75
  entry_count: 2731   Memory: 64.97%

--- Simulating at Timestamp: 1736121600 ---  # Day 5
  entry_count: 3701   Memory: 88.07%
```

The linear growth confirms that every invocation adds exactly one new entry
and never removes any. The storage leak is in the write path.

### Step 5: Narrow the failing function

Open the interactive viewer at the mid-point timestamp to inspect the call
tree:

```bash
erst debug <failing-tx-hash> \
  --network mainnet \
  --timestamp 1735797600 \
  --interactive
```

Inside the viewer:

1. Press `/` and type `storage_write`, then `Enter` to find all write events.
2. Press `n` to step through each match.
3. Press `Enter` on the offending node to expand it and reveal the key being
   written.
4. Press `e` to expand the full call tree and trace the write back to its
   caller.

```
> [contract_call] transfer(from, to, amount)
    [host_function] storage_write
      key: "transfer_log:1735797600:GBX...4W37"   <-- unique key per call
      value: { from, to, amount, ts }
```

The key includes the timestamp, making each write unique. The contract is
logging every transfer to persistent storage but never expiring or deleting
old entries.

### Step 6: Save the session for offline sharing

Capture the full time-window session so that a colleague can reproduce it
without network access:

```bash
erst debug <failing-tx-hash> \
  --network mainnet \
  --timestamp 1735689600 \
  --window 432000 \
  --save-snapshots ./storage-leak-session.erstsnap
```

Output:

```
Snapshot registry saved: ./storage-leak-session.erstsnap (5 entries)
```

Share the `.erstsnap` file. Your colleague can replay it with:

```bash
erst debug --load-snapshots ./storage-leak-session.erstsnap
```

No RPC token, no network, and no transaction hash are needed. The replay
produces identical output to the original run.

### Step 7: Fix and verify

The fix is to bound the transfer log using a rotating key based on a
truncated timestamp (e.g. hourly bucket) or to move the log to a temporary
storage tier with TTL. After deploying the fix, re-run the original command
without `--timestamp` to confirm the storage entries no longer grow.

---

## Snapshot Persistence

### File Format

Snapshot registry files (`.erstsnap`) are gzip-compressed JSON. You can
inspect the raw contents with:

```bash
zcat session.erstsnap | python3 -m json.tool | head -40
```

The top-level structure is:

```json
{
  "version": "...",
  "created_at": "2026-01-06T00:00:00Z",
  "tx_hash": "abc123...",
  "network": "mainnet",
  "envelope_xdr": "<base64>",
  "result_meta_xdr": "<base64>",
  "entries": [
    {
      "timestamp": 1735689600,
      "snapshot": {
        "ledgerEntries": [["<key xdr>", "<value xdr>"], ...],
        "linearMemory": "<base64>"
      }
    }
  ]
}
```

### Sharing Sessions

Because `.erstsnap` files are self-contained, they are safe to attach to
GitHub issues, Slack threads, or incident post-mortems. The file contains
only the ledger entries relevant to the simulated transaction — not the
entire ledger.

Compressed sizes for typical sessions:

| Entries | Approximate file size |
| ------- | --------------------- |
| 1       | 5–50 KB               |
| 5       | 15–150 KB             |
| 20      | 60–600 KB             |

### Compatibility

Snapshot registry files are forward-compatible: future versions of `erst`
can load files created by older versions. If a field is unrecognized it is
ignored.

---

## Combining with Snapshot Diff

`erst snapshot-diff` compares the linear memory of two soroban-cli-compatible
snapshot files, making it easy to see exactly which memory regions changed
between two points in time.

Export individual snapshots from a registry entry:

```bash
# Simulate and save single-entry snapshots at two points in time
erst debug <tx-hash> --timestamp 1735689600 --save-snapshots t0.erstsnap
erst debug <tx-hash> --timestamp 1736121600 --save-snapshots t5.erstsnap
```

Then diff the memory:

```bash
erst snapshot-diff --snapshot-a before.json --snapshot-b after.json
```

With focused region inspection:

```bash
# Show 256 bytes starting at offset 0x200 with 32 bytes of context
erst snapshot-diff \
  --snapshot-a before.json \
  --snapshot-b after.json \
  --offset 0x200 \
  --length 256 \
  --context 32
```

See [docs/FOOTPRINT_EXTRACTION.md](FOOTPRINT_EXTRACTION.md) for details on
extracting footprint data and [docs/LEDGER_ENTRY_VERIFICATION.md](LEDGER_ENTRY_VERIFICATION.md)
for verifying ledger entry integrity.

---

## Troubleshooting

### "snapshot registry contains no entries"

The `.erstsnap` file was created from a comparison run (`--compare-network`).
Snapshot collection currently only supports single-network runs. Re-run
without `--compare-network` to generate a usable file.

### Simulation result differs from original on-chain execution

This is expected when using `--timestamp` with a value different from the
on-chain ledger timestamp. The goal of Magic Rewind is to observe how
behavior changes with time, not to reproduce the original result exactly.
To reproduce the original, omit `--timestamp`.

### "failed to load snapshot registry: create gzip reader"

The file is not a valid `.erstsnap` archive. Verify that the file was created
by `erst debug --save-snapshots` and has not been truncated. If you
are inspecting a file shared by a colleague, confirm that the transfer was
binary-safe (not sent as text through a system that converts line endings).

### Timestamps and time zones

All timestamps in `erst` are Unix epochs (UTC). The `--timestamp` flag
does not accept date strings. Use a conversion tool:

```bash
# Linux
date -d "2026-01-01 00:00:00 UTC" +%s

# macOS
date -jf "%Y-%m-%d %H:%M:%S" "2026-01-01 00:00:00" +%s

# Python (any platform)
python3 -c "import calendar, datetime; print(calendar.timegm(datetime.datetime(2026,1,1).timetuple()))"
```

### The `--window` flag has no effect

`--window` requires `--timestamp` to be set to a non-zero value. Both flags
must be provided together:

```bash
# Correct
erst debug <tx-hash> --timestamp 1735689600 --window 3600

# No effect (window is ignored when timestamp is 0)
erst debug <tx-hash> --window 3600
```
