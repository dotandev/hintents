// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCollector_RecordRequest(t *testing.T) {
	hc := NewHealthCollector()

	// Record successful requests
	hc.RecordRequest("http://node1.example.com", 100*time.Millisecond, true)
	hc.RecordRequest("http://node1.example.com", 150*time.Millisecond, true)
	hc.RecordRequest("http://node1.example.com", 200*time.Millisecond, true)

	stats := hc.GetStats("http://node1.example.com")
	require.NotNil(t, stats)
	assert.Equal(t, int64(3), stats.TotalRequests)
	assert.Equal(t, int64(3), stats.SuccessCount)
	assert.Equal(t, int64(0), stats.FailureCount)
	assert.Equal(t, float64(1.0), stats.SuccessRate)
	assert.Equal(t, float64(100), stats.MinLatencyMs)
	assert.Equal(t, float64(200), stats.MaxLatencyMs)
	assert.InDelta(t, 150.0, stats.AvgLatencyMs, 0.1)
}

func TestHealthCollector_RecordFailures(t *testing.T) {
	hc := NewHealthCollector()

	hc.RecordRequest("http://node1.example.com", 100*time.Millisecond, true)
	hc.RecordRequest("http://node1.example.com", 0, false)
	hc.RecordRequest("http://node1.example.com", 0, false)

	stats := hc.GetStats("http://node1.example.com")
	require.NotNil(t, stats)
	assert.Equal(t, int64(3), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.SuccessCount)
	assert.Equal(t, int64(2), stats.FailureCount)
	assert.InDelta(t, 0.333, stats.SuccessRate, 0.01)
}

func TestHealthCollector_HealthScore(t *testing.T) {
	hc := NewHealthCollector()

	// Node 1: High success rate, low latency
	for i := 0; i < 10; i++ {
		hc.RecordRequest("http://fast-node.example.com", 50*time.Millisecond, true)
	}

	// Node 2: High success rate, high latency
	for i := 0; i < 10; i++ {
		hc.RecordRequest("http://slow-node.example.com", 2000*time.Millisecond, true)
	}

	// Node 3: Low success rate
	for i := 0; i < 5; i++ {
		hc.RecordRequest("http://unreliable-node.example.com", 100*time.Millisecond, true)
	}
	for i := 0; i < 5; i++ {
		hc.RecordRequest("http://unreliable-node.example.com", 0, false)
	}

	fastStats := hc.GetStats("http://fast-node.example.com")
	slowStats := hc.GetStats("http://slow-node.example.com")
	unreliableStats := hc.GetStats("http://unreliable-node.example.com")

	// Fast node should have highest score
	assert.Greater(t, fastStats.HealthScore, slowStats.HealthScore)
	assert.Greater(t, fastStats.HealthScore, unreliableStats.HealthScore)

	// Slow but reliable node should score higher than unreliable node
	assert.Greater(t, slowStats.HealthScore, unreliableStats.HealthScore)
}

func TestHealthCollector_GetHealthiestURL(t *testing.T) {
	hc := NewHealthCollector()

	// Set up nodes with different health
	for i := 0; i < 10; i++ {
		hc.RecordRequest("http://best.example.com", 50*time.Millisecond, true)
		hc.RecordRequest("http://medium.example.com", 500*time.Millisecond, true)
		hc.RecordRequest("http://worst.example.com", 1000*time.Millisecond, true)
	}
	// Add some failures to worst
	for i := 0; i < 5; i++ {
		hc.RecordRequest("http://worst.example.com", 0, false)
	}

	urls := []string{
		"http://best.example.com",
		"http://medium.example.com",
		"http://worst.example.com",
	}

	healthiest := hc.GetHealthiestURL(urls)
	assert.Equal(t, "http://best.example.com", healthiest)
}

func TestHealthCollector_GetHealthiestURL_UnknownNodes(t *testing.T) {
	hc := NewHealthCollector()

	// Only set up one node
	hc.RecordRequest("http://known.example.com", 50*time.Millisecond, true)

	urls := []string{
		"http://known.example.com",
		"http://unknown.example.com",
	}

	// Known node with good health should be preferred over unknown
	healthiest := hc.GetHealthiestURL(urls)
	assert.Equal(t, "http://known.example.com", healthiest)
}

func TestHealthCollector_RankURLsByHealth(t *testing.T) {
	hc := NewHealthCollector()

	// Set up nodes with different health
	for i := 0; i < 10; i++ {
		hc.RecordRequest("http://best.example.com", 50*time.Millisecond, true)
		hc.RecordRequest("http://medium.example.com", 500*time.Millisecond, true)
		hc.RecordRequest("http://worst.example.com", 1000*time.Millisecond, true)
	}
	// Add failures to worst
	for i := 0; i < 5; i++ {
		hc.RecordRequest("http://worst.example.com", 0, false)
	}

	urls := []string{
		"http://worst.example.com",
		"http://medium.example.com",
		"http://best.example.com",
	}

	ranked := hc.RankURLsByHealth(urls)
	assert.Equal(t, "http://best.example.com", ranked[0])
	assert.Equal(t, "http://worst.example.com", ranked[2])
}

func TestHealthCollector_GetAllStats(t *testing.T) {
	hc := NewHealthCollector()

	hc.RecordRequest("http://node1.example.com", 100*time.Millisecond, true)
	hc.RecordRequest("http://node2.example.com", 50*time.Millisecond, true)

	allStats := hc.GetAllStats()
	assert.Len(t, allStats, 2)

	// Should be sorted by health score descending
	assert.Equal(t, "http://node2.example.com", allStats[0].URL) // lower latency = higher score
}

func TestHealthCollector_Reset(t *testing.T) {
	hc := NewHealthCollector()

	hc.RecordRequest("http://node1.example.com", 100*time.Millisecond, true)
	assert.NotNil(t, hc.GetStats("http://node1.example.com"))

	hc.Reset()
	assert.Nil(t, hc.GetStats("http://node1.example.com"))
	assert.Empty(t, hc.GetAllStats())
}

func TestHealthCollector_SetCircuitState(t *testing.T) {
	hc := NewHealthCollector()

	hc.SetCircuitState("http://node1.example.com", true)
	stats := hc.GetStats("http://node1.example.com")
	require.NotNil(t, stats)
	assert.True(t, stats.CircuitOpen)

	hc.SetCircuitState("http://node1.example.com", false)
	stats = hc.GetStats("http://node1.example.com")
	assert.False(t, stats.CircuitOpen)
}

func TestHealthCollector_ConcurrentAccess(t *testing.T) {
	hc := NewHealthCollector()
	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				hc.RecordRequest("http://node.example.com", time.Duration(id)*time.Millisecond, true)
			}
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				hc.GetStats("http://node.example.com")
				hc.GetAllStats()
				hc.GetHealthiestURL([]string{"http://node.example.com"})
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	stats := hc.GetStats("http://node.example.com")
	require.NotNil(t, stats)
	assert.Equal(t, int64(1000), stats.TotalRequests)
}

func TestClient_GetHealthReport(t *testing.T) {
	client, err := NewClient(WithNetwork(Testnet))
	require.NoError(t, err)

	// Record some telemetry
	client.recordTelemetry("http://node1.example.com", 100*time.Millisecond, true)
	client.recordTelemetry("http://node2.example.com", 200*time.Millisecond, true)
	client.recordTelemetry("http://node1.example.com", 0, false)

	report := client.GetHealthReport()
	assert.NotNil(t, report)
	assert.Equal(t, "testnet", report.Network)
	assert.Len(t, report.Nodes, 2)
	assert.False(t, report.GeneratedAt.IsZero())
}

func TestClient_GetHealthReport_NoCollector(t *testing.T) {
	// Test with a manually created client without collector
	client := &Client{
		Network:         Testnet,
		healthCollector: nil,
	}

	report := client.GetHealthReport()
	assert.NotNil(t, report)
	assert.Empty(t, report.Nodes)
}

func TestNodeHealthStats_MinLatency(t *testing.T) {
	hc := NewHealthCollector()

	// First request sets initial min
	hc.RecordRequest("http://node.example.com", 500*time.Millisecond, true)
	stats := hc.GetStats("http://node.example.com")
	assert.Equal(t, float64(500), stats.MinLatencyMs)

	// Lower latency updates min
	hc.RecordRequest("http://node.example.com", 100*time.Millisecond, true)
	stats = hc.GetStats("http://node.example.com")
	assert.Equal(t, float64(100), stats.MinLatencyMs)

	// Higher latency doesn't change min
	hc.RecordRequest("http://node.example.com", 300*time.Millisecond, true)
	stats = hc.GetStats("http://node.example.com")
	assert.Equal(t, float64(100), stats.MinLatencyMs)
}
