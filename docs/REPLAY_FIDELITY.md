# Replay Fidelity: Known Limitations

This document tracks the ways a local `erst` replay can differ from what
actually happened on the real network, per ROADMAP.md §1.3's closing
condition: each gap is either fixed, or documented here as an explicit
limitation. It is not a general troubleshooting guide — it's a list of
specific, diagnosed divergences between local replay and on-chain reality.

The golden replay suite (`internal/simulator/golden_replay_test.go`,
ROADMAP.md §1.2) is what surfaces these in practice: it replays real, pinned
transactions and asserts the local result matches what the network recorded.
Two of its three fixtures currently fail that assertion for the reason in
item 1 below — tracked as `knownLimitations` entries in that file rather than
hidden or silently passed.

## 1. `network_id` is zeroed, so auth-signature verification diverges

**Status: confirmed with two independent real transactions, not fixed.**

`SimHost::set_ledger_context` (`simulator/src/runner.rs`) leaves
`network_id: [0u8; 32]` instead of the real network's passphrase hash.
Anything that verifies a signature against the network passphrase — which is
exactly what `require_auth` does for a classic (non-contract) address — computes
a different expected signing payload locally than the real network did, so a
perfectly valid signature fails verification during replay.

This was found, not theorized: the golden replay suite's first two fixture
candidates — two different testnet transactions, different contracts
(`set_prices` and `set_scores`), different signing addresses — both **succeeded
on-chain** (confirmed directly against Horizon's `successful` field) but **fail
local replay** with:

```
Host(HostError: Error(Auth, InvalidAction)
...
[Diagnostic Event] ..., data:"escalating error to VM trap from failed host function call: require_auth"
[Diagnostic Event] ..., data:["Unauthorized function call for address", G...]
```

Every address-authenticated `invoke_host_function` transaction tried while
building the golden suite hit this — it is not an occasional edge case, it is
the default outcome for this class of transaction. A clean, fully-passing
"success" fixture using real address-based auth could not be found; the one
clean-passing fixture in the suite (`testnet_f605fb6ad8d2.json`) is a failure
case for an unrelated reason (an archived/TTL-expired contract-data entry),
not a success case.

**Impact:** any `erst debug`/`erst compare` run against a transaction whose
outcome depends on `require_auth` for a classic address should be read with
this in mind — a local "failed" result does not necessarily mean the real
transaction failed, and the specific error (`Auth, InvalidAction`) is itself a
signal that this gap, not a real contract defect, may be the cause.

**Closes when:** `network_id` is set to the real network's passphrase hash in
`SimHost::set_ledger_context`, sourced from whichever network the replay
targets (mainnet, testnet, futurenet each have their own fixed passphrase
string). At that point the two `knownLimitations` entries in
`golden_replay_test.go` should stop diverging and can be deleted — the suite
is designed so that happens visibly (removing a stale entry makes the test
assert normally, rather than requiring anyone to notice and update it).

## 2. Ledger entries fetched for an uncovered footprint key are current, not historical

**Status: documented, not fixed.**

When transaction metadata doesn't cover a footprint key, `Client
.ResolveReplayState` (`internal/rpc/footprint.go`) falls back to
`GetLedgerEntries`, which returns the entry as it stands *now* — exact for
contract code (immutable once deployed), an approximation for contract data
that may have changed since the transaction's ledger closed. There is no
archival ledger-state source wired up.

This is sharper than it first appears: the golden suite's fixture-capture
attempt discovered that once a network's protocol has moved past a given
version, the *live RPC data needed to replay a transaction from that earlier
era* is often not just stale but **entirely gone** — Soroban RPC's
`getTransaction` only retains result-meta XDR for a short rolling window, far
shorter than Horizon's transaction history. An attempt to pin golden fixtures
from testnet's protocol-27 era (to match the simulator's then-current pinned
host version) failed outright with `ResolveReplayState` errors on every
candidate, because the metadata had already expired from RPC by the time it
was fetched — not approximated, unavailable. See the comment on
`goldenFixtureTargets` in `internal/simulator/golden_capture_test.go` for the
full account.

**Closes when:** an archival ledger-state source (e.g. a Stellar history
archive reader) is wired into the replay-state resolution path as a fallback
when live RPC no longer has the data.

## 3. Mainnet transactions currently can't be captured as golden fixtures at all

**Status: documented, not fixed; related to item 2 but distinct.**

Separately from the historical-data problem above, **current** mainnet
transactions also failed fixture capture: Horizon no longer serves
`result_meta_xdr` for mainnet transactions (confirmed directly — present and
populated for a testnet transaction, empty for an equally recent mainnet
one), and the client's fallback to Soroban RPC's `getTransaction` via the
public demo endpoint (`ValidationCloudMainnet`) did not succeed either. The
golden suite is testnet-only for now as a result; ROADMAP.md §1.2's ask for
"mainnet and testnet" fixtures isn't fully met yet.

**Closes when:** either a reliable mainnet Soroban RPC endpoint is identified
and wired in (possibly requiring an API key rather than the public demo
endpoint), or mainnet's own archival/history-archive access (item 2) covers
this too.

## 4. Protocol version defaults to the host's own, not the replayed ledger's real one

**Status: documented, not fixed — and more constrained than a simple default.**

`erst debug`'s `SimulationRequest.ProtocolVersion` is `nil` unless
`--protocol-version` is passed explicitly, so replay silently runs under
whatever protocol the linked `soroban-env-host` build happens to be, not the
protocol the transaction's ledger was actually on. `rpc.GetLedgerHeader`
already returns the real value; it isn't wired into the default.

Fixing this isn't just "wire in the real value," though — the host accepts
only its own linked protocol. Confirmed by hand: passing an explicitly older
protocol gets `"ledger protocol version too old for host"`; a newer one gets
the `"contract protocol number is newer than host"` error ROADMAP §1.1 opens
with. Since both mainnet and testnet have already moved to protocol 29 while
the pinned host is 27, defaulting to the *real* protocol today would make
`erst debug` fail outright on every current transaction — worse than the
current silent approximation, not better. A real fix needs to compare the
real protocol against the host's own before deciding whether to pass it
through, and warn rather than fail when they don't match. Out of scope for
the golden-suite work; noted here rather than attempted as a quick default
flip.

**Closes when:** `erst debug` passes the real protocol version only when it's
one the linked host supports, and clearly warns (rather than silently
defaulting) when it isn't.

## 5. Budget limits are not the network's

**Status: documented, not fixed.**

`applyPagedMemoryLimit` (`internal/simulator/runner.go`) sets a flat 64 MiB
ceiling, which replaces the host's default budget rather than matching the
protocol's real resource limits.

**Closes when:** the ceiling is sourced from the real per-protocol limits
rather than a fixed constant.
