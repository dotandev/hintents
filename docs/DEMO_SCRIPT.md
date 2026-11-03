# 90-second demo: opaque network error → named cause

Per ROADMAP.md §6: *"Record a 90-second capture of one real failed transaction
going from an opaque network error to a named cause with a source line."*

This uses a real, currently-failing testnet transaction — no setup, no demo
contract, nothing staged. It's one of the golden replay suite's own fixtures
(`testdata/golden/testnet_f605fb6ad8d2.json`), so what you see below is
exactly what the suite already asserts is a correct, reproducible diagnosis,
not a cherry-picked one-off.

**Transaction:** `f605fb6ad8d2d34f8ea0f6ad1832d2b1047ef17fad4ad1ea74f6b9d97f6eab11`
(testnet, confirmed failed on-chain — Horizon's `successful: false`)

## Setup (off-camera, before recording)

```bash
# Build both binaries once so the recording has no compile waits in it.
go build -o erst ./cmd/erst
cd simulator && cargo build && cd ..
export ERST_SIMULATOR_PATH="$PWD/simulator/target/debug/erst-sim"
```

## Beat 1 — the opaque network error (0:00–0:25)

Open a browser (or `curl`) to Horizon's own transaction page. This is what a
developer sees today with no tooling beyond the network itself.

```bash
curl -s "https://horizon-testnet.stellar.org/transactions/f605fb6ad8d2d34f8ea0f6ad1832d2b1047ef17fad4ad1ea74f6b9d97f6eab11" \
  | python3 -c "import json,sys; d=json.load(sys.stdin); print('successful:', d['successful']); print('result_xdr:', d['result_xdr'])"
```

**On screen:**
```
successful: False
result_xdr: AAAAAAAAq0j/////AAAAAQAAAAAAAAAY/////gAAAAA=
```

**Narration:** "A transaction failed on testnet. This is everything the
network itself tells you: a boolean, and an opaque base64 blob. No indication
of what actually went wrong."

## Beat 2 — run it through erst (0:25–0:40)

```bash
./erst debug f605fb6ad8d2d34f8ea0f6ad1832d2b1047ef17fad4ad1ea74f6b9d97f6eab11 --network testnet
```

**Narration (while it runs, ~2 seconds):** "erst fetches the transaction,
resolves the exact ledger state it ran against, and replays it locally
against the real Soroban host."

## Beat 3 — the named cause (0:40–1:15)

**On screen (the key line, called out/zoomed if the recording tool supports it):**
```
INVOKE_HOST_FUNCTION_ENTRY_ARCHIVED: contract data entry is archived
(key_hash: eba34ee007ce783486145010b8e900e30971b78bb2201fec68cc3bd780ce01ce,
 live_until_ledger: 5536377, replay_sequence: 5536391)
```

**Narration:** "Not a boolean — a named cause. The contract tried to read a
piece of its own storage that had already expired: it was only guaranteed to
live until ledger 5,536,377, and this transaction landed at ledger 5,536,391 —
14 ledgers, about 70 seconds, too late. That's the actual reason this failed,
and it's the kind of thing that's easy to miss when writing a contract:
forgetting to extend an entry's TTL before it's needed again."

## Beat 4 — close (1:15–1:30)

**Narration:** "This isn't a one-off — it's one of three transactions pinned
in erst's own test suite specifically because the result is checked against
what the network actually recorded, every time the project's CI runs."

(Optional, if there's room: mention this is the TTL-expiry diagnostic
ROADMAP.md's "State expiry diagnostics" feature row already covers — i.e. not
an isolated trick, but a designed capability.)

---

## Before recording: one thing worth fixing first

The real output today actually prints that error **three times** in slightly
different wrapping (`SIMULATION_FAILED: : SIM_EXECUTION_FAILED: : ...`,
repeated) — this is ROADMAP.md §1.4 ("Failure output is unreadable"), a known,
still-open issue. It doesn't change what Beat 3 says, but it's visually
repetitive and would look rougher than it needs to in the recording. Worth
asking to have that cleaned up (one block: readable error, call chain,
suggestion output) before filming — a comparatively small, well-scoped fix.

## Alternative, stronger version (more setup, truer to "source line")

The transaction above fails at the **host/ledger level**, before the
contract's own code runs — so there's no Wasm source line to point to, just
the ledger-state fact above. ROADMAP's phrasing ("a named cause with a source
line") is most literally satisfied by a failure *inside* a contract's own
logic — a panic, an assertion, an explicit error code — which needs a
contract whose source *we* have, so `erst`'s source-mapping can point at an
exact line.

That means: write a tiny Soroban contract that panics under a specific
condition, deploy it to testnet (needs a funded test account via friendbot),
invoke it in the failing way, then run `erst debug` and show the trace landing
on the exact `.rs` line. More setup (an hour or so, not 90 seconds), but a
stronger, literal match to the ask, and reusable as a permanent example
contract for documentation too. Say the word and I'll build and deploy it.
