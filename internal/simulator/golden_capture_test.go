// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/rpc"
)

// GoldenFixture is a pinned, checked-in real transaction used by
// TestGoldenReplay (golden_replay_test.go) to assert that erst-sim's local
// replay matches what the network actually recorded. See ROADMAP.md §1.2.
//
// Every field needed to replay fully offline is captured explicitly,
// including ProtocolVersion — recorded for provenance even though
// TestGoldenReplay does not currently pass it to the simulator. The host
// accepts only its own linked protocol (confirmed by hand — an explicit
// older protocol is rejected as "too old for host", a newer one as the
// contract's protocol being newer than the host), so fixtures captured from
// the current chain tip (protocol 29) will record a ProtocolVersion the
// pinned host (27) can't actually be asked to honor yet. See
// goldenFixtureTargets below for why current, rather than historical,
// transactions are used.
type GoldenFixture struct {
	Network         string            `json:"network"`
	TxHash          string            `json:"tx_hash"`
	EnvelopeXdr     string            `json:"envelope_xdr"`
	ResultMetaXdr   string            `json:"result_meta_xdr"`
	LedgerSequence  uint32            `json:"ledger_sequence"`
	LedgerCloseTime int64             `json:"ledger_close_time"`
	ProtocolVersion uint32            `json:"protocol_version"`
	LedgerEntries   map[string]string `json:"ledger_entries"`
	Expected        GoldenExpected    `json:"expected"`
}

// GoldenExpected is the ground truth a replay is judged against. Scoped to
// status (success/fail) for now — see ROADMAP.md §1.2 and the plan behind
// this change for why richer event/state-diff comparisons aren't attempted
// yet (the ground-truth primitives for those aren't exported/shaped for this
// use today).
type GoldenExpected struct {
	Successful bool `json:"successful"`
}

type goldenFixtureTarget struct {
	network rpc.Network
	txHash  string
}

// goldenFixtureTargets are real, current testnet transactions, not
// historical ones, and not mainnet. Two things ruled those out, both
// documented in full in docs/REPLAY_FIDELITY.md:
//
//   - An earlier attempt pinned transactions from testnet's protocol-27 era
//     specifically (matching the pinned soroban-env-host build), found by
//     binary-searching Horizon for the 27→28 ledger boundary. Every one
//     failed ResolveReplayState with "EOF while decoding 4 bytes": Soroban
//     RPC's getTransaction only retains result-meta XDR for a short rolling
//     window, far shorter than Horizon's transaction history. By the time a
//     network's protocol has moved past 27 (both already have, to 29), the
//     XDR data needed to replay a transaction from that now-past era has
//     typically already expired from RPC — it isn't just stale, it's gone.
//     This is a sharper version of the "no archival ledger-state source" gap
//     in ROADMAP.md §1.3.
//   - Mainnet: Horizon no longer serves result_meta_xdr for mainnet
//     transactions at all (confirmed directly — present and populated on
//     testnet, empty on mainnet for an equally recent transaction), and the
//     client's fallback to Soroban RPC's getTransaction via the public demo
//     endpoint didn't succeed either. Scoped out for this pass rather than
//     chasing a reliable mainnet RPC endpoint; see docs/REPLAY_FIDELITY.md.
//
// So: current testnet transactions, fully fetchable today. Their captured
// ProtocolVersion is real (29 at capture time) but TestGoldenReplay does not
// pass it to the simulator — the host is pinned to 27 and rejects a declared
// protocol newer than itself with exactly the "contract protocol number is
// newer than host" error ROADMAP §1.1 opens with. golden_replay_test.go
// leaves ProtocolVersion unset, matching what `erst debug` does today
// without an explicit --protocol-version override.
var goldenFixtureTargets = []goldenFixtureTarget{
	// Clean success: on-chain succeeded, replay succeeds.
	{network: rpc.Testnet, txHash: "255181d413307b6e466f5b146f8d37d503f3b08207ccf2b0afe86f6a335d16c9"},
	// Clean failure: on-chain failed (archived contract-data entry, TTL
	// expired before replay), replay fails the same way.
	{network: rpc.Testnet, txHash: "f605fb6ad8d2d34f8ea0f6ad1832d2b1047ef17fad4ad1ea74f6b9d97f6eab11"},
	// Known-limitation case, kept deliberately: on-chain SUCCEEDED
	// (confirmed directly against Horizon), but replay fails with
	// Auth/InvalidAction on a require_auth check. This is exactly the
	// real-world shape of ROADMAP §1.3 item 2 ("network_id is zeroed...
	// anything verifying an auth signature against the network passphrase
	// will diverge") — see golden_replay_test.go's knownLimitations map and
	// docs/REPLAY_FIDELITY.md. Kept in the suite as a live regression
	// tracker: it will start failing TestGoldenReplay's own skip-assertion
	// (and need the map entry removed) the day that gap gets fixed.
	{network: rpc.Testnet, txHash: "4802624d9c980ab188a0a837419a22c2900febe094556e8cedf5e77363c97606"},
}

// TestGenerateGoldenFixtures captures goldenFixtureTargets from live RPC and
// writes them to testdata/golden/. This is the only code in the golden-replay
// suite that ever makes a network call; TestGoldenReplay (golden_replay_test.go)
// never does. Not run in CI.
//
// Run it with: GENERATE_GOLDEN_FIXTURES=1 go test -run TestGenerateGoldenFixtures ./internal/simulator/... -v
func TestGenerateGoldenFixtures(t *testing.T) {
	if os.Getenv("GENERATE_GOLDEN_FIXTURES") != "1" {
		t.Skip("Skipping golden fixture capture. Set GENERATE_GOLDEN_FIXTURES=1 to run (requires live network access).")
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path")
	}
	outDir := filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "golden")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("failed to create %s: %v", outDir, err)
	}

	for _, target := range goldenFixtureTargets {
		target := target
		t.Run(fmt.Sprintf("%s_%s", target.network, target.txHash[:12]), func(t *testing.T) {
			client, err := rpc.NewClient(rpc.WithNetwork(target.network))
			if err != nil {
				t.Fatalf("failed to build RPC client: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			resp, err := client.GetTransaction(ctx, target.txHash)
			if err != nil {
				t.Fatalf("GetTransaction(%s): %v", target.txHash, err)
			}

			ledgerEntries, err := client.ResolveReplayState(ctx, resp.EnvelopeXdr, resp.ResultMetaXdr)
			if err != nil {
				t.Fatalf("ResolveReplayState(%s): %v", target.txHash, err)
			}

			header, err := client.GetLedgerHeader(ctx, resp.LedgerSequence)
			if err != nil {
				t.Fatalf("GetLedgerHeader(%d): %v", resp.LedgerSequence, err)
			}

			fixture := GoldenFixture{
				Network:         string(target.network),
				TxHash:          target.txHash,
				EnvelopeXdr:     resp.EnvelopeXdr,
				ResultMetaXdr:   resp.ResultMetaXdr,
				LedgerSequence:  resp.LedgerSequence,
				LedgerCloseTime: resp.LedgerCloseTime,
				ProtocolVersion: header.ProtocolVersion,
				LedgerEntries:   ledgerEntries,
				Expected:        GoldenExpected{Successful: resp.Successful},
			}

			data, err := json.MarshalIndent(fixture, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal fixture: %v", err)
			}

			path := filepath.Join(outDir, fmt.Sprintf("%s_%s.json", target.network, target.txHash[:12]))
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatalf("failed to write %s: %v", path, err)
			}
			t.Logf("wrote fixture %s (protocol %d, successful=%v, %d ledger entries)",
				path, fixture.ProtocolVersion, fixture.Expected.Successful, len(fixture.LedgerEntries))
		})
	}
}
