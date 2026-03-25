// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type headerMiddleware struct {
	next http.RoundTripper
}

func (m *headerMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Custom-Header", "injected")
	return m.next.RoundTrip(req)
}

func TestMiddlewareInjection(t *testing.T) {
	// Setup a mock server to check headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") == "injected" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc": "2.0", "result": {"status": "healthy"}, "id": 1}`))
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	// Define a custom middleware
	mw := func(next http.RoundTripper) http.RoundTripper {
		return &headerMiddleware{next: next}
	}

	// Create client with middleware
	client, err := NewClient(
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		WithMiddleware(mw),
	)
	assert.NoError(t, err)

	// Test a call that uses the HTTP client
	ctx := context.Background()
	resp, err := client.GetHealth(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Result.Status)
}

func BenchmarkMiddleware(b *testing.B) {
	// Simple middleware that does nothing
	mw := func(next http.RoundTripper) http.RoundTripper {
		return next
	}

	client, _ := NewClient(WithMiddleware(mw))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Just creating the client or doing something light
		_ = client.getHTTPClient()
	}
}
