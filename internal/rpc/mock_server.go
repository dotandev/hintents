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
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockServer provides a mock HTTP server for testing Stellar Horizon and RPC endpoints
type MockServer struct {
	server    *httptest.Server
	routes    map[string]MockRoute
	mu        sync.RWMutex
	callCount map[string]int
}

// MockRoute defines the response configuration for a specific endpointhttps://github.com/dotandev/hintents
type MockRoute struct {
	StatusCode int
	Body       interface{}
	Headers    map[string]string
}

// NewMockServer creates a new mock server with the given routes
func NewMockServer(routes map[string]MockRoute) *MockServer {
	ms := &MockServer{
		routes:    make(map[string]MockRoute),
		callCount: make(map[string]int),
	}

	// Copy routes to the server
	for path, route := range routes {
		ms.routes[path] = route
	}

	// Create the HTTP test server
	ms.server = httptest.NewServer(http.HandlerFunc(ms.handleRequest))

	return ms
}

// handleRequest handles incoming HTTP requests and returns the configured response
func (ms *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	ms.mu.Lock()
	ms.callCount[r.RequestURI]++
	ms.mu.Unlock()

	ms.mu.RLock()
	route, exists := ms.routes[r.RequestURI]
	ms.mu.RUnlock()

	// Set default response headers
	w.Header().Set("Content-Type", "application/json")

	if !exists {
		// Return 404 for unmapped endpoints
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"detail": fmt.Sprintf("endpoint not found: %s", r.RequestURI),
		}); err != nil {
			log.Printf("failed to encode response: %v", err)
		}
		return
	}

	// Set custom headers if provided
	if route.Headers != nil {
		for key, value := range route.Headers {
			w.Header().Set(key, value)
		}
	}

	w.WriteHeader(route.StatusCode)

	// Encode the response body
	if route.Body != nil {
		if err := json.NewEncoder(w).Encode(route.Body); err != nil {
			// If encoding fails, write error to response
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

// URL returns the base URL of the mock server
func (ms *MockServer) URL() string {
	return ms.server.URL
}

// Close stops the mock server
func (ms *MockServer) Close() {
	if ms.server != nil {
		ms.server.Close()
	}
}

// AddRoute adds or updates a route in the running server
func (ms *MockServer) AddRoute(path string, route MockRoute) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.routes[path] = route
}

// RemoveRoute removes a route from the running server
func (ms *MockServer) RemoveRoute(path string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.routes, path)
}

// CallCount returns the number of times a specific endpoint was called
func (ms *MockServer) CallCount(path string) int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.callCount[path]
}

// ResetCallCounts resets all call counts
func (ms *MockServer) ResetCallCounts() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.callCount = make(map[string]int)
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// TransactionResponse represents a mock transaction response from Horizon
type MockTransactionResponse struct {
	ID            string `json:"id"`
	EnvelopeXdr   string `json:"envelope_xdr"`
	ResultXdr     string `json:"result_xdr"`
	ResultMetaXdr string `json:"result_meta_xdr"`
	Hash          string `json:"hash"`
	Ledger        int64  `json:"ledger"`
	CreatedAt     string `json:"created_at"`
}

// AccountResponse represents a mock account response from Horizon
type MockAccountResponse struct {
	ID            string `json:"id"`
	AccountID     string `json:"account_id"`
	Balance       string `json:"balance"`
	Sequence      string `json:"sequence"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	SubentryCount int64  `json:"subentry_count"`
}

// Helper function to create a standard error route
func ErrorRoute(statusCode int, detail string) MockRoute {
	return MockRoute{
		StatusCode: statusCode,
		Body: ErrorResponse{
			Status: http.StatusText(statusCode),
			Detail: detail,
		},
	}
}

// RateLimitRoute creates a route that simulates rate limiting (HTTP 429)
func RateLimitRoute() MockRoute {
	return MockRoute{
		StatusCode: http.StatusTooManyRequests,
		Body: ErrorResponse{
			Status: "rate_limit_exceeded",
			Detail: "too many requests - rate limit exceeded",
		},
		Headers: map[string]string{
			"Retry-After": "60",
		},
	}
}

// ServerErrorRoute creates a route that simulates server error (HTTP 500)
func ServerErrorRoute() MockRoute {
	return MockRoute{
		StatusCode: http.StatusInternalServerError,
		Body: ErrorResponse{
			Status: "server_error",
			Detail: "internal server error",
		},
	}
}

// SuccessRoute creates a route with a successful response
func SuccessRoute(body interface{}) MockRoute {
	return MockRoute{
		StatusCode: http.StatusOK,
		Body:       body,
	}
}
