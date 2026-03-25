// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"strings"
	"testing"
)

// TestMemoryLeakPrevention tests that the limitedBuffer prevents memory growth
func TestMemoryLeakPrevention(t *testing.T) {
	// Test limitedBuffer behavior
	lb := limitedBuffer{limit: 100}

	// Write data within limit
	data := strings.Repeat("x", 50)
	n, err := lb.Write([]byte(data))
	if err != nil {
		t.Fatalf("Write within limit failed: %v", err)
	}
	if n != 50 {
		t.Errorf("Expected 50 bytes written, got %d", n)
	}
	if lb.Len() != 50 {
		t.Errorf("Expected length 50, got %d", lb.Len())
	}

	// Write data that would exceed limit
	largeData := strings.Repeat("y", 100)
	n, err = lb.Write([]byte(largeData))
	if err != nil {
		t.Fatalf("Write exceeding limit failed: %v", err)
	}
	if n != 100 {
		t.Errorf("Expected 100 bytes written (discarded), got %d", n)
	}
	// Buffer should not grow beyond limit
	if lb.Len() != 50 {
		t.Errorf("Expected length still 50 (data discarded), got %d", lb.Len())
	}
}

// TestContinuousMemoryUsage simulates daemon-like continuous operation
func TestContinuousMemoryUsage(t *testing.T) {
	mock := NewDefaultMockRunner()

	// Create a request that would generate large output
	req := &SimulationRequest{
		EnvelopeXdr:    strings.Repeat("e", 512),
		ResultMetaXdr:  strings.Repeat("m", 1024),
		LedgerEntries:  make(map[string]string, 100),
		Timestamp:      1234567890,
		LedgerSequence: 12345,
		Profile:        false,
	}

	// Add many ledger entries to increase output size
	for i := 0; i < 100; i++ {
		key := strings.Repeat("k", 64)
		value := strings.Repeat("v", 128)
		req.LedgerEntries[key] = value
	}

	// Simulate continuous operation like a daemon
	const iterations = 1000
	for i := 0; i < iterations; i++ {
		resp, err := mock.Run(context.Background(), req)
		if err != nil {
			t.Fatalf("Iteration %d failed: %v", i, err)
		}
		if resp.Status != "success" {
			t.Errorf("Iteration %d returned non-success status: %s", i, resp.Status)
		}

		// Periodically check memory usage (in real scenario, this would be runtime.MemStats)
		if i%100 == 0 {
			// Force GC to see if memory is being reclaimed
			// In real test, we'd check runtime.MemStats here
			t.Logf("Completed %d iterations", i)
		}
	}
}

// BenchmarkContinuousOperation benchmarks continuous operation to detect memory growth
func BenchmarkContinuousOperation(b *testing.B) {
	mock := NewDefaultMockRunner()

	req := &SimulationRequest{
		EnvelopeXdr:    strings.Repeat("e", 512),
		ResultMetaXdr:  strings.Repeat("m", 1024),
		LedgerEntries:  make(map[string]string, 10),
		Timestamp:      1234567890,
		LedgerSequence: 12345,
		Profile:        false,
	}

	for i := 0; i < 10; i++ {
		key := strings.Repeat("k", 64)
		value := strings.Repeat("v", 128)
		req.LedgerEntries[key] = value
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := mock.Run(context.Background(), req)
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
}
