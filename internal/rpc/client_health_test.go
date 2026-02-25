// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHealth_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		var req GetHealthRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "getHealth", req.Method)

		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Result: struct {
				Status                string `json:"status"`
				LatestLedger          uint32 `json:"latestLedger"`
				OldestLedger          uint32 `json:"oldestLedger"`
				LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
			}{
				Status:                "healthy",
				LatestLedger:          100,
				OldestLedger:          1,
				LedgerRetentionWindow: 99,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		SorobanURL: server.URL,
		AltURLs:    []string{server.URL},
	}

	resp, err := client.GetHealth(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Result.Status)
	assert.Equal(t, uint32(100), resp.Result.LatestLedger)
}

func TestGetHealth_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{
				Code:    -32601,
				Message: "Method not found",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		SorobanURL: server.URL,
		AltURLs:    []string{server.URL},
	}

	resp, err := client.GetHealth(context.Background())
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Method not found")
}

func TestGetHealth_Failover(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Result: struct {
				Status                string `json:"status"`
				LatestLedger          uint32 `json:"latestLedger"`
				OldestLedger          uint32 `json:"oldestLedger"`
				LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
			}{
				Status: "healthy",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server2.Close()

	client, _ := NewClient(
		WithNetwork(Testnet),
		WithAltURLs([]string{server1.URL, server2.URL}),
	)
	// Manually set SorobanURL to server1.URL for the first attempt
	client.SorobanURL = server1.URL

	resp, err := client.GetHealth(context.Background())
	assert.NoError(t, err)
	if assert.NotNil(t, resp) && assert.NotNil(t, resp.Result) {
		assert.Equal(t, "healthy", resp.Result.Status)
	}
}
