# Outstanding Work

Everything known to be unfinished, with what closes it. Written 2026-11-02.

Items are ordered by what blocks the most downstream work, not by effort. Each
one says what it costs to leave alone, because a tracker that only lists tasks
gets read once.

---

## 1. Blocking the product

### 1.1 `soroban-env-host` is pinned below the protocol the network runs

`simulator/Cargo.toml` declares `soroban-env-host = { version = ">=21.0, <26" }`,
which resolves to 25.2.1 — protocol 25. Contracts deployed on testnet today
target protocol 26 and 27, and the host refuses them:

```
Error(WasmVm, InvalidInput): "contract protocol number is newer than host", 27
```

The replay pipeline reaches real contract execution and stops here. Everything
downstream — traces, flamegraphs, token flow, the optimization advisor — is
unverifiable against current contracts until this moves.

The same pin blocks two other things: `stellar-xdr` cannot reach 25.0.1 because
`soroban-env-common 25.2.1` requires it at exactly `=25.0.0`, and the
`soroban-env-host` advisory (muxed address conversions, low) has no fix below
26.0.0.

`soroban-env-host` is published up to 30.x, so this is five major versions of
host API change. It is the single largest piece of work in this file and
deserves its own branch.

**Closes when:** the simulator builds and replays against a host matching the
protocol the network is on, with the golden suite (1.3) passing.

### 1.2 No golden replay suite

`internal/simulator/regression_harness.go` promises that "erst-sim produces
identical results to the original execution" and has no fixtures. There is no
`testdata/`, and open issue #1899 records that the harness does not extract
real ledger keys either.

Nine distinct defects in the replay path were fixed in the pass that produced
this file. Every one of them was invisible to the test suite, because nothing
replays a real transaction and compares the result to what the network
recorded. Without that, these regress silently.

**Closes when:** a set of real transactions (success and failure, mainnet and
testnet) is pinned with checked-in envelope/meta/ledger-entry fixtures, and CI
asserts local replay matches the on-chain outcome for every one.

### 1.3 Replay fidelity gaps that are known but unaddressed

Each of these makes a replay differ from what the network did. None blocks
execution; all affect whether the answer is trustworthy.

- **Ledger entries fetched from RPC are current, not historical.** When the
  transaction metadata does not cover a footprint key,
  `Client.ResolveReplayState` falls back to `getLedgerEntries`, which returns
  the entry as it stands now. Exact for contract code, an approximation for
  contract data that has changed since. There is no archival-state source
  wired up.
- **`network_id` is zeroed.** `SimHost::set_ledger_context` leaves
  `network_id: [0u8; 32]`. Anything deriving a contract ID or verifying an auth
  signature against the network passphrase will diverge.
- **Protocol version defaults to the host's own.** The replay should use the
  protocol the ledger was on. `rpc.GetLedgerHeader` already returns it and is
  not wired in.
- **Budget limits are not the network's.** `applyPagedMemoryLimit` sets a 64 MiB
  ceiling, which replaces the host's default budget rather than matching the
  protocol's real limits.

**Closes when:** each is either fixed or documented in `docs/` as an explicit
limitation of local replay.

### 1.4 Failure output is unreadable

A failed simulation prints the same escaped JSON blob three times, preceded by a
raw `slog` ERROR line. The project already knows how to render diagnostic
events and the `fn_call` chain — the trace viewer does it — and none of that is
used on the error path.

**Closes when:** one failure prints one block: the host error in the readable
form, the call chain that led to it, and the suggestion engine's output. Raw
payloads behind `--verbose`.

---

## 2. Security backlog

### 2.1 Dependabot alerts

175 open at the time of writing: 8 critical, 75 high, 71 medium, 21 low.

| Manifest | Alerts |
|---|---|
| `package-lock.json` (root) | 83 |
| `docs/showcase/package-lock.json` | 60 |
| `go.mod` | 12 |
| `simulator/Cargo.lock` | 10 |
| `erst_extension/package-lock.json` | 7 |
| `playground-api/package-lock.json` | 2 |
| `simulator/fuzz/Cargo.toml` | 1 |

The dependency updates in this pass address the high and critical advisories in
dependencies that ship. They are uncommitted, so GitHub has not re-scanned and
the counts above still reflect the old state.

Two distinctions worth keeping straight:

- `scripts/security/npm-audit-gate.sh` gates on **high or critical in
  production dependencies**. It passes.
- Dependabot reports **everything, including dev dependencies**. Those alerts
  will remain after a push, and most of what is left is a vulnerable lint or
  test tool that no user of this project can reach.

**Closes when:** the gate passes on every manifest, and anything Dependabot
still reports is either a dev dependency or listed in
`.github/npm-audit-exceptions.txt` with a reason.

### 2.2 `@stellar/stellar-sdk` 14 → 17 migration

Accepted in `.github/npm-audit-exceptions.txt`. The SDK at 14.6.1 depends on
`toml` 5.x, which carries unfixed DoS and prototype-pollution advisories. The
only fix path is stellar-sdk 17, and the XDR codec API changed shape in
between: `TransactionMeta.v1()`, the `.switch()` accessors and the callable
array arms all became plain discriminated-union properties.

Attempting the bump produces **27 type errors** across:

- `src/footprint/extractor.ts`
- `src/xdr/decoder.ts`
- `src/xdr/types.ts`
- `src/rpc/fallback-client.ts`

Reachability: this project imports only `xdr` from the SDK. The `toml` parser is
SEP-1 `stellar.toml` handling, which nothing here calls, so the advisories are
not on a code path this repository executes.

**Closes when:** the four files are migrated, `tsc` is clean, and the exception
is removed.

### 2.3 `gosec` backlog

`gosec` had never run — it was gated on `matrix.go-version == '1.23'` while the
matrix had moved to 1.25, so GitHub skipped it every build and reported green.
The condition no longer hardcodes a version, so it runs from now on, but
non-blocking: a local run reports 104 high-severity findings.

| Rule | Count | Worth reading? |
|---|---|---|
| G115 int conversion | 74 | Mostly noise where XDR's sized integers meet Go's `int` |
| G703 path traversal | 17 | Yes |
| G404 weak RNG | 5 | Yes — `math/rand` where `crypto/rand` is expected |
| G702 `os/exec` | 3 | Constructed paths to fixed binaries (`launchctl`); low risk |
| G704 SSRF | 3 | Worth a look |
| G101 hardcoded credential | 1 | False positive — the Futurenet network passphrase |
| G122 WalkDir callback | 1 | Worth a look |

**Closes when:** G404 and G703 are triaged, G115 is excluded or addressed, and
`continue-on-error` is dropped from the step in `.github/workflows/ci.yml`.

### 2.4 Go standard library advisories requiring Go 1.26

`govulncheck` reports 36 vulnerabilities reachable from this code, 35 of them in
the standard library. CI pins were moved from `"1.25.0"` — the *oldest* 1.25
release — to `"1.26.x"`, which clears the ones fixed in patch releases.

What remains needs the `go` directive itself at 1.26:

- 12 standard library advisories fixed only in go1.26.9 / go1.26.10
- 5 advisories in `golang.org/x/net`, fixed in v0.60.0, which requires Go 1.26

The directive was deliberately left at 1.25.0 so contributors are not forced to
upgrade mid-flight with 17 pull requests open.

**Closes when:** the open PR queue is drained enough that bumping `go 1.26.0` in
`go.mod` is not disruptive.

### 2.5 `github.com/go-chi/chi` v4 is unmaintained

One `govulncheck` finding against `chi@v4.1.2+incompatible`, marked
`Fixed in: N/A`. chi v4 is no longer maintained; the fix is `chi/v5`.

**Closes when:** migrated to `chi/v5`.

---

## 3. CI and tooling debt

### 3.1 Actions pinned to moving branches

`securego/gosec@master` and `dtolnay/rust-toolchain@master` both track a branch.
Whoever owns those repositories can change what executes in CI, with access to
whatever the workflow can reach. The `unpinned-action` check in
`scripts/security/scan-diff.sh` flags this pattern on new additions but does not
retroactively fix these two.

**Closes when:** both are pinned to a release tag, or a commit SHA.

### 3.2 The version-string coupling that rotted four steps

Four steps in `.github/workflows/ci.yml` were gated on
`matrix.os == 'ubuntu-latest' && matrix.go-version == '<literal>'`. Bumping the
Go version silently switched them off and the job still reported green. The
conditions now check the OS alone.

Worth a convention: **a step's condition should never name a version**, because
nothing fails when it stops matching.

### 3.3 Dependabot grouping is inconsistent

The five npm entries added in this pass group minor and patch updates into one
pull request per project per week. The two that existed before —
`/erst_extension` and `/docs/showcase` — are ungrouped at a limit of 5.

**Closes when:** the same grouping is applied to both, or a reason is recorded
for the difference.

### 3.4 `docs/showcase` is unreferenced

`private: true`, no `vercel.json`, not built by any workflow, not mentioned in
`render.yaml` or the Makefile. It was migrated to Next 16 / React 19 in this
pass to clear five critical advisories, and the build was verified — but it
accounts for 60 of the 175 Dependabot alerts and nothing consumes it.

**Decide:** build it in CI so it stays honest, or delete it. Carrying an
unreferenced Next app is a standing maintenance cost for no return.

---

## 4. Repository hygiene

- `CHECKLIST.md` and `PR_MESSAGE.md` are listed under "Temporary docs/notes" in
  `.gitignore` yet are tracked. Either untrack them or remove the ignore rules;
  the half state confuses `git add`.
- `Contributor_Anomalies_Report.docx` (38 KB) and `GITHUB_ISSUES_DRAFT.txt` are
  tracked working documents. Decide whether they belong in the repository.
- The root `package.json` carries an `allowScripts` block, which is
  `@lavamoat/allow-scripts` configuration. lavamoat is not in the dependency
  tree, so nothing reads it. `.github/expected-install-scripts.txt` is the
  version that is enforced; remove the dead block or wire lavamoat up.
- `.vscode/extensions.json` and `.vscode/settings.json` are tracked but ignored.
  Intentional if the aim is to ignore *new* personal settings; worth confirming.

---

## 5. Dead interfaces

Flags and fields that exist on one side of the Go/Rust boundary and are ignored
on the other. Each one is a promise the CLI makes and does not keep.

| Interface | State |
|---|---|
| `--protocol-version` | The Go request carried it; the Rust `SimulationRequest` had no such field, so serde discarded it. Wired up in this pass — verify end to end. |
| `timestamp` (Rust) | Declared `String`, marked `#[allow(dead_code)]`, never consumed. Go sending a number into it failed the whole request, so `--timestamp N` was broken for anyone who tried. Now mapped to `ledger_timestamp`; the dead `String` field should be deleted. |
| `_timestamp` (Rust) | `Option<i64>`, underscore-prefixed, unread. Delete. |
| `clock_advance_seconds` | Declared in `types.rs`, never read by `main.rs`. Either wire it up or remove it. |
| `result_meta_xdr` (Rust) | Parsed into `let _result_meta` and discarded. The parse was against the wrong type until this pass; now correct, but still unused. Decide whether the simulator needs it at all. |

---

## 5b. Feature feasibility — verified against the host API

Checked against `soroban-env-host 25.2.1` and `soroban-wasmi
0.31.1-soroban.20.0.1` on 2026-11-02, so these are findings rather than
estimates.

### Not feasible as originally described

**Instruction-level stepping with local variable inspection.**
`soroban-wasmi`'s `call_resumable` resumes only after a *host error* — the type
is documented as "the resumable call encountered a host error and can be
resumed". A fuel trap returns a hard `Err` with no resumption path, and nothing
in `src/vm.rs` exposes a per-instruction hook. Delivering breakpoints at
arbitrary offsets with locals inspection would mean forking the VM.

Do not promise this in a grant application.

### Feasible, and what each one rests on

| Feature | Rests on | Notes |
|---|---|---|
| **Call-level debugger** | `TraceHook` with `EnvCall` / `EnvRet` / `PushCtx` / `PopCtx`, plus `dwarf` + `sourcemap` + existing snapshots | Step through host calls and contract frames mapped to Rust source, rewindable. Still a first for Soroban — but it is not instruction-level, and should not be described as if it were. `TraceHook` is `#[doc(hidden)]`, so treat it as an unstable API. |
| **Footprint doctor** | `Storage.footprint` is `pub`; `Footprint(pub FootprintMap)`; `AccessType::{ReadOnly, ReadWrite}`; `Host::try_finish() -> (Storage, Events)` | Over-declaration and access-type mismatches come from comparing the declared footprint against the keys the storage map actually holds after execution. Detecting *missing* keys needs a second pass using `Storage::with_recording_footprint(Rc<dyn SnapshotSource>)`, which requires implementing `SnapshotSource` over the loaded ledger entries — none exists in this repository yet. |
| **Auth tree debugging** | envelope `SorobanAuthorizationEntry` tree + `internal/authtrace` | Presentation and analysis over data already present. |
| **State expiry diagnostics** | TTL entries (now synthesised in `internal/rpc/ttl.go`) + replay ledger sequence | |
| **ABI-decoded state diff** | `internal/abi` + `internal/state` + `rpc.ExtractPostStateLedgerEntries` | |
| **Protocol-diff replay** | one simulator binary per protocol, selected by the Go side | Avoids linking two incompatible host versions into one binary. Depends on 1.1. |
| **Fork mode** | `internal/daemon` JSON-RPC + `internal/state` + `internal/snapshot` | Needs enough of the Soroban RPC surface for the SDKs to talk to it unmodified. |
| **GitHub Action** | the golden suite (1.2) | |

### Suggested build order

Footprint doctor first: it is self-contained, needs no change to the host
version, attacks a leading cause of transaction failure, and is demonstrable on
a single transaction.

## 6. Before applying to the Stellar Community Fund

Ordered by how much each changes a reviewer's impression. See the accompanying
discussion for reasoning.

- [ ] **1.1 first.** A debugger that cannot replay a transaction from the
      network as it runs today has no demo. Nothing else on this list matters
      as much.
- [ ] **1.2 next.** "We replay N real mainnet transactions and match the
      on-chain result every time" is the single most convincing sentence
      available, and it is a test suite, not a claim.
- [ ] Record a 90-second capture of one real failed transaction going from an
      opaque network error to a named cause with a source line.
- [ ] Narrow the pitch. 45 CLI commands against one broken core path reads as
      unfocused. Decide what `erst` *is* in one sentence and move the rest
      behind `erst experimental` or out of the README.
- [ ] Clear section 5. Dead interfaces are what a technical reviewer finds when
      they open the hood.
- [ ] Find users. 301 contributors and 14 stars is a contribution programme, not
      adoption. Three developers saying "this found a bug I could not find" is
      worth more than every commit in the history.
- [ ] Write the milestones as verifiable deliverables with dates, each one
      something a reviewer could check themselves.

---

## Done in the pass that produced this file

Context for anyone picking this up — not a to-do list.

- Nine defects in the replay path, from the footprint never being read through
  to ledger context being wiped by the first entry load. `erst debug` now
  resolves state, loads TTLs, sets ledger context and reaches contract
  execution.
- Two CI workflows for malicious code and supply chain, three scanner scripts,
  and the waiver files they read.
- Dependency updates: six Go modules, four Rust crates, four npm projects
  including a Next 14 → 16 / React 18 → 19 migration of `docs/showcase` with a
  verified build.
- CI Go pins moved off the oldest 1.25 patch; four step conditions decoupled
  from the version string; `gosec` runs for the first time.
- All seven npm projects added to Dependabot (previously two).

Verified at that point: Go 63 packages / 0 failures, Rust 625 tests / 0
failures, `go build`, `go vet`, `gofmt`, `cargo build`, `cargo clippy -D
warnings`, `cargo fmt` all clean.
