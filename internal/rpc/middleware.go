// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"net/http"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

// NewLoggingMiddleware returns a Middleware that logs each outbound HTTP request
// and its response at the transport layer using the package-level structured logger.
//
// Logging policy (unified across Horizon and Soroban paths):
//   - Success at the HTTP transport layer: INFO  (user-opt-in per-request tracing)
//   - Failure at the HTTP transport layer: ERROR
//   - Success at the RPC attempt level:    DEBUG  (see client.go attempt functions)
//   - Failure at the RPC attempt level:    ERROR
//   - Retry / failover events:             WARN
//
// Each log record includes:
//   - method     – HTTP verb (GET, POST, …)
//   - url        – full request URL
//   - status     – HTTP response status code
//   - latency_ms – round-trip duration in milliseconds
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
