// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package rpc — CI log-output tests for issue #856
//
// These tests enforce the unified logging policy across Horizon and Soroban
// paths:
//
//	Pre-request  (attempt level): DEBUG
//	Success      (attempt level): DEBUG
//	Failure      (attempt level): ERROR
//	Retry / failover:             WARN
//
// The HTTP-transport middleware (NewLoggingMiddleware) deliberately remains at
// INFO for success because it is user-opt-in per-request tracing; that
// behaviour is covered separately in middleware_test.go.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// logLevel extracts the "level" field from a JSON slog line.  Returns ""
// when the line is not valid JSON or has no level field.
func logLevel(line []byte) string {
	var m map[string]any
	if json.Unmarshal(line, &m) != nil {
		return ""
	}
	s, _ := m["level"].(string)
	return s
}

// logMsg extracts the "msg" field from a JSON slog line.
func logMsg(line []byte) string {
	var m map[string]any
	if json.Unmarshal(line, &m) != nil {
		return ""
	}
	s, _ := m["msg"].(string)
	return s
}

// splitLines splits buf into non-empty lines.
func splitLines(buf *bytes.Buffer) [][]byte {
	var out [][]byte
	for _, l := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		if len(l) > 0 {
			out = append(out, l)
		}
	}
	return out
}

// findLogEntry returns the level of the first log line whose msg equals want,
// or ("", false) if no such line exists.
func findLogEntry(buf *bytes.Buffer, want string) (level string, found bool) {
	for _, line := range splitLines(buf) {
		if logMsg(line) == want {
			return logLevel(line), true
		}
	}
	return "", false
}

// noInfoInBuf asserts that no log line in buf carries level INFO.
// Useful to confirm attempt-level success logs have been downgraded to DEBUG.
func noInfoInBuf(t *testing.T, buf *bytes.Buffer, ctx string) {
	t.Helper()
	for _, line := range splitLines(buf) {
		if logLevel(line) == "INFO" && logMsg(line) != "http request completed" {
			msg := logMsg(line)
			t.Errorf("%s: unexpected INFO log at attempt level: msg=%q", ctx, msg)
		}
	}
}

// newSorobanClient creates a minimal *Client pointing both Horizon and Soroban
// at url, with a single AltURL entry so failover paths initialise correctly.
func newSorobanClient(url string) *Client {
	return &Client{
		Horizon:    &mockHorizonClient{},
		HorizonURL: url,
		SorobanURL: url,
		Network:    Testnet,
		AltURLs:    []string{url},
	}
}

// ---------------------------------------------------------------------------
// Horizon path — GetTransaction
// ---------------------------------------------------------------------------

// TestLoggingPolicy_HorizonGetTransaction_SuccessIsDebug ensures that a
// successful Horizon GetTransaction attempt emits "Transaction fetched" at
// DEBUG, not INFO.
func TestLoggingPolicy_HorizonGetTransaction_SuccessIsDebug(t *testing.T) {
	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	mock := &mockHorizonClient{
		TransactionDetailFunc: func(_ string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{
				EnvelopeXdr:   "env",
				ResultXdr:     "res",
				ResultMetaXdr: "meta",
			}, nil
		},
	}
	c := newTestClient(mock)

	_, err := c.GetTransaction(context.Background(), "abc123")
	require.NoError(t, err)

	level, found := findLogEntry(&buf, "Transaction fetched")
	require.True(t, found, "expected 'Transaction fetched' log entry")
	assert.Equal(t, "DEBUG", level,
		"[policy #856] Horizon GetTransaction success must be logged at DEBUG, not INFO")
}

// TestLoggingPolicy_HorizonGetTransaction_FailureIsError ensures that a failed
// Horizon GetTransaction attempt emits "Failed to fetch transaction" at ERROR.
func TestLoggingPolicy_HorizonGetTransaction_FailureIsError(t *testing.T) {
	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	mock := &mockHorizonClient{
		TransactionDetailFunc: func(_ string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, errors.New("not found")
		},
	}
	c := newTestClient(mock)

	_, err := c.GetTransaction(context.Background(), "badhash")
	assert.Error(t, err)

	level, found := findLogEntry(&buf, "Failed to fetch transaction")
	require.True(t, found, "expected 'Failed to fetch transaction' log entry")
	assert.Equal(t, "ERROR", level,
		"[policy #856] Horizon GetTransaction failure must be logged at ERROR")
}

// ---------------------------------------------------------------------------
// Soroban path — GetHealth
// ---------------------------------------------------------------------------

// TestLoggingPolicy_SorobanGetHealth_SuccessIsDebug ensures that a successful
// Soroban getHealth attempt emits its success log at DEBUG.
func TestLoggingPolicy_SorobanGetHealth_SuccessIsDebug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"status":"healthy","latestLedger":100,"oldestLedger":1,"ledgerRetentionWindow":50}}`)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	c := newSorobanClient(srv.URL)
	resp, err := c.GetHealth(context.Background())
	require.NoError(t, err)
	require.NotNil(t, resp)

	level, found := findLogEntry(&buf, "Soroban RPC health check successful")
	require.True(t, found, "expected 'Soroban RPC health check successful' log entry")
	assert.Equal(t, "DEBUG", level,
		"[policy #856] Soroban getHealth success must be logged at DEBUG, not INFO")
}

// TestLoggingPolicy_SorobanGetHealth_RPCErrorIsError ensures that a JSON-RPC
// error from the Soroban node is logged at ERROR.
func TestLoggingPolicy_SorobanGetHealth_RPCErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"node unhealthy"}}`)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	c := newSorobanClient(srv.URL)
	_, err := c.GetHealth(context.Background())
	assert.Error(t, err)

	level, found := findLogEntry(&buf, "Soroban getHealth RPC error")
	require.True(t, found, "expected 'Soroban getHealth RPC error' log entry")
	assert.Equal(t, "ERROR", level,
		"[policy #856] Soroban getHealth RPC error must be logged at ERROR")
}

// TestLoggingPolicy_SorobanGetHealth_TransportErrorIsError ensures that a
// transport-level failure (no server) is also logged at ERROR.
func TestLoggingPolicy_SorobanGetHealth_TransportErrorIsError(t *testing.T) {
	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	// Point at a server that immediately closes the connection.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	c := newSorobanClient(srv.URL)
	_, err := c.GetHealth(context.Background())
	assert.Error(t, err)

	// We only verify that no INFO appears at attempt level (policy guard).
	noInfoInBuf(t, &buf, "getHealth transport error")
}

// ---------------------------------------------------------------------------
// Soroban path — SimulateTransaction
// ---------------------------------------------------------------------------

// TestLoggingPolicy_SorobanSimulate_SuccessIsDebug ensures that a successful
// simulateTransaction attempt emits its success log at DEBUG.
func TestLoggingPolicy_SorobanSimulate_SuccessIsDebug(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"minResourceFee":"100","transactionData":""}}`)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	c := newSorobanClient(srv.URL)
	resp, err := c.SimulateTransaction(context.Background(), "dGVzdA==")
	require.NoError(t, err)
	require.NotNil(t, resp)

	level, found := findLogEntry(&buf, "Soroban simulateTransaction succeeded")
	require.True(t, found, "expected 'Soroban simulateTransaction succeeded' log entry")
	assert.Equal(t, "DEBUG", level,
		"[policy #856] Soroban simulateTransaction success must be logged at DEBUG")
}

// TestLoggingPolicy_SorobanSimulate_RPCErrorIsError ensures that a JSON-RPC
// error from simulateTransaction is logged at ERROR.
func TestLoggingPolicy_SorobanSimulate_RPCErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"simulation failed"}}`)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	c := newSorobanClient(srv.URL)
	_, err := c.SimulateTransaction(context.Background(), "dGVzdA==")
	assert.Error(t, err)

	level, found := findLogEntry(&buf, "Soroban simulateTransaction RPC error")
	require.True(t, found, "expected 'Soroban simulateTransaction RPC error' log entry")
	assert.Equal(t, "ERROR", level,
		"[policy #856] Soroban simulateTransaction RPC error must be logged at ERROR")
}

// TestLoggingPolicy_SorobanSimulate_TransportErrorIsError ensures that a
// transport-level failure from simulateTransaction is logged at ERROR.
func TestLoggingPolicy_SorobanSimulate_TransportErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	c := newSorobanClient(srv.URL)
	_, err := c.SimulateTransaction(context.Background(), "dGVzdA==")
	assert.Error(t, err)

	level, found := findLogEntry(&buf, "Soroban simulateTransaction request failed")
	require.True(t, found, "expected 'Soroban simulateTransaction request failed' log entry")
	assert.Equal(t, "ERROR", level,
		"[policy #856] Soroban simulateTransaction transport error must be logged at ERROR")
}

// ---------------------------------------------------------------------------
// Soroban path — GetLedgerEntries
// ---------------------------------------------------------------------------

// TestLoggingPolicy_SorobanGetLedgerEntries_RPCErrorIsError ensures that a
// JSON-RPC error returned by getLedgerEntries is logged at ERROR.
func TestLoggingPolicy_SorobanGetLedgerEntries_RPCErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32002,"message":"entry not found"}}`)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	c := newSorobanClient(srv.URL)
	_, err := c.GetLedgerEntries(context.Background(), []string{"AAAA"})
	assert.Error(t, err)

	level, found := findLogEntry(&buf, "Soroban getLedgerEntries RPC error")
	require.True(t, found, "expected 'Soroban getLedgerEntries RPC error' log entry")
	assert.Equal(t, "ERROR", level,
		"[policy #856] Soroban getLedgerEntries RPC error must be logged at ERROR")
}

// TestLoggingPolicy_SorobanGetLedgerEntries_TransportErrorIsError ensures that
// a transport-level failure from getLedgerEntries is logged at ERROR.
func TestLoggingPolicy_SorobanGetLedgerEntries_TransportErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	c := newSorobanClient(srv.URL)
	_, err := c.GetLedgerEntries(context.Background(), []string{"AAAA"})
	assert.Error(t, err)

	level, found := findLogEntry(&buf, "Soroban getLedgerEntries request failed")
	require.True(t, found, "expected 'Soroban getLedgerEntries request failed' log entry")
	assert.Equal(t, "ERROR", level,
		"[policy #856] Soroban getLedgerEntries transport error must be logged at ERROR")
}

// TestLoggingPolicy_NoAttemptLevelINFOOnSuccess confirms that after the policy
// standardisation, attempt-level functions never emit INFO for success.
// It exercises GetHealth (Soroban) and GetTransaction (Horizon).
func TestLoggingPolicy_NoAttemptLevelINFOOnSuccess(t *testing.T) {
	// --- Soroban GetHealth ---
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"status":"healthy","latestLedger":100,"oldestLedger":1,"ledgerRetentionWindow":50}}`)
	}))
	defer healthSrv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	c := newSorobanClient(healthSrv.URL)
	_, err := c.GetHealth(context.Background())
	require.NoError(t, err)

	// Expect no INFO-level entries other than those from the transport middleware
	// (which is NOT enabled here since we didn't pass WithLoggingEnabled(true)).
	for _, line := range splitLines(&buf) {
		lvl := logLevel(line)
		msg := logMsg(line)
		assert.NotEqual(t, "INFO", lvl,
			"[policy #856] attempt-level success must be DEBUG, not INFO (msg=%q)", msg)
	}

	// --- Horizon GetTransaction ---
	buf.Reset()

	mock := &mockHorizonClient{
		TransactionDetailFunc: func(_ string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{
				EnvelopeXdr:   "env",
				ResultXdr:     "res",
				ResultMetaXdr: "meta",
			}, nil
		},
	}
	hc := newTestClient(mock)
	_, err = hc.GetTransaction(context.Background(), "abc123")
	require.NoError(t, err)

	for _, line := range splitLines(&buf) {
		lvl := logLevel(line)
		msg := logMsg(line)
		assert.NotEqual(t, "INFO", lvl,
			"[policy #856] Horizon attempt-level success must be DEBUG, not INFO (msg=%q)", msg)
	}
}

// ---------------------------------------------------------------------------
// Middleware regression — success stays at INFO
// ---------------------------------------------------------------------------

// TestLoggingPolicy_Middleware_SuccessRemainsINFO ensures the HTTP transport
// middleware (user-opt-in) still logs successful requests at INFO.
func TestLoggingPolicy_Middleware_SuccessRemainsINFO(t *testing.T) {
	body := `{"jsonrpc":"2.0","result":{"status":"healthy","latestLedger":1,"oldestLedger":1,"ledgerRetentionWindow":1},"id":1}`
	srv := captureServer(t, http.StatusOK, body)
	defer srv.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	client, err := NewClient(
		WithHorizonURL(srv.URL),
		WithSorobanURL(srv.URL),
		WithLoggingEnabled(true),
	)
	require.NoError(t, err)

	_, _ = client.GetHealth(context.Background())

	level, found := findLogEntry(&buf, "http request completed")
	require.True(t, found, "expected 'http request completed' log from middleware")
	assert.Equal(t, "INFO", level,
		"[policy #856] HTTP transport middleware success must remain at INFO (user-opt-in tracing)")
}
