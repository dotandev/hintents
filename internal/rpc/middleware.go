// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"net/http"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

// NewLoggingMiddleware returns a Middleware that logs each outbound HTTP request
// and its response at INFO level using the package-level structured logger.
//
// Each log record includes:
//   - method  – HTTP verb (GET, POST, …)
//   - url     – full request URL
//   - status  – HTTP response status code
//   - latency – round-trip duration in milliseconds
//
// Errors from the inner transport are logged at ERROR level with an "error" field
// instead of a status code.
func NewLoggingMiddleware() Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &loggingTransport{next: next}
	}
}

type loggingTransport struct {
	next http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.next.RoundTrip(req)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}

		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}

		logger.Logger.Error("http request failed",
			"method", req.Method,
			"url", req.URL.String(),
			"latency_ms", latencyMs,
			"status", statusCode,
			"error", err,
		)
		return resp, err
	}

	logger.Logger.Info("http request completed",
		"method", req.Method,
		"url", req.URL.String(),
		"status", resp.StatusCode,
		"latency_ms", latencyMs,
	)
	return resp, nil
}
