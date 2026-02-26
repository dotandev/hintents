// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newRetryHTTPClient() *http.Client {
	cfg := RetryConfig{
		MaxRetries:         2,
		InitialBackoff:     1 * time.Millisecond,
		MaxBackoff:         2 * time.Millisecond,
		JitterFraction:     0,
		StatusCodesToRetry: []int{http.StatusTooManyRequests},
	}
	transport := NewRetryTransport(cfg, http.DefaultTransport)
	return &http.Client{Transport: transport}
}

func TestSimulateTransactionRetriesOnRateLimit(t *testing.T) {
	var calls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		resp := SimulateTransactionResponse{
			Jsonrpc: "2.0",
			ID:      1,
		}
		resp.Result.MinResourceFee = "1"
		resp.Result.TransactionData = "AAAA"
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		WithHTTPClient(newRetryHTTPClient()),
		WithCacheEnabled(false),
	)
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}

	resp, err := client.SimulateTransaction(context.Background(), "AAAA")
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}

	if resp.Result.MinResourceFee != "1" {
		t.Fatalf("unexpected response: %+v", resp.Result)
	}

	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", atomic.LoadInt32(&calls))
	}
}

func TestGetLedgerEntriesRetriesOnRateLimit(t *testing.T) {
	// Use a valid base64-encoded XDR LedgerKey so VerifyLedgerEntries passes.
	validKeys := makeKeys(1)
	testKey := validKeys[0]

	var calls int32
	validKey := createTestLedgerKey(t, 42)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		resp := GetLedgerEntriesResponse{
			Jsonrpc: "2.0",
			ID:      1,
		}
		resp.Result.Entries = []struct {
			Key                string `json:"key"`
			Xdr                string `json:"xdr"`
			LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
			LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
		}{{
			Key: testKey,
		resp.Result.Entries = []LedgerEntryResult{{
			Key: validKey,
			Xdr: "BBB",
		}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		WithHTTPClient(newRetryHTTPClient()),
		WithCacheEnabled(false),
	)
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}

	entries, err := client.GetLedgerEntries(context.Background(), []string{testKey})
	entries, err := client.GetLedgerEntries(context.Background(), []string{key})
	entries, err := client.GetLedgerEntries(context.Background(), []string{validKey})
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}

	if entries[testKey] != "BBB" {
	if entries[key] != "BBB" {
	if entries[validKey] != "BBB" {
		t.Fatalf("unexpected ledger entry: %v", entries)
	}

	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", atomic.LoadInt32(&calls))
	}
}