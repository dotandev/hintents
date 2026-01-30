// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpc

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockServer_BasicSetup(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/abc123": {
			StatusCode: http.StatusOK,
			Body: MockTransactionResponse{
				Hash:          "abc123",
				EnvelopeXdr:   "envelope-xdr",
				ResultXdr:     "result-xdr",
				ResultMetaXdr: "meta-xdr",
			},
		},
	}

	server := NewMockServer(routes)
	defer server.Close()

	assert.NotEmpty(t, server.URL())
	resp, err := http.Get(server.URL() + "/transactions/abc123")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var txResp MockTransactionResponse
	err = json.NewDecoder(resp.Body).Decode(&txResp)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", txResp.Hash)
	assert.Equal(t, "envelope-xdr", txResp.EnvelopeXdr)
}

func TestMockServer_NotFound(t *testing.T) {
	server := NewMockServer(map[string]MockRoute{})
	defer server.Close()

	resp, err := http.Get(server.URL() + "/transactions/unknown")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	assert.NoError(t, err)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.Detail, "endpoint not found")
}

func TestMockServer_RateLimitError(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/limited": RateLimitRoute(),
	}

	server := NewMockServer(routes)
	defer server.Close()

	resp, err := http.Get(server.URL() + "/transactions/limited")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// Check headers
	assert.Equal(t, "60", resp.Header.Get("Retry-After"))

	var errResp ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	assert.NoError(t, err)
	assert.Equal(t, "rate_limit_exceeded", errResp.Status)
}

func TestMockServer_ServerError(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/error": ServerErrorRoute(),
	}

	server := NewMockServer(routes)
	defer server.Close()

	resp, err := http.Get(server.URL() + "/transactions/error")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var errResp ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	assert.NoError(t, err)
	assert.Equal(t, "server_error", errResp.Status)
}

func TestMockServer_CustomErrorRoute(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/forbidden": ErrorRoute(http.StatusForbidden, "access denied"),
	}

	server := NewMockServer(routes)
	defer server.Close()

	resp, err := http.Get(server.URL() + "/transactions/forbidden")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	var errResp ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	assert.NoError(t, err)
	assert.Equal(t, "access denied", errResp.Detail)
}

func TestMockServer_SuccessRoute(t *testing.T) {
	txResp := MockTransactionResponse{
		ID:            "abc123",
		Hash:          "abc123",
		EnvelopeXdr:   "envelope",
		ResultXdr:     "result",
		ResultMetaXdr: "meta",
	}

	routes := map[string]MockRoute{
		"/transactions/abc123": SuccessRoute(txResp),
	}

	server := NewMockServer(routes)
	defer server.Close()

	resp, err := http.Get(server.URL() + "/transactions/abc123")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respData MockTransactionResponse
	err = json.NewDecoder(resp.Body).Decode(&respData)
	assert.NoError(t, err)
	assert.Equal(t, txResp.Hash, respData.Hash)
}

func TestMockServer_AddRoute(t *testing.T) {
	server := NewMockServer(map[string]MockRoute{})
	defer server.Close()

	// Initially, endpoint should not exist
	resp, err := http.Get(server.URL() + "/accounts/test")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Add a new route
	accountResp := MockAccountResponse{
		ID:        "test",
		AccountID: "GTEST...",
		Balance:   "1000.0000000",
		Sequence:  "1",
	}
	server.AddRoute("/accounts/test", SuccessRoute(accountResp))

	// Now it should work
	resp, err = http.Get(server.URL() + "/accounts/test")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respData MockAccountResponse
	err = json.NewDecoder(resp.Body).Decode(&respData)
	assert.NoError(t, err)
	assert.Equal(t, "test", respData.ID)
}

func TestMockServer_RemoveRoute(t *testing.T) {
	routes := map[string]MockRoute{
		"/accounts/test": SuccessRoute(MockAccountResponse{ID: "test"}),
	}

	server := NewMockServer(routes)
	defer server.Close()

	// Route should exist
	resp, err := http.Get(server.URL() + "/accounts/test")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Remove the route
	server.RemoveRoute("/accounts/test")

	// Now it should return 404
	resp, err = http.Get(server.URL() + "/accounts/test")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestMockServer_CallCounting(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/abc123": SuccessRoute(MockTransactionResponse{Hash: "abc123"}),
	}

	server := NewMockServer(routes)
	defer server.Close()

	// Make multiple requests
	for i := 0; i < 5; i++ {
		resp, err := http.Get(server.URL() + "/transactions/abc123")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Verify call count
	assert.Equal(t, 5, server.CallCount("/transactions/abc123"))

	// Make request to another endpoint
	resp, err := http.Get(server.URL() + "/notfound")
	assert.NoError(t, err)
	resp.Body.Close()

	// Verify separate count
	assert.Equal(t, 1, server.CallCount("/notfound"))
}

func TestMockServer_ResetCallCounts(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/abc123": SuccessRoute(MockTransactionResponse{Hash: "abc123"}),
	}

	server := NewMockServer(routes)
	defer server.Close()

	// Make some requests
	for i := 0; i < 3; i++ {
		resp, err := http.Get(server.URL() + "/transactions/abc123")
		assert.NoError(t, err)
		resp.Body.Close()
	}

	assert.Equal(t, 3, server.CallCount("/transactions/abc123"))

	// Reset counts
	server.ResetCallCounts()
	assert.Equal(t, 0, server.CallCount("/transactions/abc123"))
}

func TestMockServer_CustomHeaders(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/abc123": {
			StatusCode: http.StatusOK,
			Body:       MockTransactionResponse{Hash: "abc123"},
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
				"Cache-Control":   "no-cache",
			},
		},
	}

	server := NewMockServer(routes)
	defer server.Close()

	resp, err := http.Get(server.URL() + "/transactions/abc123")
	assert.NoError(t, err)

	assert.Equal(t, "custom-value", resp.Header.Get("X-Custom-Header"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
}

func TestMockServer_MultipleEndpoints(t *testing.T) {
	txResp := MockTransactionResponse{
		Hash:          "abc123",
		EnvelopeXdr:   "envelope",
		ResultXdr:     "result",
		ResultMetaXdr: "meta",
	}

	accountResp := MockAccountResponse{
		ID:        "gtest",
		AccountID: "GTEST...",
		Balance:   "1000.0000000",
	}

	routes := map[string]MockRoute{
		"/transactions/abc123":  SuccessRoute(txResp),
		"/accounts/gtest":       SuccessRoute(accountResp),
		"/transactions/limited": RateLimitRoute(),
		"/transactions/error":   ServerErrorRoute(),
	}

	server := NewMockServer(routes)
	defer server.Close()

	// Test transaction endpoint
	resp, err := http.Get(server.URL() + "/transactions/abc123")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Test account endpoint
	resp, err = http.Get(server.URL() + "/accounts/gtest")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Test rate limit
	resp, err = http.Get(server.URL() + "/transactions/limited")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	resp.Body.Close()

	// Test server error
	resp, err = http.Get(server.URL() + "/transactions/error")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	resp.Body.Close()

	// Test 404
	resp, err = http.Get(server.URL() + "/unknown")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestMockServer_ClientUsage(t *testing.T) {
	// This test demonstrates how to use the mock server with NewClientWithURL
	txResp := MockTransactionResponse{
		Hash:          "tx123",
		EnvelopeXdr:   "envelope-xdr-data",
		ResultXdr:     "result-xdr-data",
		ResultMetaXdr: "meta-xdr-data",
	}

	routes := map[string]MockRoute{
		"/transactions/tx123": SuccessRoute(txResp),
	}

	mockServer := NewMockServer(routes)
	defer mockServer.Close()

	// Create a client pointing to the mock server
	client := NewClientWithURL(mockServer.URL(), Testnet, "")
	assert.NotNil(t, client)

	// The horizonclient would now use the mock server URLs
	// This is a placeholder to show how the mock server would be used
	assert.Equal(t, Testnet, client.Network)
}

func TestMockServer_RequestBody(t *testing.T) {
	// Test that the server can handle POST requests and read request body
	server := NewMockServer(map[string]MockRoute{
		"/submit": {
			StatusCode: http.StatusOK,
			Body: map[string]string{
				"status": "success",
				"id":     "submitted-tx-123",
			},
		},
	})
	defer server.Close()

	resp, err := http.Post(
		server.URL()+"/submit",
		"application/json",
		nil,
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "success")
}

func TestMockServer_EmptyBody(t *testing.T) {
	routes := map[string]MockRoute{
		"/health": {
			StatusCode: http.StatusOK,
			Body:       nil,
		},
	}

	server := NewMockServer(routes)
	defer server.Close()

	resp, err := http.Get(server.URL() + "/health")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Empty(t, body)
}
