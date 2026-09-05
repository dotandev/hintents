// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// makeImportTestLedgerKey builds a valid base64 XDR LedgerKey for an account
// with the given index byte, mirroring the rpc package's test helpers.
func makeImportTestLedgerKey(t *testing.T, index int) string {
	t.Helper()
	var raw xdr.Uint256
	raw[31] = byte(index)

	accountID := xdr.AccountId{
		Type:    xdr.PublicKeyTypePublicKeyTypeEd25519,
		Ed25519: &raw,
	}
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: accountID,
		},
	}
	encoded, err := rpc.EncodeLedgerKey(key)
	if err != nil {
		t.Fatalf("EncodeLedgerKey: %v", err)
	}
	return encoded
}

// makeImportTestEntry builds a valid base64 XDR LedgerEntry whose derived key
// matches keyB64, so rpc.VerifyLedgerEntries passes.
func makeImportTestEntry(t *testing.T, keyB64 string) string {
	t.Helper()
	keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	var lk xdr.LedgerKey
	if err := xdr.SafeUnmarshal(keyBytes, &lk); err != nil {
		t.Fatalf("unmarshal key: %v", err)
	}

	var entry xdr.LedgerEntry
	entry.LastModifiedLedgerSeq = 100
	entry.Data = xdr.LedgerEntryData{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.AccountEntry{
			AccountId: lk.Account.AccountId,
			Balance:   1000,
		},
	}
	eb, err := entry.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	return base64.StdEncoding.EncodeToString(eb)
}

// newImportMockServer serves getLedgerEntries responses for the requested keys.
func newImportMockServer(t *testing.T, mu *sync.Mutex, requested *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpc.GetLedgerEntriesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rawKeys, ok := req.Params[0].([]interface{})
		if !ok {
			t.Errorf("unexpected params: %#v", req.Params)
			http.Error(w, "bad params", http.StatusBadRequest)
			return
		}

		mu.Lock()
		for _, raw := range rawKeys {
			if key, ok := raw.(string); ok {
				*requested = append(*requested, key)
			}
		}
		mu.Unlock()

		resp := rpc.GetLedgerEntriesResponse{Jsonrpc: "2.0", ID: 1}
		for _, raw := range rawKeys {
			key, ok := raw.(string)
			if !ok {
				continue
			}
			resp.Result.Entries = append(resp.Result.Entries, rpc.LedgerEntryResult{
				Key: key,
				Xdr: makeImportTestEntry(t, key),
			})
		}
		resp.Result.LatestLedger = 12345

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestImportNetworkState_FetchesAndWritesSnapshot(t *testing.T) {
	var (
		mu        sync.Mutex
		requested []string
	)
	server := newImportMockServer(t, &mu, &requested)
	defer server.Close()

	outPath := filepath.Join(t.TempDir(), "imported", "snapshot.json")

	// Key indices 100-102 are unique to this test: the RPC layer keeps a
	// process-wide cache, so tests must not share key material.
	result, err := ImportNetworkState(context.Background(), ImportConfig{
		LedgerKeys: []string{
			makeImportTestLedgerKey(t, 100),
			makeImportTestLedgerKey(t, 101),
			makeImportTestLedgerKey(t, 102),
		},
		OutputPath: outPath,
		Network:    "testnet",
		RPCURL:     server.URL,
	})
	if err != nil {
		t.Fatalf("ImportNetworkState: %v", err)
	}

	if result.Entries != 3 {
		t.Errorf("expected 3 entries, got %d", result.Entries)
	}
	if result.FetchedKeys != 3 {
		t.Errorf("expected 3 fetched keys, got %d", result.FetchedKeys)
	}
	if result.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}

	// Snapshot file must exist and parse back to the same entries.
	snap, err := snapshot.Load(outPath)
	if err != nil {
		t.Fatalf("snapshot.Load: %v", err)
	}
	if got := len(snap.LedgerEntries); got != 3 {
		t.Errorf("expected 3 ledger entries in file, got %d", got)
	}
	if snap.Fingerprint != result.Fingerprint {
		t.Errorf("fingerprint mismatch: file=%s result=%s", snap.Fingerprint, result.Fingerprint)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requested) != 3 {
		t.Errorf("expected 3 keys requested from RPC, got %d", len(requested))
	}
}

func TestImportNetworkState_MergesWithExistingSnapshot(t *testing.T) {
	var (
		mu        sync.Mutex
		requested []string
	)
	server := newImportMockServer(t, &mu, &requested)
	defer server.Close()

	outPath := filepath.Join(t.TempDir(), "snapshot.json")

	// Seed an existing snapshot with one entry (index 110, unique to this
	// test because the RPC cache is process-wide).
	seedKey := makeImportTestLedgerKey(t, 110)
	seed := snapshot.FromMap(map[string]string{seedKey: makeImportTestEntry(t, seedKey)})
	if err := snapshot.Save(outPath, seed); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	result, err := ImportNetworkState(context.Background(), ImportConfig{
		LedgerKeys: []string{
			makeImportTestLedgerKey(t, 111),
			makeImportTestLedgerKey(t, 112),
		},
		OutputPath: outPath,
		Network:    "testnet",
		RPCURL:     server.URL,
	})
	if err != nil {
		t.Fatalf("ImportNetworkState: %v", err)
	}

	// 1 seeded + 2 fetched = 3 entries.
	if result.Entries != 3 {
		t.Errorf("expected 3 entries after merge, got %d", result.Entries)
	}
	if result.FetchedKeys != 2 {
		t.Errorf("expected 2 newly fetched keys, got %d", result.FetchedKeys)
	}

	snap, err := snapshot.Load(outPath)
	if err != nil {
		t.Fatalf("snapshot.Load: %v", err)
	}
	if got := len(snap.LedgerEntries); got != 3 {
		t.Errorf("expected 3 entries in merged file, got %d", got)
	}
}

func TestImportNetworkState_ValidationErrors(t *testing.T) {
	if _, err := ImportNetworkState(context.Background(), ImportConfig{
		OutputPath: filepath.Join(t.TempDir(), "s.json"),
	}); err == nil {
		t.Error("expected error when no contracts or keys provided")
	}

	if _, err := ImportNetworkState(context.Background(), ImportConfig{
		ContractIDs: []string{"CABC"},
	}); err == nil {
		t.Error("expected error when output path missing")
	}
}

func TestImportNetworkState_ServerErrorPropagates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"boom"}}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	// Use a key index unused by other tests: the RPC layer's process-wide
	// cache would otherwise serve a cached entry from an earlier test and
	// mask the server failure.
	_, err := ImportNetworkState(context.Background(), ImportConfig{
		LedgerKeys: []string{makeImportTestLedgerKey(t, 200)},
		OutputPath: filepath.Join(t.TempDir(), "s.json"),
		Network:    "testnet",
		RPCURL:     server.URL,
		Timeout:    5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error from failing RPC server")
	}
}
