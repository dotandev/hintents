// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	ierrors "github.com/dotandev/hintents/internal/errors"
)

// knownLimitations maps a golden fixture's tx hash to why its replay is
// expected to diverge from the recorded on-chain outcome, for a reason this
// repo already knows about and has deliberately not fixed yet (tracked in
// ROADMAP.md §1.3, not something TestGoldenReplay's own scope covers). A
// fixture listed here is skipped rather than asserted on, with the reason
// printed so the skip is loud, not silent. The day the underlying gap is
// fixed, this fixture will no longer diverge, and the entry should be
// deleted — at which point TestGoldenReplay starts asserting on it like any
// other fixture.
var knownLimitations = map[string]string{
	"4802624d9c980ab188a0a837419a22c2900febe094556e8cedf5e77363c97606": "ROADMAP.md §1.3 item 2: network_id is zeroed in SimHost::set_ledger_context, " +
		"so auth-signature verification diverges from the real network. This transaction " +
		"succeeded on-chain (confirmed directly against Horizon) but fails replay here with " +
		"Auth/InvalidAction on a require_auth check — exactly network_id's documented effect.",
	// Second, independent confirmation of the same gap: a different
	// contract, different invoked function (set_scores vs set_prices),
	// different signer — same divergence (on-chain succeeded, replay fails
	// on require_auth with Auth/InvalidAction). Found while looking for a
	// second "clean success" fixture; every address-authenticated
	// invoke_host_function transaction tried so far hits this, consistent
	// with network_id being zeroed for every signature check, not an
	// occasional edge case.
	"255181d413307b6e466f5b146f8d37d503f3b08207ccf2b0afe86f6a335d16c9": "ROADMAP.md §1.3 item 2: network_id is zeroed in SimHost::set_ledger_context, " +
		"so auth-signature verification diverges from the real network. This transaction " +
		"succeeded on-chain (confirmed directly against Horizon) but fails replay here with " +
		"Auth/InvalidAction on a require_auth check — a second, independent confirmation of " +
		"network_id's documented effect.",
}

// TestGoldenReplay asserts that replaying each fixture in testdata/golden/
// through the real erst-sim binary produces the same success/failure outcome
// the network recorded (ROADMAP.md §1.2). It makes no network calls — every
// fixture is a static, pre-captured JSON file (see golden_capture_test.go for
// how they're produced).
//
// Unlike TestPerfRegression, this can't run against a MockRunner — testing
// the real simulator's behavior against real recorded outcomes is the whole
// point. So unlike most of this package's tests, it needs the actual
// erst-sim binary (via ERST_SIM_PATH or PATH), which the plain `go test
// ./...` invocation in ci.yml's `go` job never builds — only the dedicated
// `golden-replay` job does. It skips (not fails) when the binary isn't
// available, so `go test ./...` stays green everywhere else.
func TestGoldenReplay(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get current file path")
	}
	fixtureDir := filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "golden")

	fixturePaths, err := filepath.Glob(filepath.Join(fixtureDir, "*.json"))
	if err != nil {
		t.Fatalf("failed to glob fixtures in %s: %v", fixtureDir, err)
	}
	if len(fixturePaths) == 0 {
		t.Fatalf("no golden fixtures found in %s — run TestGenerateGoldenFixtures "+
			"(GENERATE_GOLDEN_FIXTURES=1) to populate it", fixtureDir)
	}

	runner, err := NewRunnerInterface()
	if err != nil {
		if erstErr, ok := err.(*ierrors.ErstError); ok && erstErr.Code == ierrors.ErstSimulatorNotFound {
			t.Skipf("erst-sim binary not available (set ERST_SIM_PATH or build it "+
				"first to run this test for real): %v", err)
		}
		t.Fatalf("failed to resolve erst-sim runner: %v", err)
	}
	defer runner.Close()

	for _, path := range fixturePaths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read %s: %v", path, err)
			}

			var fixture GoldenFixture
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatalf("failed to parse %s: %v", path, err)
			}

			req := &SimulationRequest{
				EnvelopeXdr:    fixture.EnvelopeXdr,
				ResultMetaXdr:  fixture.ResultMetaXdr,
				LedgerEntries:  fixture.LedgerEntries,
				Timestamp:      fixture.LedgerCloseTime,
				LedgerSequence: fixture.LedgerSequence,
				// ProtocolVersion deliberately left nil — see the comment on
				// goldenFixtureTargets in golden_capture_test.go for why the
				// fixture's captured (real, current) protocol isn't fed to a
				// host pinned several protocols behind.
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Runner.Run never returns (resp, nil) with a failure embedded in
			// resp.Status for a logical execution failure — it converts any
			// such failure (and any infrastructure failure) into the returned
			// error instead, exactly like `erst debug`/`erst compare` already
			// treat it (see internal/cmd/debug.go's `errors.WrapSimulationFailed
			// (err, "")` on any non-nil err from Run). So "did replay predict
			// success" is "did Run return a nil error", not a resp.Status check.
			resp, err := runner.Run(ctx, req)
			gotSuccessful := err == nil

			if reason, known := knownLimitations[fixture.TxHash]; known {
				if gotSuccessful == fixture.Expected.Successful {
					t.Fatalf("fixture %s is listed in knownLimitations as a divergence "+
						"(%s), but replay now matches the recorded outcome (both %v) — "+
						"the underlying gap may be fixed; remove this fixture's entry from "+
						"knownLimitations so it's asserted on normally",
						fixture.TxHash, reason, gotSuccessful)
				}
				t.Skipf("known limitation, not asserted: %s", reason)
				return
			}

			if gotSuccessful != fixture.Expected.Successful {
				var status string
				if resp != nil {
					status = resp.Status
				}
				t.Fatalf("replay of %s: local result was successful=%v (status=%q, err=%v), "+
					"but the network recorded successful=%v",
					fixture.TxHash, gotSuccessful, status, err, fixture.Expected.Successful)
			}
		})
	}
}
