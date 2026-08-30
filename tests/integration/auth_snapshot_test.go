// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"testing"

	"github.com/dotandev/hintents/internal/authtrace"
	"github.com/dotandev/hintents/internal/trace"
)

func TestAuthSnapshotMultiSigFlow(t *testing.T) {
	const accountID = "GACCOUNT"

	tracker := authtrace.NewTracker(authtrace.Config{MaxEventDepth: 100})
	tracker.InitializeAccountContext(
		accountID,
		[]authtrace.SignerInfo{
			{AccountID: accountID, SignerKey: "invoker", SignerType: authtrace.Ed25519, Weight: 1},
			{AccountID: accountID, SignerKey: "address", SignerType: authtrace.Ed25519, Weight: 1},
		},
		authtrace.ThresholdConfig{HighThreshold: 2},
	)

	execTrace := trace.NewExecutionTrace("auth-snapshot", 1)

	tracker.RecordSignatureVerification(accountID, "invoker", authtrace.Ed25519, true, 1)
	tracker.RecordThresholdCheck(accountID, 2, 1, false)
	execTrace.AddState(trace.ExecutionState{
		Operation: "require_auth",
		EventType: trace.EventTypeAuth,
		HostState: map[string]interface{}{
			"signature_balance": 1,
			"account_nonce":     1,
			"auth_events":       authEventsAsSnapshotList(tracker.GetAuthEvents(accountID)),
		},
	})

	tracker.RecordSignatureVerification(accountID, "address", authtrace.Ed25519, true, 1)
	tracker.RecordThresholdCheck(accountID, 2, 2, true)
	execTrace.AddState(trace.ExecutionState{
		Operation: "require_auth",
		EventType: trace.EventTypeAuth,
		HostState: map[string]interface{}{
			"signature_balance": 2,
			"account_nonce":     2,
			"auth_events":       authEventsAsSnapshotList(tracker.GetAuthEvents(accountID)),
		},
	})

	if got := len(execTrace.Snapshots); got != 2 {
		t.Fatalf("snapshots count = %d, want 2", got)
	}

	cases := []struct {
		step            int
		wantNonce       int
		wantSigBalance  int
		wantEventLength int
	}{
		{step: 0, wantNonce: 1, wantSigBalance: 1, wantEventLength: 2},
		{step: 1, wantNonce: 2, wantSigBalance: 2, wantEventLength: 4},
	}

	for _, tc := range cases {
		reconstructed, err := execTrace.ReconstructStateAt(tc.step)
		if err != nil {
			t.Fatalf("ReconstructStateAt(%d) failed: %v", tc.step, err)
		}

		if got := asInt(t, reconstructed.HostState["account_nonce"]); got != tc.wantNonce {
			t.Fatalf("step %d nonce = %d, want %d", tc.step, got, tc.wantNonce)
		}
		if got := asInt(t, reconstructed.HostState["signature_balance"]); got != tc.wantSigBalance {
			t.Fatalf("step %d signature_balance = %d, want %d", tc.step, got, tc.wantSigBalance)
		}

		events, ok := reconstructed.HostState["auth_events"].([]map[string]interface{})
		if !ok {
			t.Fatalf("step %d auth_events type = %T, want []map[string]interface{}", tc.step, reconstructed.HostState["auth_events"])
		}
		if len(events) != tc.wantEventLength {
			t.Fatalf("step %d auth_events length = %d, want %d", tc.step, len(events), tc.wantEventLength)
		}
	}

	authTrace := tracker.GenerateTrace()
	if authTrace.ValidSignatures != 2 {
		t.Fatalf("valid signatures = %d, want 2", authTrace.ValidSignatures)
	}
	if len(authTrace.AuthEvents) != 4 {
		t.Fatalf("auth events = %d, want 4", len(authTrace.AuthEvents))
	}
	if got := execTrace.FilteredStepCount(trace.EventTypeAuth); got != 2 {
		t.Fatalf("filtered auth step count = %d, want 2", got)
	}
}

func authEventsAsSnapshotList(events []authtrace.AuthEvent) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]interface{}{
			"event_type": e.EventType,
			"status":     e.Status,
			"signer_key": e.SignerKey,
			"weight":     int(e.Weight),
		})
	}
	return out
}

func asInt(t *testing.T, v interface{}) int {
	t.Helper()
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint32:
		return int(n)
	case float64:
		return int(n)
	default:
		t.Fatalf("unexpected numeric type %T", v)
		return 0
	}
}
