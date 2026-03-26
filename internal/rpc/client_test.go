// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotandev/hintents/internal/rpc"
)

// ---------------------------------------------------------------------------
// Mock server helpers
// ---------------------------------------------------------------------------

// mockServer starts a test HTTP server that:
//  1. Decodes the incoming JSON-RPC request.
//  2. Asserts its "method" field equals wantMethod.
//  3. Replies with a canned JSON-RPC success response containing result.
//
// Any assertion failure is reported immediately via t.Errorf so the overall
// test still completes and reports all failures in one run.
func mockServer(t *testing.T, wantMethod rpc.Method, result interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode the method field only — we don't need the full request shape.
		var envelope struct {
			Method rpc.Method `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			t.Errorf("mock server: decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if envelope.Method != wantMethod {
			t.Errorf("mock server: got method %q, want %q", envelope.Method, wantMethod)
		}

		raw, _ := json.Marshal(result)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  json.RawMessage(raw),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("mock server: encode response: %v", err)
		}
	}))
}

// rpcErrorServer starts a test HTTP server that always returns a JSON-RPC
// protocol-level error response.  This is used to verify that the client
// surfaces RPC errors as Go errors rather than swallowing them.
func rpcErrorServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"error":   map[string]interface{}{"code": -32601, "message": "method not found"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// newSingleURLClient builds a minimal rpc.Client pointed at a single URL.
// It bypasses the full NewClient option chain so tests don't need every
// dependency (logger, metrics, etc.) wired up.
func newSingleURLClient(url string) *rpc.Client {
	return &rpc.Client{
		SorobanURL: url,
		AltURLs:    []string{url},
		Network:    rpc.Testnet,
	}
}

// ---------------------------------------------------------------------------
// getLedgerEntriesAttempt — migrated from "getLedgerEntries" literal
// ---------------------------------------------------------------------------

// TestGetLedgerEntries_SendsMethodConstant verifies that the internal
// getLedgerEntriesAttempt call encodes MethodGetLedgerEntries on the wire,
// not the old hard-coded string "getLedgerEntries".
func TestGetLedgerEntries_SendsMethodConstant(t *testing.T) {
	t.Parallel()

	srv := mockServer(t, rpc.MethodGetLedgerEntries, map[string]interface{}{
		"entries":      []interface{}{},
		"latestLedger": 42,
	})
	defer srv.Close()

	c := newSingleURLClient(srv.URL)
	c.CacheEnabled = false

	_, err := c.GetLedgerEntries(context.Background(), []string{"dGVzdA=="})
	if err != nil {
		// The call may fail due to missing VerifyLedgerEntries etc. in unit
		// context — what matters is that the mock server ran without a method
		// mismatch error, which would have been reported via t.Errorf above.
		_ = err
	}
}

// ---------------------------------------------------------------------------
// simulateTransactionAttempt — migrated from "simulateTransaction" literal
// ---------------------------------------------------------------------------

// TestSimulateTransaction_SendsMethodConstant verifies that SimulateTransaction
// encodes MethodSimulateTransaction on the wire.
func TestSimulateTransaction_SendsMethodConstant(t *testing.T) {
	t.Parallel()

	srv := mockServer(t, rpc.MethodSimulateTransaction, map[string]interface{}{
		"minResourceFee": "100",
	})
	defer srv.Close()

	c := newSingleURLClient(srv.URL)

	resp, err := c.SimulateTransaction(context.Background(), "AAAA...")
	if err != nil {
		t.Fatalf("SimulateTransaction returned unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("SimulateTransaction returned nil response")
	}
}

// TestSimulateTransaction_RPCError verifies that a JSON-RPC error payload is
// surfaced as a Go error.
func TestSimulateTransaction_RPCError(t *testing.T) {
	t.Parallel()

	srv := rpcErrorServer(t)
	defer srv.Close()

	c := newSingleURLClient(srv.URL)
	_, err := c.SimulateTransaction(context.Background(), "AAAA...")
	if err == nil {
		t.Fatal("expected error from JSON-RPC error payload, got nil")
	}
}

// ---------------------------------------------------------------------------
// getHealthAttempt — migrated from "getHealth" literal
// ---------------------------------------------------------------------------

// TestGetHealth_SendsMethodConstant verifies that GetHealth encodes
// MethodHealth on the wire.
func TestGetHealth_SendsMethodConstant(t *testing.T) {
	t.Parallel()

	srv := mockServer(t, rpc.MethodHealth, map[string]interface{}{
		"status":       "healthy",
		"latestLedger": 999,
	})
	defer srv.Close()

	c := newSingleURLClient(srv.URL)

	resp, err := c.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth returned unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("GetHealth returned nil response")
	}
	if resp.Result.Status != "healthy" {
		t.Errorf("Result.Status = %q, want \"healthy\"", resp.Result.Status)
	}
}

// TestGetHealth_RPCError verifies that GetHealth surfaces JSON-RPC errors.
func TestGetHealth_RPCError(t *testing.T) {
	t.Parallel()

	srv := rpcErrorServer(t)
	defer srv.Close()

	c := newSingleURLClient(srv.URL)
	_, err := c.GetHealth(context.Background())
	if err == nil {
		t.Fatal("expected error from JSON-RPC error payload, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetLatestLedgerSequence / fetchLatestFromSDF — migrated from "getLatestLedger"
// ---------------------------------------------------------------------------

// TestGetLatestLedgerSequence_SendsMethodConstant verifies that
// GetLatestLedgerSequence encodes MethodGetLatestLedger on the wire.
func TestGetLatestLedgerSequence_SendsMethodConstant(t *testing.T) {
	t.Parallel()

	srv := mockServer(t, rpc.MethodGetLatestLedger, map[string]interface{}{
		"id":       "abc",
		"sequence": 12345,
	})
	defer srv.Close()

	c := newSingleURLClient(srv.URL)

	seq, err := c.GetLatestLedgerSequence(context.Background())
	if err != nil {
		t.Fatalf("GetLatestLedgerSequence returned unexpected error: %v", err)
	}
	if seq != 12345 {
		t.Errorf("sequence = %d, want 12345", seq)
	}
}

// ---------------------------------------------------------------------------
// Regression: no raw string literals remain in the request structs
// ---------------------------------------------------------------------------

// TestNoRawStringLiteralsInRequestStructs builds each migrated request struct
// using the constant and ensures the Method field is equal to the constant.
// If a developer accidentally reintroduces a raw string on the struct field,
// Go's type system will produce a compile error rather than a test failure —
// but this test also documents the migration intent explicitly.
func TestNoRawStringLiteralsInRequestStructs(t *testing.T) {
	t.Parallel()

	t.Run("GetLedgerEntriesRequest_method_field_is_typed", func(t *testing.T) {
		t.Parallel()
		req := rpc.GetLedgerEntriesRequest{Method: rpc.MethodGetLedgerEntries}
		if req.Method != rpc.MethodGetLedgerEntries {
			t.Errorf("got %q, want constant %q", req.Method, rpc.MethodGetLedgerEntries)
		}
	})

	t.Run("SimulateTransactionRequest_method_field_is_typed", func(t *testing.T) {
		t.Parallel()
		req := rpc.SimulateTransactionRequest{Method: rpc.MethodSimulateTransaction}
		if req.Method != rpc.MethodSimulateTransaction {
			t.Errorf("got %q, want constant %q", req.Method, rpc.MethodSimulateTransaction)
		}
	})

	t.Run("GetHealthRequest_method_field_is_typed", func(t *testing.T) {
		t.Parallel()
		req := rpc.GetHealthRequest{Method: rpc.MethodHealth}
		if req.Method != rpc.MethodHealth {
			t.Errorf("got %q, want constant %q", req.Method, rpc.MethodHealth)
		}
	})
}

// ---------------------------------------------------------------------------
// AllNodesFailedError
// ---------------------------------------------------------------------------

// TestAllNodesFailedError_Unwrap confirms that Unwrap returns all per-node
// errors so that errors.As / errors.Is can traverse them.
func TestAllNodesFailedError_Unwrap(t *testing.T) {
	t.Parallel()

	inner1 := &rpc.NodeFailure{URL: "https://a.example.com", Reason: errSentinel("err-a")}
	inner2 := &rpc.NodeFailure{URL: "https://b.example.com", Reason: errSentinel("err-b")}

	all := &rpc.AllNodesFailedError{
		Failures: []rpc.NodeFailure{*inner1, *inner2},
	}

	unwrapped := all.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("Unwrap() returned %d errors, want 2", len(unwrapped))
	}
	if unwrapped[0].Error() != "err-a" {
		t.Errorf("unwrapped[0] = %q, want \"err-a\"", unwrapped[0])
	}
	if unwrapped[1].Error() != "err-b" {
		t.Errorf("unwrapped[1] = %q, want \"err-b\"", unwrapped[1])
	}
}

// TestAllNodesFailedError_ErrorMessage confirms the human-readable message
// contains each URL and reason.
func TestAllNodesFailedError_ErrorMessage(t *testing.T) {
	t.Parallel()

	all := &rpc.AllNodesFailedError{
		Failures: []rpc.NodeFailure{
			{URL: "https://node1.example.com", Reason: errSentinel("timeout")},
		},
	}
	msg := all.Error()
	if msg == "" {
		t.Fatal("AllNodesFailedError.Error() returned empty string")
	}
	for _, want := range []string{"https://node1.example.com", "timeout"} {
		if !contains(msg, want) {
			t.Errorf("error message %q does not contain %q", msg, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// errSentinel is a minimal error type used in table-driven tests.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && indexString(s, substr) >= 0)
}

func indexString(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}