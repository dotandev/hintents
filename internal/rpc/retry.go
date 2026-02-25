// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
)

// RetryConfig defines the retry behavior
type RetryConfig struct {
	MaxRetries         int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	JitterFraction     float64
	StatusCodesToRetry []int
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:         3,
		InitialBackoff:     1 * time.Second,
		MaxBackoff:         10 * time.Second,
		JitterFraction:     0.1,
		StatusCodesToRetry: []int{429, 503, 504},
	}
}

// Retrier handles HTTP request retries with exponential backoff and jitter
type Retrier struct {
	config RetryConfig
	client *http.Client
}

// NewRetrier creates a new Retrier with the given config and HTTP client
func NewRetrier(config RetryConfig, client *http.Client) *Retrier {
	if client == nil {
		client = http.DefaultClient
	}
	return &Retrier{
		config: config,
		client: client,
	}
}

// Do executes an HTTP request with retry logic
func (r *Retrier) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
	backoff := r.config.InitialBackoff

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := r.waitWithContext(ctx, backoff); err != nil {
				return nil, errors.WrapRPCTimeout(err)
			}
		}

		resp, err := r.client.Do(req.Clone(ctx))
		if err != nil {
			lastErr = err
			if attempt < r.config.MaxRetries {
				logger.Logger.Debug("Request failed, will retry", "attempt", attempt+1, "error", err)
			}
			backoff = r.nextBackoff(backoff)
			continue
		}

		// Check if response status is retryable
		if r.shouldRetry(resp.StatusCode) {
			lastErr = fmt.Errorf("status code %d", resp.StatusCode)
			retryAfter := r.getRetryAfter(resp)

			logger.Logger.Warn("Rate limited or temporary failure, will retry",
				"attempt", attempt+1,
				"status_code", resp.StatusCode,
				"retry_after", retryAfter,
			)

			resp.Body.Close()

			if retryAfter > 0 {
				backoff = retryAfter
			} else {
				backoff = r.nextBackoff(backoff)
			}

			if attempt < r.config.MaxRetries {
				continue
			}
			// If we've exhausted retries on a retryable error, return error
			return nil, errors.WrapRPCConnectionFailed(lastErr)
		}

		// Success or non-retryable error
		return resp, nil
	}

	return nil, errors.WrapRPCConnectionFailed(lastErr)
}

// shouldRetry determines if the response status code warrants a retry
func (r *Retrier) shouldRetry(statusCode int) bool {
	for _, code := range r.config.StatusCodesToRetry {
		if statusCode == code {
			return true
		}
	}
	return false
}

// getRetryAfter parses the Retry-After header and returns the duration
// Supports both "seconds" and "HTTP-date" formats (RFC 7231)
func (r *Retrier) getRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP-date
	if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		dur := time.Until(t)
		if dur > 0 {
			return dur
		}
	}

	return 0
}

// nextBackoff calculates the next backoff duration with exponential backoff and jitter
func (r *Retrier) nextBackoff(current time.Duration) time.Duration {
	// Exponential backoff: double the current duration
	next := time.Duration(float64(current) * 2)
	if next > r.config.MaxBackoff {
		next = r.config.MaxBackoff
	}

	// Add jitter: ±JitterFraction of the duration
	if r.config.JitterFraction > 0 {
		jitterAmount := float64(next) * r.config.JitterFraction
		jitterRange := math.Round(jitterAmount)
		jitter := time.Duration(rand.Int63n(int64(jitterRange)*2) - int64(jitterRange))
		next = next + jitter
		if next < 0 {
			next = 0
		}
	}

	return next
}

// waitWithContext waits for the specified duration or until context is cancelled
func (r *Retrier) waitWithContext(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RetryTransport is an http.RoundTripper that adds retry logic to requests
type RetryTransport struct {
	config    RetryConfig
	transport http.RoundTripper
}

// NewRetryTransport creates a new RetryTransport with the given config
func NewRetryTransport(config RetryConfig, transport http.RoundTripper) *RetryTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &RetryTransport{
		config:    config,
		transport: transport,
	}
}

// RoundTrip implements http.RoundTripper interface with retry logic
func (rt *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error
	backoff := rt.config.InitialBackoff

	for attempt := 0; attempt <= rt.config.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := rt.waitWithContext(req.Context(), backoff); err != nil {
				return nil, errors.WrapRPCTimeout(err)
			}
		}

		resp, err := rt.transport.RoundTrip(req)
		if err != nil {
			lastErr = err
			if attempt < rt.config.MaxRetries {
				logger.Logger.Debug("RoundTrip failed, will retry", "attempt", attempt+1, "error", err)
			}
			backoff = rt.nextBackoff(backoff)
			continue
		}

		// Check if response status is retryable
		if rt.shouldRetry(resp.StatusCode) {
			lastErr = fmt.Errorf("status code %d", resp.StatusCode)
			retryAfter := rt.getRetryAfter(resp)

			logger.Logger.Warn("Rate limited or temporary failure, will retry",
				"attempt", attempt+1,
				"status_code", resp.StatusCode,
				"retry_after", retryAfter,
			)

			resp.Body.Close()

			if retryAfter > 0 {
				backoff = retryAfter
			} else {
				backoff = rt.nextBackoff(backoff)
			}

			if attempt < rt.config.MaxRetries {
				continue
			}
			// If we've exhausted retries on a retryable error, return error
			return nil, errors.WrapRPCConnectionFailed(lastErr)
		}

		// Success or non-retryable error
		return resp, nil
	}

	return nil, errors.WrapRPCConnectionFailed(lastErr)
}

// shouldRetry determines if the response status code warrants a retry
func (rt *RetryTransport) shouldRetry(statusCode int) bool {
	for _, code := range rt.config.StatusCodesToRetry {
		if statusCode == code {
			return true
		}
	}
	return false
}

// getRetryAfter parses the Retry-After header and returns the duration
func (rt *RetryTransport) getRetryAfter(resp *http.Response) time.Duration {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP-date
	if t, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		dur := time.Until(t)
		if dur > 0 {
			return dur
		}
	}

	return 0
}

// nextBackoff calculates the next backoff duration with exponential backoff and jitter
func (rt *RetryTransport) nextBackoff(current time.Duration) time.Duration {
	// Exponential backoff: double the current duration
	next := time.Duration(float64(current) * 2)
	if next > rt.config.MaxBackoff {
		next = rt.config.MaxBackoff
	}

	// Add jitter: ±JitterFraction of the duration
	if rt.config.JitterFraction > 0 {
		jitterAmount := float64(next) * rt.config.JitterFraction
		jitterRange := math.Round(jitterAmount)
		jitter := time.Duration(rand.Int63n(int64(jitterRange)*2) - int64(jitterRange))
		next = next + jitter
		if next < 0 {
			next = 0
		}
	}

	return next
}

// waitWithContext waits for the specified duration or until context is cancelled
func (rt *RetryTransport) waitWithContext(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
