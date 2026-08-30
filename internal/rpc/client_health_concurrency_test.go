// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIsHealthyLocked_HighConcurrency verifies that isHealthyLocked is
// deadlock-free and race-condition-free under heavy parallel access.
// Run with: go test -race -run TestIsHealthyLocked_HighConcurrency
func TestIsHealthyLocked_HighConcurrency(t *testing.T) {
	const (
		numGoroutines = 100
		numIterations = 50
		testURL       = "https://example.stellar.org"
	)

	client := &Client{
		AltURLs:    []string{testURL},
		SorobanURL: testURL,
	}

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				switch j % 3 {
				case 0:
					_ = client.isHealthy(testURL)
				case 1:
					client.markFailure(testURL)
				case 2:
					client.markSuccess(testURL)
				}
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock detected: goroutines did not finish within 10 seconds")
	}
}

// TestIsHealthyLocked_CircuitBreakerSemantics verifies the core health logic
// through the package methods that manage failure state.
func TestIsHealthyLocked_CircuitBreakerSemantics(t *testing.T) {
	const testURL = "https://example.stellar.org"

	t.Run("healthy when no failures recorded", func(t *testing.T) {
		c := &Client{AltURLs: []string{testURL}, SorobanURL: testURL}
		assert.True(t, c.isHealthy(testURL))
	})

	t.Run("healthy when failures below threshold", func(t *testing.T) {
		c := &Client{AltURLs: []string{testURL}, SorobanURL: testURL}
		for i := 0; i < 4; i++ {
			c.markFailure(testURL)
		}
		assert.True(t, c.isHealthy(testURL))
	})

	t.Run("unhealthy when failures reach threshold within window", func(t *testing.T) {
		c := &Client{AltURLs: []string{testURL}, SorobanURL: testURL}
		for i := 0; i < 5; i++ {
			c.markFailure(testURL)
		}
		assert.False(t, c.isHealthy(testURL))
	})

	t.Run("healthy again after success resets failures", func(t *testing.T) {
		c := &Client{AltURLs: []string{testURL}, SorobanURL: testURL}
		for i := 0; i < 5; i++ {
			c.markFailure(testURL)
		}
		assert.False(t, c.isHealthy(testURL))
		c.markSuccess(testURL)
		assert.True(t, c.isHealthy(testURL))
	})

	t.Run("healthy again after circuit timeout elapses", func(t *testing.T) {
		c := &Client{AltURLs: []string{testURL}, SorobanURL: testURL}
		for i := 0; i < 5; i++ {
			c.markFailure(testURL)
		}
		c.mu.Lock()
		c.lastFailure[testURL] = time.Now().Add(-61 * time.Second)
		c.mu.Unlock()
		assert.True(t, c.isHealthy(testURL))
	})
}

// TestIsHealthyLocked_MultiURL ensures independent circuit state per URL
// under concurrent access across multiple endpoints.
func TestIsHealthyLocked_MultiURL(t *testing.T) {
	urls := []string{
		"https://node1.stellar.org",
		"https://node2.stellar.org",
		"https://node3.stellar.org",
	}

	c := &Client{AltURLs: urls, SorobanURL: urls[0]}

	for i := 0; i < 5; i++ {
		c.markFailure(urls[1])
	}
	for i := 0; i < 3; i++ {
		c.markFailure(urls[2])
	}

	var wg sync.WaitGroup
	results := make([]bool, len(urls))

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			results[idx] = c.isHealthy(u)
		}(i, url)
	}

	wg.Wait()

	assert.True(t, results[0], "node1 should be healthy (0 failures)")
	assert.False(t, results[1], "node2 should be unhealthy (5 failures, recent)")
	assert.True(t, results[2], "node3 should be healthy (3 failures, below threshold)")
}
