// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"sync"
	"testing"
)

// TestMarkFailureSuccessRace verifies that concurrent calls to markHorizonFailure,
// markHorizonSuccess, markSorobanFailure, and markSorobanSuccess do not race.
// Run with: go test -race ./internal/rpc/
func TestMarkFailureSuccessRace(t *testing.T) {
	c := &Client{
		HorizonURL: "https://horizon.example.com",
		SorobanURL: "https://soroban.example.com",
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	for i := 0; i < goroutines; i++ {
		go func() { defer wg.Done(); c.markFailure(c.HorizonURL) }()
		go func() { defer wg.Done(); c.markSuccess(c.HorizonURL) }()
		go func() { defer wg.Done(); c.markFailure(c.SorobanURL) }()
		go func() { defer wg.Done(); c.markSuccess(c.SorobanURL) }()
	}

	wg.Wait()
}
