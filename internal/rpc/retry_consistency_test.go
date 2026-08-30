// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRetryLogic_ConsistentBackoff verifies that Retrier and RetryTransport produce
// backoff values in the same range when given identical RetryConfig values.
// Before the Protocol V2 standardization, both types duplicated the backoff logic
// independently; drift between copies could silently alter retry behavior.
func TestRetryLogic_ConsistentBackoff(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:         3,
		InitialBackoff:     100 * time.Millisecond,
		MaxBackoff:         1 * time.Second,
		JitterFraction:     0.1,
		StatusCodesToRetry: []int{429, 503},
	}

	retrier := NewRetrier(cfg, nil)
	transport := NewRetryTransport(cfg, nil)

	expectedMax := time.Duration(float64(cfg.InitialBackoff) * 2 * (1.0 + cfg.JitterFraction))

	const samples = 200
	for i := 0; i < samples; i++ {
		rb := retrier.nextBackoff(cfg.InitialBackoff)
		tb := transport.nextBackoff(cfg.InitialBackoff)

		assert.GreaterOrEqual(t, rb, time.Duration(0),
			"Retrier backoff must be non-negative")
		assert.LessOrEqual(t, rb, expectedMax,
			"Retrier backoff must not exceed expected maximum")

		assert.GreaterOrEqual(t, tb, time.Duration(0),
			"RetryTransport backoff must be non-negative")
		assert.LessOrEqual(t, tb, expectedMax,
			"RetryTransport backoff must not exceed expected maximum")
	}
}

// TestRetryLogic_ShouldRetryConsistency verifies that Retrier and RetryTransport
// classify the same status codes as retryable.
func TestRetryLogic_ShouldRetryConsistency(t *testing.T) {
	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, nil)
	transport := NewRetryTransport(cfg, nil)

	codes := []int{200, 400, 404, 413, 429, 500, 503, 504}
	for _, code := range codes {
		assert.Equal(t,
			retrier.shouldRetry(code),
			transport.shouldRetry(code),
			"shouldRetry result must match between Retrier and RetryTransport for status %d", code,
		)
	}
}

// TestRetryLogic_NoJitter verifies that with JitterFraction=0 both types return
// a deterministic exponential backoff (no randomness).
func TestRetryLogic_NoJitter(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:         3,
		InitialBackoff:     200 * time.Millisecond,
		MaxBackoff:         10 * time.Second,
		JitterFraction:     0,
		StatusCodesToRetry: []int{429},
	}

	retrier := NewRetrier(cfg, nil)
	transport := NewRetryTransport(cfg, nil)

	expected := time.Duration(float64(cfg.InitialBackoff) * 2) // 400ms

	for i := 0; i < 5; i++ {
		assert.Equal(t, expected, retrier.nextBackoff(cfg.InitialBackoff),
			"Retrier: deterministic backoff expected when JitterFraction=0")
		assert.Equal(t, expected, transport.nextBackoff(cfg.InitialBackoff),
			"RetryTransport: deterministic backoff expected when JitterFraction=0")
	}
}

// BenchmarkRetryLogic_BackoffCalculation measures the cost of the shared
// nextBackoff computation used by both Retrier and RetryTransport.
func BenchmarkRetryLogic_BackoffCalculation(b *testing.B) {
	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retrier.nextBackoff(cfg.InitialBackoff)
	}
}
