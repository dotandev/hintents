# Integrating erst-sim into GitHub Actions

This tutorial shows how to run `erst-sim` automatically on every pull request so
that contract upgrade regressions are caught before code reaches your main branch.

By the end you will have a workflow that:

- Builds your Soroban contracts from source
- Runs `erst-sim` against each contract's transaction envelope
- Fails the PR if the simulation returns an error status or exceeds your budget thresholds
- Posts a summary of CPU instructions and memory usage as a PR comment

---

## Prerequisites

- A repository containing one or more Soroban smart contracts
- `erst-sim` available as a pre-built binary or buildable from source in your repo
- A way to produce a signed (or unsigned) transaction envelope XDR for each contract
  you want to validate — typically a small script that calls the Stellar SDK

---

## Repository layout assumed by this tutorial

```
.
├── contracts/
│   └── my_contract/
│       ├── Cargo.toml
│       └── src/lib.rs
├── scripts/
│   └── build_envelope.sh   # produces envelope.b64 and ledger_entries.json
├── erst-sim/               # erst-sim source, or install it in CI
└── .github/
    └── workflows/
        └── erst-sim.yml
```

---

## Step 1 — Build erst-sim in CI

If you distribute `erst-sim` as a pre-built binary, download it in the workflow.
If you build it from source (the approach shown here), cache the Rust build
artifacts to keep subsequent runs fast.

```yaml
# .github/workflows/erst-sim.yml
name: erst-sim contract validation

on:
  pull_request:
    paths:
      - 'contracts/**'
      - 'erst-sim/**'

jobs:
  simulate:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@stable
        with:
          targets: wasm32-unknown-unknown

      - name: Cache Rust build artefacts
        uses: Swatinem/rust-cache@v2
        with:
          workspaces: |
            erst-sim
            contracts/my_contract

      - name: Build erst-sim
        run: cargo build --release --manifest-path erst-sim/Cargo.toml

      - name: Build contract WASM
        run: |
          cargo build \
            --release \
            --target wasm32-unknown-unknown \
            --manifest-path contracts/my_contract/Cargo.toml
```

---

## Step 2 — Produce a transaction envelope

`erst-sim` reads a JSON payload from stdin that includes a base64-encoded
transaction envelope and, optionally, the ledger entries needed for the
simulation.  Your `scripts/build_envelope.sh` should write this payload to a
file so the workflow can pipe it in.

A minimal payload looks like this:

```json
{
  "envelope_xdr": "<base64-encoded TransactionEnvelope XDR>",
  "result_meta_xdr": "",
  "ledger_entries": {},
  "enable_optimization_advisor": true,
  "profile": false
}
```

Add the script invocation to the workflow:

```yaml
      - name: Install Node.js (for envelope builder)
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install envelope builder dependencies
        run: npm ci --prefix scripts

      - name: Build simulation payload
        run: bash scripts/build_envelope.sh > /tmp/sim_payload.json
        env:
          CONTRACT_WASM: contracts/my_contract/target/wasm32-unknown-unknown/release/my_contract.wasm
          NETWORK_PASSPHRASE: ${{ vars.NETWORK_PASSPHRASE }}
```

---

## Step 3 — Run erst-sim and validate the result

Pipe the payload into `erst-sim`, capture the JSON response, and fail the job
if `status` is not `"success"`.

```yaml
      - name: Run erst-sim
        id: sim
        run: |
          ./erst-sim/target/release/erst-sim < /tmp/sim_payload.json \
            | tee /tmp/sim_result.json

      - name: Assert simulation succeeded
        run: |
          status=$(jq -r '.status' /tmp/sim_result.json)
          if [ "$status" != "success" ]; then
            echo "erst-sim returned status: $status"
            jq -r '.error // "no error field"' /tmp/sim_result.json
            exit 1
          fi
```

---

## Step 4 — Enforce budget thresholds

Letting any simulation pass regardless of resource usage defeats the purpose of
the check.  Add threshold assertions after the status check.

The example below limits CPU instructions to 80 % of the Soroban network limit
(100 000 000 instructions) and memory to 50 MB.  Adjust the thresholds to suit
your contract.

```yaml
      - name: Enforce budget thresholds
        run: |
          cpu=$(jq '.budget_usage.cpu_instructions' /tmp/sim_result.json)
          mem=$(jq '.budget_usage.memory_bytes'     /tmp/sim_result.json)

          CPU_THRESHOLD=80000000   # 80 % of the 100M instruction limit
          MEM_THRESHOLD=52428800   # 50 MiB

          echo "CPU instructions used : $cpu (threshold: $CPU_THRESHOLD)"
          echo "Memory bytes used     : $mem (threshold: $MEM_THRESHOLD)"

          if [ "$cpu" -gt "$CPU_THRESHOLD" ]; then
            echo "CPU budget threshold exceeded"
            exit 1
          fi

          if [ "$mem" -gt "$MEM_THRESHOLD" ]; then
            echo "Memory budget threshold exceeded"
            exit 1
          fi
```

---

## Step 5 — Post a PR comment with the simulation summary

Use the GitHub API to post the budget summary as a comment so reviewers can see
resource usage without leaving the PR page.

```yaml
      - name: Post simulation summary as PR comment
        if: always() && github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const fs   = require('fs');
            const data = JSON.parse(fs.readFileSync('/tmp/sim_result.json', 'utf8'));

            const status = data.status;
            const icon   = status === 'success' ? 'white_check_mark' : 'x';
            const budget = data.budget_usage ?? {};

            const cpuPct = budget.cpu_usage_percent != null
              ? budget.cpu_usage_percent.toFixed(2) + '%'
              : 'n/a';
            const memPct = budget.memory_usage_percent != null
              ? budget.memory_usage_percent.toFixed(2) + '%'
              : 'n/a';

            const errorLine = data.error
              ? `\n**Error:** \`${data.error}\``
              : '';

            const body = `## erst-sim result :${icon}:

            | Metric | Value |
            |---|---|
            | Status | \`${status}\` |
            | CPU instructions | ${budget.cpu_instructions ?? 'n/a'} (${cpuPct} of limit) |
            | Memory bytes | ${budget.memory_bytes ?? 'n/a'} (${memPct} of limit) |
            | Operations | ${budget.operations_count ?? 'n/a'} |
            ${errorLine}

            <details><summary>Full simulation log</summary>

            \`\`\`json
            ${JSON.stringify(data.logs ?? [], null, 2)}
            \`\`\`

            </details>`;

            github.rest.issues.createComment({
              owner:   context.repo.owner,
              repo:    context.repo.repo,
              issue_number: context.issue.number,
              body,
            });
```

---

## Complete workflow file

```yaml
# .github/workflows/erst-sim.yml
name: erst-sim contract validation

on:
  pull_request:
    paths:
      - 'contracts/**'
      - 'erst-sim/**'

jobs:
  simulate:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@stable
        with:
          targets: wasm32-unknown-unknown

      - name: Cache Rust build artefacts
        uses: Swatinem/rust-cache@v2
        with:
          workspaces: |
            erst-sim
            contracts/my_contract

      - name: Build erst-sim
        run: cargo build --release --manifest-path erst-sim/Cargo.toml

      - name: Build contract WASM
        run: |
          cargo build \
            --release \
            --target wasm32-unknown-unknown \
            --manifest-path contracts/my_contract/Cargo.toml

      - name: Install Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install envelope builder dependencies
        run: npm ci --prefix scripts

      - name: Build simulation payload
        run: bash scripts/build_envelope.sh > /tmp/sim_payload.json
        env:
          CONTRACT_WASM: contracts/my_contract/target/wasm32-unknown-unknown/release/my_contract.wasm
          NETWORK_PASSPHRASE: ${{ vars.NETWORK_PASSPHRASE }}

      - name: Run erst-sim
        run: |
          ./erst-sim/target/release/erst-sim < /tmp/sim_payload.json \
            | tee /tmp/sim_result.json

      - name: Assert simulation succeeded
        run: |
          status=$(jq -r '.status' /tmp/sim_result.json)
          if [ "$status" != "success" ]; then
            echo "erst-sim returned status: $status"
            jq -r '.error // "no error field"' /tmp/sim_result.json
            exit 1
          fi

      - name: Enforce budget thresholds
        run: |
          cpu=$(jq '.budget_usage.cpu_instructions' /tmp/sim_result.json)
          mem=$(jq '.budget_usage.memory_bytes'     /tmp/sim_result.json)

          CPU_THRESHOLD=80000000
          MEM_THRESHOLD=52428800

          echo "CPU instructions used : $cpu (threshold: $CPU_THRESHOLD)"
          echo "Memory bytes used     : $mem (threshold: $MEM_THRESHOLD)"

          if [ "$cpu" -gt "$CPU_THRESHOLD" ]; then
            echo "CPU budget threshold exceeded"
            exit 1
          fi

          if [ "$mem" -gt "$MEM_THRESHOLD" ]; then
            echo "Memory budget threshold exceeded"
            exit 1
          fi

      - name: Post simulation summary as PR comment
        if: always() && github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const fs   = require('fs');
            const data = JSON.parse(fs.readFileSync('/tmp/sim_result.json', 'utf8'));

            const status = data.status;
            const icon   = status === 'success' ? 'white_check_mark' : 'x';
            const budget = data.budget_usage ?? {};

            const cpuPct = budget.cpu_usage_percent != null
              ? budget.cpu_usage_percent.toFixed(2) + '%'
              : 'n/a';
            const memPct = budget.memory_usage_percent != null
              ? budget.memory_usage_percent.toFixed(2) + '%'
              : 'n/a';

            const errorLine = data.error
              ? `\n**Error:** \`${data.error}\``
              : '';

            const body = `## erst-sim result :${icon}:

            | Metric | Value |
            |---|---|
            | Status | \`${status}\` |
            | CPU instructions | ${budget.cpu_instructions ?? 'n/a'} (${cpuPct} of limit) |
            | Memory bytes | ${budget.memory_bytes ?? 'n/a'} (${memPct} of limit) |
            | Operations | ${budget.operations_count ?? 'n/a'} |
            ${errorLine}

            <details><summary>Full simulation log</summary>

            \`\`\`json
            ${JSON.stringify(data.logs ?? [], null, 2)}
            \`\`\`

            </details>`;

            github.rest.issues.createComment({
              owner:   context.repo.owner,
              repo:    context.repo.repo,
              issue_number: context.issue.number,
              body,
            });
```

---

## Validating multiple contracts

If your repository contains more than one contract, use a matrix strategy so
each contract is simulated independently and a failure in one does not mask
failures in others.

```yaml
jobs:
  simulate:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        contract:
          - name: my_contract
            path: contracts/my_contract
          - name: token_contract
            path: contracts/token_contract

    steps:
      # ... build steps as above, substituting ${{ matrix.contract.path }}
```

---

## Troubleshooting

**erst-sim exits with `status: "error"` and `"Invalid JSON"`**
The payload written by `build_envelope.sh` is not valid JSON.  Run the script
locally and pipe the output through `jq .` to identify the issue.

**`Failed to parse Envelope XDR`**
The base64 string in `envelope_xdr` was generated against a different network or
protocol version than the one compiled into erst-sim.  Verify that
`NETWORK_PASSPHRASE` matches your target network.

**`result_meta_xdr` warnings in the simulation log**
Passing an empty string for `result_meta_xdr` is valid; erst-sim will run the
simulation with empty host storage.  If your contract reads ledger state, you
must populate `ledger_entries` with the relevant key/entry XDR pairs or the
simulation will produce a storage-miss error.

**Threshold values**
The CPU and memory thresholds in Step 4 are examples.  The canonical network
limits are exposed by erst-sim in the `budget_usage.cpu_limit` and
`budget_usage.memory_limit` fields of every successful response.  Use those
values as the ceiling and set your thresholds as a percentage of them rather
than hard-coding absolute numbers.