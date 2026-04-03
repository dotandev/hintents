// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"math"
	"testing"
	"time"
)

func TestFullJitterDistribution(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.JitterFraction = 0.1
	cfg.InitialBackoff = 100 * time.Millisecond
	cfg.MaxBackoff = 1 * time.Second

	retrier := NewRetrier(cfg, nil)

	// Generate many backoff values to test distribution
	const samples = 10000
	backoffs := make([]time.Duration, samples)

	for i := 0; i < samples; i++ {
		backoffs[i] = retrier.nextBackoff(cfg.InitialBackoff)
	}

	// Calculate statistics
	var sum time.Duration
	min := backoffs[0]
	max := backoffs[0]

	for _, b := range backoffs {
		sum += b
		if b < min {
			min = b
		}
		if b > max {
			max = b
		}
	}

	mean := sum / time.Duration(samples)

	// Expected range: 0 to initialBackoff * 2 * (1 + JitterFraction)
	expectedMax := time.Duration(float64(cfg.InitialBackoff) * 2 * (1.0 + cfg.JitterFraction))

	// The full-jitter algorithm draws from [0, maxJitter) so min can legitimately be
	// zero (rand.Float64() returns [0.0, 1.0)).  We verify spread via the mean and
	// unique-value checks below instead of asserting min > 0.

	if max > expectedMax {
		t.Errorf("Maximum backoff %v exceeds expected max %v", max, expectedMax)
	}

	// Mean should be roughly half of the maximum range
	expectedMean := expectedMax / 2
	if math.Abs(float64(mean-expectedMean)) > float64(expectedMean)*0.1 { // 10% tolerance
		t.Errorf("Mean backoff %v is not close to expected mean %v", mean, expectedMean)
	}

	// Test that values are distributed (not all the same)
	uniqueValues := make(map[time.Duration]bool)
	for _, b := range backoffs {
		uniqueValues[b] = true
	}

	if len(uniqueValues) < samples/10 { // At least 10% unique values
		t.Error("Jitter values are not sufficiently distributed")
	}
}

func TestExponentialBackoffWithJitter(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.JitterFraction = 0.1
	cfg.InitialBackoff = 100 * time.Millisecond
	cfg.MaxBackoff = 10 * time.Second

	retrier := NewRetrier(cfg, nil)

	// Test exponential growth with jitter
	current := cfg.InitialBackoff
	for i := 0; i < 5; i++ {
		next := retrier.nextBackoff(current)

		// With full jitter, next should be between 0 and current*2*(1+jitter)
		expectedMax := time.Duration(float64(current) * 2 * (1.0 + cfg.JitterFraction))

		if next < 0 {
			t.Errorf("Backoff should not be negative, got %v", next)
		}

		if next > expectedMax {
			t.Errorf("Backoff %v exceeds expected max %v", next, expectedMax)
		}

		// Update current for next iteration (without jitter for testing exponential growth)
		current = time.Duration(float64(current) * 2)
		if current > cfg.MaxBackoff {
			current = cfg.MaxBackoff
		}
	}
}

func TestJitterPreventsThunderingHerd(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.JitterFraction = 0.1
	cfg.InitialBackoff = 100 * time.Millisecond

	// Simulate multiple clients retrying simultaneously
	const numClients = 100
	retryTimes := make([]time.Time, numClients)

	// All clients start at the same time and get the same backoff
	baseTime := time.Now()
	for i := 0; i < numClients; i++ {
		retrier := NewRetrier(cfg, nil)
		backoff := retrier.nextBackoff(cfg.InitialBackoff)
		retryTimes[i] = baseTime.Add(backoff)
	}

	// Count how many clients retry at exactly the same time (within 1ms)
	simultaneous := make(map[int64]int)
	for _, retryTime := range retryTimes {
		bucket := retryTime.UnixMilli()
		simultaneous[bucket]++
	}

	// With full jitter, we should not have many simultaneous retries
	maxSimultaneous := 0
	for _, count := range simultaneous {
		if count > maxSimultaneous {
			maxSimultaneous = count
		}
	}

	// Allow some clustering due to timing, but not all at once
	if maxSimultaneous > numClients/2 {
		t.Errorf("Too many simultaneous retries: %d out of %d clients", maxSimultaneous, numClients)
	}

	// Test that jitter spreads retries over time
	if len(simultaneous) < numClients/10 {
		t.Errorf("Jitter not spreading retries enough: only %d unique time buckets for %d clients", len(simultaneous), numClients)
	}
}

func TestJitterFractionZero(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.JitterFraction = 0 // No jitter
	cfg.InitialBackoff = 100 * time.Millisecond

	retrier := NewRetrier(cfg, nil)

	// Without jitter, backoff should be deterministic
	backoff1 := retrier.nextBackoff(cfg.InitialBackoff)
	backoff2 := retrier.nextBackoff(cfg.InitialBackoff)

	expected := time.Duration(float64(cfg.InitialBackoff) * 2)
	if backoff1 != expected || backoff2 != expected {
		t.Errorf("Without jitter, backoff should be deterministic: got %v and %v, expected %v", backoff1, backoff2, expected)
	}
}

func TestRetryTransportJitter(t *testing.T) {
	cfg := DefaultRetryConfig()
	cfg.JitterFraction = 0.1
	cfg.InitialBackoff = 100 * time.Millisecond

	transport := NewRetryTransport(cfg, nil)

	// Test that RetryTransport also uses jitter
	backoff := transport.nextBackoff(cfg.InitialBackoff)

	expectedMax := time.Duration(float64(cfg.InitialBackoff) * 2 * (1.0 + cfg.JitterFraction))

	if backoff < 0 || backoff > expectedMax {
		t.Errorf("RetryTransport backoff %v outside expected range [0, %v]", backoff, expectedMax)
	}
}

func BenchmarkJitterCalculation(b *testing.B) {
	cfg := DefaultRetryConfig()
	retrier := NewRetrier(cfg, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		retrier.nextBackoff(cfg.InitialBackoff)
	}
}
