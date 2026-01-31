// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetLedgerEntries_EmptyKeys(t *testing.T) {
	client := &Client{SorobanURL: "http://example.invalid", CacheEnabled: false}
	ctx := context.Background()

	got, err := client.GetLedgerEntries(ctx, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(got))
	}
}

func TestGetLedgerEntries_FetchesMultipleKeys(t *testing.T) {
	var reqCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req GetLedgerEntriesRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Method != "getLedgerEntries" {
			http.Error(w, "unexpected method: "+req.Method, http.StatusBadRequest)
			return
		}
		if len(req.Params) != 1 {
			http.Error(w, "expected 1 param", http.StatusBadRequest)
			return
		}

		keysIface, ok := req.Params[0].([]interface{})
		if !ok {
			http.Error(w, "expected params[0] to be array", http.StatusBadRequest)
			return
		}

		resp := GetLedgerEntriesResponse{Jsonrpc: "2.0", ID: 1}
		resp.Result.LatestLedger = 123
		resp.Result.Entries = make([]struct {
			Key                string `json:"key"`
			Xdr                string `json:"xdr"`
			LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
			LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
		}, 0, len(keysIface))

		for i, k := range keysIface {
			ks, _ := k.(string)
			resp.Result.Entries = append(resp.Result.Entries, struct {
				Key                string `json:"key"`
				Xdr                string `json:"xdr"`
				LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
				LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
			}{
				Key:                ks,
				Xdr:                "xdr-" + ks,
				LastModifiedLedger: 1000 + i,
				LiveUntilLedger:    2000 + i,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{SorobanURL: server.URL, CacheEnabled: false}
	keys := []string{"k1", "k2", "k3", "k4", "k5"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := client.GetLedgerEntries(ctx, keys)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != len(keys) {
		t.Fatalf("expected %d entries, got %d", len(keys), len(got))
	}
	for _, k := range keys {
		if got[k] != "xdr-"+k {
			t.Fatalf("expected key %q to map to %q, got %q", k, "xdr-"+k, got[k])
		}
	}

	if atomic.LoadInt32(&reqCount) != 1 {
		t.Fatalf("expected 1 rpc request, got %d", reqCount)
	}
}

func TestGetLedgerEntries_BatchesOver50Keys(t *testing.T) {
	var reqCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req GetLedgerEntriesRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Method != "getLedgerEntries" {
			http.Error(w, "unexpected method: "+req.Method, http.StatusBadRequest)
			return
		}

		keysIface, ok := req.Params[0].([]interface{})
		if !ok {
			http.Error(w, "expected params[0] to be array", http.StatusBadRequest)
			return
		}
		if len(keysIface) > 50 {
			http.Error(w, "batch too large", http.StatusBadRequest)
			return
		}

		resp := GetLedgerEntriesResponse{Jsonrpc: "2.0", ID: 1}
		resp.Result.LatestLedger = 123
		resp.Result.Entries = make([]struct {
			Key                string `json:"key"`
			Xdr                string `json:"xdr"`
			LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
			LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
		}, 0, len(keysIface))

		for i, k := range keysIface {
			ks, _ := k.(string)
			resp.Result.Entries = append(resp.Result.Entries, struct {
				Key                string `json:"key"`
				Xdr                string `json:"xdr"`
				LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
				LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
			}{
				Key:                ks,
				Xdr:                "xdr-" + ks,
				LastModifiedLedger: 1000 + i,
				LiveUntilLedger:    2000 + i,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{SorobanURL: server.URL, CacheEnabled: false}

	keys := make([]string, 75)
	for i := range keys {
		keys[i] = "k" + string(rune('a'+(i%26))) + "-" + string(rune('0'+(i%10)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := client.GetLedgerEntries(ctx, keys)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != len(keys) {
		t.Fatalf("expected %d entries, got %d", len(keys), len(got))
	}
	if atomic.LoadInt32(&reqCount) < 2 {
		t.Fatalf("expected at least 2 rpc requests due to batching, got %d", reqCount)
	}
}
