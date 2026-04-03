// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
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
		JitterFraction:     0.1, // 10% jitter to prevent thundering herd
		StatusCodesToRetry: []int{429, 503, 504},
	}
}

// retryLogic holds the shared retry behavior used by both Retrier and RetryTransport.
// Embedding this struct in either type promotes its methods, eliminating duplicated code
// while keeping each type's transport/client wiring independent.
type retryLogic struct {
	config RetryConfig
}

// shouldRetry reports whether the given HTTP status code warrants a retry.
func (rl retryLogic) shouldRetry(statusCode int) bool {
	for _, code := range rl.config.StatusCodesToRetry {
		if statusCode == code {
			return true
		}
	}
	return false
}

// getRetryAfter parses the Retry-After response header.
// Supports both integer-seconds and RFC 1123 HTTP-date formats (RFC 7231 §7.1.3).
func (rl retryLogic) getRetryAfter(resp *http.Response) time.Duration {
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

// nextBackoff calculates the next wait duration using exponential backoff with full jitter.
// Full jitter prevents thundering-herd problems when many clients retry simultaneously.
func (rl retryLogic) nextBackoff(current time.Duration) time.Duration {
	// Exponential backoff: double the current duration, capped at MaxBackoff
	next := time.Duration(float64(current) * 2)
	if next > rl.config.MaxBackoff {
		next = rl.config.MaxBackoff
	}

	// Full jitter: random value in [0, next*(1+JitterFraction))
	if rl.config.JitterFraction > 0 {
		maxJitter := float64(next) * (1.0 + rl.config.JitterFraction)
		jitter := time.Duration(rand.Float64() * maxJitter)
		next = jitter
		if next < 0 {
			next = 0
		}
	}

	return next
}

// waitWithContext sleeps for duration or returns early when ctx is cancelled.
func (rl retryLogic) waitWithContext(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Retrier handles HTTP request retries with exponential backoff and jitter.
type Retrier struct {
	retryLogic
	client *http.Client
}

// NewRetrier creates a new Retrier with the given config and HTTP client.
func NewRetrier(config RetryConfig, client *http.Client) *Retrier {
	if client == nil {
		client = http.DefaultClient
	}
	return &Retrier{
		retryLogic: retryLogic{config: config},
		client:     client,
	}
}

// Do executes an HTTP request with retry logic.
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

		// HTTP 413: response too large -- not retryable
		if resp.StatusCode == http.StatusRequestEntityTooLarge {
			resp.Body.Close()
			return nil, errors.WrapRPCResponseTooLarge(req.URL.String())
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

// RetryTransport is an http.RoundTripper that adds retry logic to every request.
type RetryTransport struct {
	retryLogic
	transport http.RoundTripper
}

// NewRetryTransport creates a new RetryTransport with the given config.
func NewRetryTransport(config RetryConfig, transport http.RoundTripper) *RetryTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &RetryTransport{
		retryLogic: retryLogic{config: config},
		transport:  transport,
	}
}

// RoundTrip implements http.RoundTripper with retry logic.
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

		// HTTP 413: response too large -- not retryable
		if resp.StatusCode == http.StatusRequestEntityTooLarge {
			resp.Body.Close()
			return nil, errors.WrapRPCResponseTooLarge(req.URL.String())
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
