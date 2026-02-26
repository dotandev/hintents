// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
)

// ==================== Compute-Heavy Benchmarks ====================
// These benchmarks measure CPU and memory overhead without network I/O

// BenchmarkJSONRPCMarshal benchmarks JSON-RPC request marshaling
func BenchmarkJSONRPCMarshal(b *testing.B) {
	tests := []struct {
		name    string
		numKeys int
	}{
		{"Single", 1},
		{"Small", 10},
		{"Medium", 50},
		{"Large", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			keys := make([]string, tt.numKeys)
			for i := 0; i < tt.numKeys; i++ {
				keys[i] = strings.Repeat("a", 64) // 64-char base64 keys
			}

			req := GetLedgerEntriesRequest{
				Jsonrpc: "2.0",
				ID:      1,
				Method:  "getLedgerEntries",
				Params:  []interface{}{keys},
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := json.Marshal(req)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkJSONRPCUnmarshal benchmarks JSON-RPC response unmarshaling
func BenchmarkJSONRPCUnmarshal(b *testing.B) {
	tests := []struct {
		name       string
		numEntries int
	}{
		{"Single", 1},
		{"Small", 10},
		{"Medium", 50},
		{"Large", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create mock response
			resp := GetLedgerEntriesResponse{
				Jsonrpc: "2.0",
				ID:      1,
			}
			resp.Result.LatestLedger = 12345
			resp.Result.Entries = make([]LedgerEntryResult, tt.numEntries)

			for i := 0; i < tt.numEntries; i++ {
				resp.Result.Entries[i].Key = strings.Repeat("k", 64)
				resp.Result.Entries[i].Xdr = strings.Repeat("x", 128)
				resp.Result.Entries[i].LastModifiedLedger = 1000 + i
				resp.Result.Entries[i].LiveUntilLedger = 2000 + i
			}

			respBytes, _ := json.Marshal(resp)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var r GetLedgerEntriesResponse
				err := json.Unmarshal(respBytes, &r)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkParseTransactionResponse benchmarks transaction response parsing
func BenchmarkParseTransactionResponse(b *testing.B) {
	tx := hProtocol.Transaction{
		EnvelopeXdr:   strings.Repeat("e", 512),
		ResultXdr:     strings.Repeat("r", 256),
		ResultMetaXdr: strings.Repeat("m", 1024),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp := ParseTransactionResponse(tx)
		if resp == nil {
			b.Fatal("nil response")
		}
	}
}

// BenchmarkLedgerHeaderParsing benchmarks ledger header parsing
func BenchmarkLedgerHeaderParsing(b *testing.B) {
	txCount := int32(10)
	failedTxCount := int32(2)
	ledger := hProtocol.Ledger{
		Hash:                       "test-hash",
		Sequence:                   12345,
		SuccessfulTransactionCount: txCount,
		FailedTransactionCount:     &failedTxCount,
		ProtocolVersion:            20,
		BaseFee:                    100,
		BaseReserve:                5000000,
		MaxTxSetSize:               1000,
		HeaderXDR:                  strings.Repeat("h", 256),
		ClosedAt:                   time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp := FromHorizonLedger(ledger)
		if resp == nil {
			b.Fatal("nil response")
		}
	}
}

// BenchmarkLargeJSONParsing benchmarks parsing large JSON responses
func BenchmarkLargeJSONParsing(b *testing.B) {
	// Simulate a 1MB JSON response
	resp := GetLedgerEntriesResponse{
		Jsonrpc: "2.0",
		ID:      1,
	}
	resp.Result.LatestLedger = 99999
	resp.Result.Entries = make([]LedgerEntryResult, 500)

	for i := 0; i < 500; i++ {
		resp.Result.Entries[i].Key = strings.Repeat("k", 100)
		resp.Result.Entries[i].Xdr = strings.Repeat("x", 2000) // Large XDR
		resp.Result.Entries[i].LastModifiedLedger = 1000 + i
		resp.Result.Entries[i].LiveUntilLedger = 2000 + i
	}

	respBytes, _ := json.Marshal(resp)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var r GetLedgerEntriesResponse
		err := json.Unmarshal(respBytes, &r)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkXDRExtraction benchmarks XDR field extraction
func BenchmarkXDRExtraction(b *testing.B) {
	resp := &TransactionResponse{
		EnvelopeXdr:   strings.Repeat("e", 512),
		ResultXdr:     strings.Repeat("r", 256),
		ResultMetaXdr: strings.Repeat("m", 1024),
	}

	b.Run("EnvelopeXdr", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			xdr := ExtractEnvelopeXdr(resp)
			if xdr == "" {
				b.Fatal("empty xdr")
			}
		}
	})

	b.Run("ResultXdr", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			xdr := ExtractResultXdr(resp)
			if xdr == "" {
				b.Fatal("empty xdr")
			}
		}
	})

	b.Run("ResultMetaXdr", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			xdr := ExtractResultMetaXdr(resp)
			if xdr == "" {
				b.Fatal("empty xdr")
			}
		}
	})
}

// ==================== Network-Heavy Benchmarks ====================
// These benchmarks measure RPC call overhead with mock servers

// BenchmarkGetLedgerEntries benchmarks the GetLedgerEntries RPC call
func BenchmarkGetLedgerEntries(b *testing.B) {
	tests := []struct {
		name    string
		numKeys int
	}{
		{"Single", 1},
		{"Small", 10},
		{"Medium", 50},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create mock Soroban RPC server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Read and validate request
				body, _ := io.ReadAll(r.Body)
				var req GetLedgerEntriesRequest
				json.Unmarshal(body, &req)

				// Create response with matching number of entries
				resp := GetLedgerEntriesResponse{
					Jsonrpc: "2.0",
					ID:      1,
				}
				resp.Result.LatestLedger = 12345
				resp.Result.Entries = make([]LedgerEntryResult, len(req.Params[0].([]interface{})))

				for i := range resp.Result.Entries {
					resp.Result.Entries[i].Key = strings.Repeat("k", 64)
					resp.Result.Entries[i].Xdr = strings.Repeat("x", 128)
					resp.Result.Entries[i].LastModifiedLedger = 1000 + i
					resp.Result.Entries[i].LiveUntilLedger = 2000 + i
				}

				json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			client := &Client{
				SorobanURL: server.URL,
			}

			keys := make([]string, tt.numKeys)
			for i := 0; i < tt.numKeys; i++ {
				keys[i] = strings.Repeat("a", 64)
			}

			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := client.GetLedgerEntries(ctx, keys)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkGetTransaction benchmarks the GetTransaction call with mock
func BenchmarkGetTransaction(b *testing.B) {
	mockTx := hProtocol.Transaction{
		EnvelopeXdr:   strings.Repeat("e", 512),
		ResultXdr:     strings.Repeat("r", 256),
		ResultMetaXdr: strings.Repeat("m", 1024),
	}

	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return mockTx, nil
		},
	}

	client := &Client{
		Horizon: mock,
		Network: Testnet,
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := client.GetTransaction(ctx, "test-hash")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetLedgerHeader benchmarks ledger header fetching
func BenchmarkGetLedgerHeader(b *testing.B) {
	txCount := int32(10)
	failedTxCount := int32(2)
	mockLedger := hProtocol.Ledger{
		Hash:                       "test-hash",
		Sequence:                   12345,
		SuccessfulTransactionCount: txCount,
		FailedTransactionCount:     &failedTxCount,
		ProtocolVersion:            20,
		BaseFee:                    100,
		BaseReserve:                5000000,
		MaxTxSetSize:               1000,
		HeaderXDR:                  strings.Repeat("h", 256),
		ClosedAt:                   time.Now(),
	}

	mock := &mockHorizonClient{
		LedgerDetailFunc: func(sequence uint32) (hProtocol.Ledger, error) {
			return mockLedger, nil
		},
	}

	client := &Client{
		Horizon: mock,
		Network: Testnet,
		Config:  TestnetConfig,
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := client.GetLedgerHeader(ctx, 12345)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHTTPRoundTrip benchmarks HTTP round-trip with auth
func BenchmarkHTTPRoundTrip(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	transport := &authTransport{
		token:     "test-token",
		transport: http.DefaultTransport,
	}

	httpClient := &http.Client{Transport: transport}

	b.Run("WithAuth", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := httpClient.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})

	b.Run("WithoutAuth", func(b *testing.B) {
		defaultClient := http.DefaultClient
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := defaultClient.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

// BenchmarkJSONRPCRoundTrip benchmarks full JSON-RPC round-trip
func BenchmarkJSONRPCRoundTrip(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req GetLedgerEntriesRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := GetLedgerEntriesResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
		}
		resp.Result.LatestLedger = 12345
		resp.Result.Entries = make([]LedgerEntryResult, 1)
		resp.Result.Entries[0].Key = "test-key"
		resp.Result.Entries[0].Xdr = "test-xdr"

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	reqBody := GetLedgerEntriesRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "getLedgerEntries",
		Params:  []interface{}{[]string{"key1"}},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bodyBytes, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", server.URL, bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		respBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var rpcResp GetLedgerEntriesResponse
		json.Unmarshal(respBytes, &rpcResp)
	}
}
