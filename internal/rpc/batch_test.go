// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// chunkKeys unit tests
// =============================================================================

func TestChunkKeys_Empty(t *testing.T) {
	chunks := chunkKeys([]string{}, batchSize)
	assert.Empty(t, chunks)
}

func TestChunkKeys_BelowBatchSize(t *testing.T) {
	keys := makeKeys(50)
	chunks := chunkKeys(keys, batchSize)
	require.Len(t, chunks, 1)
	assert.Equal(t, keys, chunks[0])
}

func TestChunkKeys_ExactBatchSize(t *testing.T) {
	keys := makeKeys(batchSize)
	chunks := chunkKeys(keys, batchSize)
	require.Len(t, chunks, 1)
	assert.Len(t, chunks[0], batchSize)
}

func TestChunkKeys_MultipleBatches(t *testing.T) {
	keys := makeKeys(250)
	chunks := chunkKeys(keys, batchSize)
	// 250 keys → 3 chunks: 100, 100, 50
	require.Len(t, chunks, 3)
	assert.Len(t, chunks[0], 100)
	assert.Len(t, chunks[1], 100)
	assert.Len(t, chunks[2], 50)
}

func TestChunkKeys_PreservesOrder(t *testing.T) {
	keys := makeKeys(150)
	chunks := chunkKeys(keys, batchSize)
	require.Len(t, chunks, 2)

	var got []string
	for _, c := range chunks {
		got = append(got, c...)
	}
	assert.Equal(t, keys, got)
}

func TestChunkKeys_NegativeSize_FallsBackToDefault(t *testing.T) {
	keys := makeKeys(10)
	chunks := chunkKeys(keys, -1)
	// -1 falls back to batchSize (100), so 10 keys → 1 chunk
	require.Len(t, chunks, 1)
	assert.Equal(t, keys, chunks[0])
}

// =============================================================================
// BatchGetLedgerEntries integration-style tests (mock HTTP server)
// =============================================================================

// TestBatchGetLedgerEntries_EmptyInput returns immediately without any HTTP call.
func TestBatchGetLedgerEntries_EmptyInput(t *testing.T) {
	c := &Client{SorobanURL: "http://unused"}
	result, err := c.BatchGetLedgerEntries(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestBatchGetLedgerEntries_SmallFootprint delegates to GetLedgerEntries directly.
func TestBatchGetLedgerEntries_SmallFootprint(t *testing.T) {
	keys := makeKeys(10)
	srv := newMockSorobanServer(t, keys)
	defer srv.Close()

	c := newBatchTestClient(t, srv.URL)
	result, err := c.BatchGetLedgerEntries(context.Background(), keys)
	require.NoError(t, err)
	assert.Len(t, result, 10)
}

// TestBatchGetLedgerEntries_LargeFootprint batches keys over the threshold.
func TestBatchGetLedgerEntries_LargeFootprint(t *testing.T) {
	keys := makeKeys(250)
	srv := newMockSorobanServer(t, keys)
	defer srv.Close()

	c := newBatchTestClient(t, srv.URL)
	result, err := c.BatchGetLedgerEntries(context.Background(), keys)
	require.NoError(t, err)
	assert.Len(t, result, 250)
}

// TestBatchGetLedgerEntries_ExactThreshold uses exactly largFootprintThreshold keys,
// which must take the single-request path.
func TestBatchGetLedgerEntries_ExactThreshold(t *testing.T) {
	keys := makeKeys(largFootprintThreshold)
	srv := newMockSorobanServer(t, keys)
	defer srv.Close()

	c := newBatchTestClient(t, srv.URL)
	result, err := c.BatchGetLedgerEntries(context.Background(), keys)
	require.NoError(t, err)
	assert.Len(t, result, largFootprintThreshold)
}

// TestBatchGetLedgerEntries_OneBeyondThreshold forces the batching path.
func TestBatchGetLedgerEntries_OneBeyondThreshold(t *testing.T) {
	keys := makeKeys(largFootprintThreshold + 1)
	srv := newMockSorobanServer(t, keys)
	defer srv.Close()

	c := newBatchTestClient(t, srv.URL)
	result, err := c.BatchGetLedgerEntries(context.Background(), keys)
	require.NoError(t, err)
	assert.Len(t, result, largFootprintThreshold+1)
}

// TestBatchGetLedgerEntries_ServerError propagates an error from a failing batch.
func TestBatchGetLedgerEntries_ServerError(t *testing.T) {
	keys := makeKeys(150)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"server error"}}`))
	}))
	defer srv.Close()

	c := newBatchTestClient(t, srv.URL)
	_, err := c.BatchGetLedgerEntries(context.Background(), keys)
	assert.Error(t, err)
}

// TestBatchGetLedgerEntries_ContextCancelled respects context cancellation.
func TestBatchGetLedgerEntries_ContextCancelled(t *testing.T) {
	keys := makeKeys(200)
	srv := newMockSorobanServer(t, keys)
	defer srv.Close()

	c := newBatchTestClient(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.BatchGetLedgerEntries(ctx, keys)
	assert.Error(t, err)
}

// =============================================================================
// Helpers
// =============================================================================

// makeKeys generates n valid base64-encoded XDR LedgerKey strings using
// deterministic ContractCode keys seeded by index.
func makeKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		var hash xdr.Hash
		// Fill hash bytes deterministically from i so every key is unique.
		hash[0] = byte(i & 0xff)
		hash[1] = byte((i >> 8) & 0xff)
		hash[2] = byte((i >> 16) & 0xff)
		// remaining bytes stay zero — enough for uniqueness across reasonable n

		key := xdr.LedgerKey{
			Type:         xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.LedgerKeyContractCode{Hash: hash},
		}
		b, err := key.MarshalBinary()
		if err != nil {
			panic(fmt.Sprintf("makeKeys: failed to marshal key %d: %v", i, err))
		}
		keys[i] = base64.StdEncoding.EncodeToString(b)
	}
	return keys
}

// newMockSorobanServer creates an httptest.Server that echoes back entries for
// every key present in the known set, matching the getLedgerEntries RPC shape.
func newMockSorobanServer(t *testing.T, known []string) *httptest.Server {
	t.Helper()
	index := make(map[string]struct{}, len(known))
	for _, k := range known {
		index[k] = struct{}{}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params [][]string `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		type entry struct {
			Key                string `json:"key"`
			Xdr                string `json:"xdr"`
			LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
			LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
		}

		var entries []entry
		if len(req.Params) > 0 {
			for _, k := range req.Params[0] {
				if _, ok := index[k]; ok {
					entries = append(entries, entry{
						Key:                k,
						Xdr:                "xdr-" + k,
						LastModifiedLedger: 100,
						LiveUntilLedger:    200,
					})
				}
			}
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"entries":      entries,
				"latestLedger": 999,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// newBatchTestClient builds a minimal Client pointed at the given Soroban URL.
func newBatchTestClient(t *testing.T, sorobanURL string) *Client {
	t.Helper()
	return &Client{
		Network:      Testnet,
		SorobanURL:   sorobanURL,
		HorizonURL:   sorobanURL,
		AltURLs:      []string{sorobanURL},
		CacheEnabled: false,
		httpClient:   &http.Client{},
		failures:     make(map[string]int),
		lastFailure:  make(map[string]time.Time),
	}
}
