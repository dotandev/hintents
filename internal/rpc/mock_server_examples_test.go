// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMockServer_CompleteWorkflow demonstrates how to use the mock server for testing
// This test shows a complete testing workflow.
func TestMockServer_CompleteWorkflow(t *testing.T) {
	// Step 1: Create mock responses
	successTx := MockTransactionResponse{
		ID:            "tx-success",
		Hash:          "abc123",
		EnvelopeXdr:   "envelope-xdr",
		ResultXdr:     "result-xdr",
		ResultMetaXdr: "meta-xdr",
		Ledger:        1000,
		CreatedAt:     "2026-01-28T12:00:00Z",
	}

	successAccount := MockAccountResponse{
		ID:            "acc-success",
		AccountID:     "GADDRESS123...",
		Balance:       "1000.0000000",
		Sequence:      "42",
		CreatedAt:     "2026-01-28T12:00:00Z",
		UpdatedAt:     "2026-01-28T12:00:00Z",
		SubentryCount: 5,
	}

	// Step 2: Define routes for the mock server
	routes := map[string]MockRoute{
		// Successful responses
		"/transactions/abc123": SuccessRoute(successTx),
		"/accounts/GADDRESS":   SuccessRoute(successAccount),

		// Error scenarios
		"/transactions/ratelimit": RateLimitRoute(),
		"/transactions/error":     ServerErrorRoute(),
		"/forbidden":              ErrorRoute(http.StatusForbidden, "access denied"),
	}

	// Step 3: Create and start the mock server
	mockServer := NewMockServer(routes)
	defer mockServer.Close()

	// Step 4: Create a client pointing to the mock server
	client := NewClientWithURL(mockServer.URL(), Testnet)
	assert.NotNil(t, client)

	// Step 5: Test successful scenarios - use direct HTTP to mock server
	// (The real horizonclient would use these endpoints)
	resp, err := http.Get(mockServer.URL() + "/transactions/abc123")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Step 6: Verify that endpoints were called
	callCount := mockServer.CallCount("/transactions/abc123")
	assert.Equal(t, 1, callCount)
}

// TestMockServer_ErrorHandlingScenarios demonstrates error scenario testing
func TestMockServer_ErrorHandlingScenarios(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/429": RateLimitRoute(),
		"/transactions/500": ServerErrorRoute(),
		"/transactions/404": {
			StatusCode: http.StatusNotFound,
			Body: map[string]string{
				"status": "error",
				"detail": "transaction not found",
			},
		},
	}

	mockServer := NewMockServer(routes)
	defer mockServer.Close()

	// Test rate limiting handling
	resp, _ := http.Get(mockServer.URL() + "/transactions/429")
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "60", resp.Header.Get("Retry-After"))

	// Test server error handling
	resp, _ = http.Get(mockServer.URL() + "/transactions/500")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Test not found handling
	resp, _ = http.Get(mockServer.URL() + "/transactions/404")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestMockServer_DynamicRoutesManagement demonstrates adding and removing routes at runtime
func TestMockServer_DynamicRoutesManagement(t *testing.T) {
	mockServer := NewMockServer(map[string]MockRoute{})
	defer mockServer.Close()

	// Initially the route doesn't exist
	resp, _ := http.Get(mockServer.URL() + "/transactions/new")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Add a new route dynamically
	newTx := MockTransactionResponse{
		Hash:          "dynamic-tx",
		EnvelopeXdr:   "dynamic-envelope",
		ResultXdr:     "dynamic-result",
		ResultMetaXdr: "dynamic-meta",
	}
	mockServer.AddRoute("/transactions/new", SuccessRoute(newTx))

	// Now the route exists
	resp, _ = http.Get(mockServer.URL() + "/transactions/new")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Remove the route
	mockServer.RemoveRoute("/transactions/new")

	// Now it's gone again
	resp, _ = http.Get(mockServer.URL() + "/transactions/new")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestMockServer_RequestTrackingVerification demonstrates verifying which endpoints were called
func TestMockServer_RequestTrackingVerification(t *testing.T) {
	routes := map[string]MockRoute{
		"/transactions/abc": SuccessRoute(MockTransactionResponse{Hash: "abc"}),
		"/accounts/xyz":     SuccessRoute(MockAccountResponse{ID: "xyz"}),
	}

	mockServer := NewMockServer(routes)
	defer mockServer.Close()

	// Make several requests
	for i := 0; i < 5; i++ {
		resp, err := http.Get(mockServer.URL() + "/transactions/abc")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()
	}

	for i := 0; i < 3; i++ {
		resp, err := http.Get(mockServer.URL() + "/accounts/xyz")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		resp.Body.Close()
	}

	// Verify call counts
	assert.Equal(t, 5, mockServer.CallCount("/transactions/abc"))
	assert.Equal(t, 3, mockServer.CallCount("/accounts/xyz"))

	// Reset counts
	mockServer.ResetCallCounts()
	assert.Equal(t, 0, mockServer.CallCount("/transactions/abc"))
}

// TestMockServer_CustomResponseHeaders demonstrates setting custom response headers
func TestMockServer_CustomResponseHeaders(t *testing.T) {
	routes := map[string]MockRoute{
		"/with-headers": {
			StatusCode: http.StatusOK,
			Body: map[string]string{
				"status": "ok",
			},
			Headers: map[string]string{
				"X-Custom":      "header-value",
				"Cache-Control": "no-cache",
				"Retry-After":   "30",
			},
		},
	}

	mockServer := NewMockServer(routes)
	defer mockServer.Close()

	resp, _ := http.Get(mockServer.URL() + "/with-headers")
	assert.Equal(t, "header-value", resp.Header.Get("X-Custom"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "30", resp.Header.Get("Retry-After"))
}

// TestMockServer_OfflineTestingWorkflow shows a realistic offline testing scenario
func TestMockServer_OfflineTestingWorkflow(t *testing.T) {
	// Simulate a complete transaction workflow
	successResponse := MockTransactionResponse{
		ID:            "tx123",
		Hash:          "abc123",
		EnvelopeXdr:   "AAAAAGAIyXWVfkfbPL0b5cyy6MTXLsHMx7OG2M7qsQ/aKQAAAGQBcvNrABT5gAAAAB0ABwAAAAEf/d+p+qVfsFvDGYLBSXJR3EIRhJJ4KJHJlgCTe5yJ+FaHF0KhNKe1hTVsYzjJR4F/Gfm5mSRwVcVr2xIL0DmEL0KYVr/hHVb47TwJgQYVLvGwAAAC6H+UQAAAAAAAAAAQAAABsAAAAAAAAAAAB/93aoaaxqrGpxbxkPLgkkkBxvqCf8xVhkF5i6l7l4qAVXR1KJDg8SkL7o4hzRO8wCM5OzGppCL5AiBzWe/eM1OxOlkGAJE2u8j5jrBxrPrgbPNq4c8HSGXNLB5lT+PqiN0Q==",
		ResultXdr:     "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA=",
		ResultMetaXdr: "AAAAAgAAAAA=",
		Ledger:        12345,
		CreatedAt:     "2026-01-28T12:00:00Z",
	}

	routes := map[string]MockRoute{
		"/transactions/abc123":    SuccessRoute(successResponse),
		"/transactions/notfound":  ErrorRoute(http.StatusNotFound, "transaction not found"),
		"/transactions/ratelimit": RateLimitRoute(),
		"/transactions/server":    ServerErrorRoute(),
	}

	mockServer := NewMockServer(routes)
	defer mockServer.Close()

	// Test 1: Successful transaction fetch
	resp, err := http.Get(mockServer.URL() + "/transactions/abc123")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test 2: Handle not found
	resp, err = http.Get(mockServer.URL() + "/transactions/notfound")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Test 3: Handle rate limiting
	resp, err = http.Get(mockServer.URL() + "/transactions/ratelimit")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// Test 4: Handle server error
	resp, err = http.Get(mockServer.URL() + "/transactions/server")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Verify all endpoints were tested
	assert.Equal(t, 1, mockServer.CallCount("/transactions/abc123"))
	assert.Equal(t, 1, mockServer.CallCount("/transactions/notfound"))
	assert.Equal(t, 1, mockServer.CallCount("/transactions/ratelimit"))
	assert.Equal(t, 1, mockServer.CallCount("/transactions/server"))
}
