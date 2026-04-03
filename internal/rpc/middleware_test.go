// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type headerMiddleware struct {
	next http.RoundTripper
}

type trackingReadCloser struct {
	closed bool
}

func (t *trackingReadCloser) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
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

// TestCreateHTTPClient_MiddlewareAppliedOnce verifies that each middleware is wrapped
// into the transport chain exactly once.  Before the Protocol V2 standardization fix,
// createHTTPClient applied the middleware slice twice (once before RetryTransport and
// once after), causing every middleware to intercept each request twice.
func TestCreateHTTPClient_MiddlewareAppliedOnce(t *testing.T) {
	wrapCount := 0
	mw := func(next http.RoundTripper) http.RoundTripper {
		wrapCount++
		return next
	}

	_ = createHTTPClient("token", 5*time.Second, mw)

	if wrapCount != 1 {
		t.Errorf("middleware should be applied exactly once, got %d applications", wrapCount)
	}
}

// TestCreateHTTPClient_MultipleMiddlewaresAppliedOnceEach verifies the invariant holds
// when more than one middleware is provided.
func TestCreateHTTPClient_MultipleMiddlewaresAppliedOnceEach(t *testing.T) {
	counts := make([]int, 3)
	mws := make([]Middleware, 3)
	for i := range mws {
		i := i
		mws[i] = func(next http.RoundTripper) http.RoundTripper {
			counts[i]++
			return next
		}
	}

	_ = createHTTPClient("", 5*time.Second, mws...)

	for i, c := range counts {
		if c != 1 {
			t.Errorf("middleware[%d] should be applied exactly once, got %d", i, c)
		}
	}
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

// captureHandler returns an httptest server and a channel that receives every request.
func captureServer(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
}

// redirectLogger swaps the package-level logger so log output can be inspected in tests.
// It returns a restore function that must be called when the test is done.
func redirectLogger(buf *bytes.Buffer) func() {
	orig := logger.Logger
	logger.Logger = slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return func() { logger.Logger = orig }
}

// TestLoggingMiddleware_SuccessfulRequest verifies that a successful request
// produces an INFO log entry with method, url, status, and latency fields.
func TestLoggingMiddleware_SuccessfulRequest(t *testing.T) {
	healthBody := `{"jsonrpc":"2.0","result":{"status":"healthy"},"id":1}`
	server := captureServer(t, http.StatusOK, healthBody)
	defer server.Close()

	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	client, err := NewClient(
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		WithLoggingEnabled(true),
	)
	require.NoError(t, err)

	_, _ = client.GetHealth(context.Background())

	require.NotEmpty(t, buf.String(), "expected at least one log line")

	// Find the "http request completed" log entry.
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry["msg"] == "http request completed" {
			found = true
			assert.Equal(t, "INFO", entry["level"])
			assert.NotEmpty(t, entry["url"])
			assert.NotNil(t, entry["status"])
			assert.NotNil(t, entry["latency_ms"])
			break
		}
	}
	assert.True(t, found, "expected 'http request completed' log entry")
}

// TestLoggingMiddleware_FailedRequest verifies that a transport-level error
// produces an ERROR log entry with an error field.
func TestLoggingMiddleware_FailedRequest(t *testing.T) {
	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	// errorTransport always returns an error to simulate a network failure.
	errTransport := RoundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("simulated network failure")
	})

	lmw := NewLoggingMiddleware()
	transport := lmw(errTransport)

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/rpc", nil)
	resp, err := transport.RoundTrip(req)

	assert.Nil(t, resp)
	assert.Error(t, err)

	require.NotEmpty(t, buf.String(), "expected at least one log line")

	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if jsonErr := json.Unmarshal(line, &entry); jsonErr != nil {
			continue
		}
		if entry["msg"] == "http request failed" {
			found = true
			assert.Equal(t, "ERROR", entry["level"])
			assert.NotEmpty(t, entry["error"])
			break
		}
	}
	assert.True(t, found, "expected 'http request failed' log entry")
}

// TestLoggingMiddleware_FailedRequestWithResponse ensures we preserve the
// transport response while closing its body when an error is returned.
func TestLoggingMiddleware_FailedRequestWithResponse(t *testing.T) {
	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	body := &trackingReadCloser{}
	errTransport := RoundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       body,
		}, errors.New("upstream failure")
	})

	lmw := NewLoggingMiddleware()
	transport := lmw(errTransport)

	req, _ := http.NewRequest(http.MethodGet, "http://example.invalid/rpc", nil)
	resp, err := transport.RoundTrip(req)

	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.True(t, body.closed, "expected response body to be closed on error")

	require.NotEmpty(t, buf.String(), "expected at least one log line")

	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if jsonErr := json.Unmarshal(line, &entry); jsonErr != nil {
			continue
		}
		if entry["msg"] == "http request failed" {
			found = true
			assert.Equal(t, float64(http.StatusBadGateway), entry["status"])
			break
		}
	}
	assert.True(t, found, "expected 'http request failed' log entry")
}

// TestWithLoggingEnabled_DisabledByDefault checks that no logging middleware is
// active unless explicitly opted in, so users are not surprised by log output.
func TestWithLoggingEnabled_DisabledByDefault(t *testing.T) {
	var buf bytes.Buffer
	restore := redirectLogger(&buf)
	defer restore()

	server := captureServer(t, http.StatusOK, `{"jsonrpc":"2.0","result":{"status":"healthy"},"id":1}`)
	defer server.Close()

	client, err := NewClient(
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		// No WithLoggingEnabled(true)
	)
	require.NoError(t, err)

	_, _ = client.GetHealth(context.Background())

	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var entry map[string]any
		if json.Unmarshal(line, &entry) == nil {
			assert.NotEqual(t, "http request completed", entry["msg"],
				"logging middleware should not be active by default")
		}
	}
}

// TestMiddlewareChainOrdering ensures that when multiple middlewares are stacked
// they execute in the expected outermost-first order.
func TestMiddlewareChainOrdering(t *testing.T) {
	var order []string

	makeTracer := func(name string) Middleware {
		return func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				order = append(order, name+":before")
				resp, err := next.RoundTrip(req)
				order = append(order, name+":after")
				return resp, err
			})
		}
	}

	server := captureServer(t, http.StatusOK, `{"jsonrpc":"2.0","result":{"status":"healthy"},"id":1}`)
	defer server.Close()

	client, err := NewClient(
		WithHorizonURL(server.URL),
		WithSorobanURL(server.URL),
		WithMiddleware(makeTracer("outer"), makeTracer("inner")),
	)
	require.NoError(t, err)

	_, _ = client.GetHealth(context.Background())

	// "outer" should be applied last and therefore appear first in the call sequence.
	require.GreaterOrEqual(t, len(order), 4, "expected at least 4 trace entries")
	assert.Equal(t, "outer:before", order[0])
	assert.Equal(t, "inner:before", order[1])
	assert.Equal(t, "inner:after", order[len(order)-2])
	assert.Equal(t, "outer:after", order[len(order)-1])
}
