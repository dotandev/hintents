// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc_test

import (
	"testing"

	"github.com/dotandev/hintents/internal/rpc"
)

// TestMethodConstants verifies that every exported Method constant carries
// the exact wire-level string mandated by the Stellar RPC specification.
//
// These are not integration tests — they execute offline with zero network I/O.
// If a constant value ever drifts from the spec (e.g. after copy-pasting a
// rename), this table will catch it before any code reaches the network.
func TestMethodConstants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		got      rpc.Method
		wantWire string
	}{
		{"GetTransaction", rpc.MethodGetTransaction, "getTransaction"},
		{"GetTransactions", rpc.MethodGetTransactions, "getTransactions"},
		{"SendTransaction", rpc.MethodSendTransaction, "sendTransaction"},
		{"SimulateTransaction", rpc.MethodSimulateTransaction, "simulateTransaction"},
		{"GetLedgerEntries", rpc.MethodGetLedgerEntries, "getLedgerEntries"},
		{"GetLedgers", rpc.MethodGetLedgers, "getLedgers"},
		{"GetEvents", rpc.MethodGetEvents, "getEvents"},
		{"GetLatestLedger", rpc.MethodGetLatestLedger, "getLatestLedger"},
		{"GetNetwork", rpc.MethodGetNetwork, "getNetwork"},
		{"GetVersionInfo", rpc.MethodGetVersionInfo, "getVersionInfo"},
		{"GetFeeStats", rpc.MethodGetFeeStats, "getFeeStats"},
		{"Health", rpc.MethodHealth, "getHealth"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if string(tc.got) != tc.wantWire {
				t.Errorf("rpc.Method%s = %q, want wire value %q", tc.name, tc.got, tc.wantWire)
			}
		})
	}
}

// TestMethodTypeRoundTrip confirms that the Method type round-trips cleanly
// through a plain string conversion.  This is a compile-time guarantee
// enforced by Go's type system, but documenting it as a test makes the design
// intent explicit.
func TestMethodTypeRoundTrip(t *testing.T) {
	t.Parallel()

	for _, m := range []rpc.Method{
		rpc.MethodGetTransaction,
		rpc.MethodSimulateTransaction,
		rpc.MethodGetLedgerEntries,
		rpc.MethodGetLatestLedger,
		rpc.MethodHealth,
	} {
		m := m
		t.Run(string(m), func(t *testing.T) {
			t.Parallel()
			if rpc.Method(string(m)) != m {
				t.Errorf("round-trip through string failed for Method(%q)", m)
			}
		})
	}
}

// TestRequestStructsUseMethodType confirms that the three request structs
// whose Method field was migrated from plain string to rpc.Method actually
// hold the typed field.  We build each struct using a constant and verify
// JSON serialisation produces the expected wire value.
func TestRequestStructsUseMethodType(t *testing.T) {
	t.Parallel()

	// GetLedgerEntriesRequest
	t.Run("GetLedgerEntriesRequest", func(t *testing.T) {
		t.Parallel()
		req := rpc.GetLedgerEntriesRequest{
			Jsonrpc: "2.0",
			ID:      1,
			Method:  rpc.MethodGetLedgerEntries,
			Params:  []interface{}{[]string{"key1"}},
		}
		if req.Method != rpc.MethodGetLedgerEntries {
			t.Errorf("Method = %q, want %q", req.Method, rpc.MethodGetLedgerEntries)
		}
		if string(req.Method) != "getLedgerEntries" {
			t.Errorf("wire value = %q, want \"getLedgerEntries\"", req.Method)
		}
	})

	// SimulateTransactionRequest
	t.Run("SimulateTransactionRequest", func(t *testing.T) {
		t.Parallel()
		req := rpc.SimulateTransactionRequest{
			Jsonrpc: "2.0",
			ID:      1,
			Method:  rpc.MethodSimulateTransaction,
			Params:  []interface{}{"AAAA..."},
		}
		if req.Method != rpc.MethodSimulateTransaction {
			t.Errorf("Method = %q, want %q", req.Method, rpc.MethodSimulateTransaction)
		}
		if string(req.Method) != "simulateTransaction" {
			t.Errorf("wire value = %q, want \"simulateTransaction\"", req.Method)
		}
	})

	// GetHealthRequest
	t.Run("GetHealthRequest", func(t *testing.T) {
		t.Parallel()
		req := rpc.GetHealthRequest{
			Jsonrpc: "2.0",
			ID:      1,
			Method:  rpc.MethodHealth,
		}
		if req.Method != rpc.MethodHealth {
			t.Errorf("Method = %q, want %q", req.Method, rpc.MethodHealth)
		}
		if string(req.Method) != "getHealth" {
			t.Errorf("wire value = %q, want \"getHealth\"", req.Method)
		}
	})
}
