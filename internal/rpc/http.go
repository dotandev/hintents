// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// authTransport is a custom HTTP RoundTripper that adds authentication headers
type authTransport struct {
	token     string
	transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper interface
func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		// Add Bearer token to Authorization header
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.transport.RoundTrip(req)
}

// Middleware defines a function that wraps an http.RoundTripper
type Middleware func(http.RoundTripper) http.RoundTripper

// RoundTripperFunc is a helper to implement http.RoundTripper with a function
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper
func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (c *Client) getHTTPClient() *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return http.DefaultClient
}

// createHTTPClient creates an HTTP client with optional authentication, a configurable timeout, and custom middlewares.
func createHTTPClient(token string, timeout time.Duration, middlewares ...Middleware) *http.Client {
	cfg := DefaultRetryConfig()

	var transport http.RoundTripper = http.DefaultTransport
	if token != "" {
		transport = &authTransport{
			token:     token,
			transport: transport,
		}
	}

	transport = NewRetryTransport(cfg, transport)

	// Apply custom middlewares in reverse order so the first one becomes outermost
	for i := len(middlewares) - 1; i >= 0; i-- {
		if mw := middlewares[i]; mw != nil {
			transport = mw(transport)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func (c *Client) postRequest(ctx context.Context, payload interface{}, result interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use c.SorobanURL as the endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", c.SorobanURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use the client's internal httpClient
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}
