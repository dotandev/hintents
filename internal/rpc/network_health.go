// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"sort"
	"sync"
	"time"
)

// NodeHealthStats tracks telemetry data for a single RPC node.
type NodeHealthStats struct {
	URL            string        `json:"url"`
	TotalRequests  int64         `json:"total_requests"`
	SuccessCount   int64         `json:"success_count"`
	FailureCount   int64         `json:"failure_count"`
	TotalLatency   time.Duration `json:"-"`
	AvgLatencyMs   float64       `json:"avg_latency_ms"`
	MinLatencyMs   float64       `json:"min_latency_ms"`
	MaxLatencyMs   float64       `json:"max_latency_ms"`
	LastUpdated    time.Time     `json:"last_updated"`
	CircuitOpen    bool          `json:"circuit_open"`
	SuccessRate    float64       `json:"success_rate"`
	HealthScore    float64       `json:"health_score"`
	recentLatency  []time.Duration
	maxSamples     int
}

// HealthReport aggregates health statistics for all configured RPC nodes.
type HealthReport struct {
	Nodes       []NodeHealthStats `json:"nodes"`
	GeneratedAt time.Time         `json:"generated_at"`
	Network     string            `json:"network"`
}

// GetHealthReport returns a snapshot of health telemetry for all known RPC nodes.
func (c *Client) GetHealthReport() *HealthReport {
	if c.healthCollector == nil {
		return &HealthReport{
			Nodes:       []NodeHealthStats{},
			GeneratedAt: time.Now(),
			Network:     c.GetNetworkName(),
		}
	}

	stats := c.healthCollector.GetAllStats()

	// Include current circuit breaker state from client failure tracking.
	c.mu.RLock()
	for i := range stats {
		stats[i].CircuitOpen = !c.isHealthyLocked(stats[i].URL)
	}
	c.mu.RUnlock()

	return &HealthReport{
		Nodes:       stats,
		GeneratedAt: time.Now(),
		Network:     c.GetNetworkName(),
	}
}

// recordTelemetry records request telemetry when health collection is enabled.
func (c *Client) recordTelemetry(url string, latency time.Duration, success bool) {
	if c.healthCollector != nil {
		c.healthCollector.RecordRequest(url, latency, success)
	}
}

// HealthCollector manages background telemetry collection for RPC nodes.
type HealthCollector struct {
	mu         sync.RWMutex
	stats      map[string]*NodeHealthStats
	maxSamples int
}

const (
	defaultMaxSamples     = 100
	latencyWeight         = 0.4
	successRateWeight     = 0.6
	recentLatencySamples  = 10
)

// NewHealthCollector creates a new health collector with default settings.
func NewHealthCollector() *HealthCollector {
	return &HealthCollector{
		stats:      make(map[string]*NodeHealthStats),
		maxSamples: defaultMaxSamples,
	}
}

// RecordRequest records telemetry for a single RPC request.
func (hc *HealthCollector) RecordRequest(url string, latency time.Duration, success bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	stats, exists := hc.stats[url]
	if !exists {
		stats = &NodeHealthStats{
			URL:           url,
			MinLatencyMs:  -1,
			recentLatency: make([]time.Duration, 0, recentLatencySamples),
			maxSamples:    hc.maxSamples,
		}
		hc.stats[url] = stats
	}

	stats.TotalRequests++
	stats.LastUpdated = time.Now()

	if success {
		stats.SuccessCount++
		stats.TotalLatency += latency

		latencyMs := float64(latency.Milliseconds())
		if stats.MinLatencyMs < 0 || latencyMs < stats.MinLatencyMs {
			stats.MinLatencyMs = latencyMs
		}
		if latencyMs > stats.MaxLatencyMs {
			stats.MaxLatencyMs = latencyMs
		}

		// Track recent latencies for weighted average
		if len(stats.recentLatency) >= recentLatencySamples {
			stats.recentLatency = stats.recentLatency[1:]
		}
		stats.recentLatency = append(stats.recentLatency, latency)
	} else {
		stats.FailureCount++
	}

	// Recompute derived metrics
	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalRequests)
	}
	if stats.SuccessCount > 0 {
		stats.AvgLatencyMs = float64(stats.TotalLatency.Milliseconds()) / float64(stats.SuccessCount)
	}

	stats.HealthScore = hc.computeHealthScore(stats)
}

// computeHealthScore calculates a weighted health score for a node.
// Higher scores indicate healthier nodes.
func (hc *HealthCollector) computeHealthScore(stats *NodeHealthStats) float64 {
	if stats.TotalRequests == 0 {
		return 0.5 // neutral score for unknown nodes
	}

	// Normalize latency score (lower latency = higher score)
	// Use recent average for more responsive scoring
	var latencyScore float64
	if len(stats.recentLatency) > 0 {
		var total time.Duration
		for _, l := range stats.recentLatency {
			total += l
		}
		avgRecent := float64(total.Milliseconds()) / float64(len(stats.recentLatency))
		// Score from 0-1: 100ms or less = 1.0, 5000ms or more = 0.0
		latencyScore = 1.0 - (avgRecent / 5000.0)
		if latencyScore < 0 {
			latencyScore = 0
		}
		if latencyScore > 1 {
			latencyScore = 1
		}
	}

	// Combine success rate and latency scores
	return (stats.SuccessRate * successRateWeight) + (latencyScore * latencyWeight)
}

// GetStats returns a copy of stats for a specific URL.
func (hc *HealthCollector) GetStats(url string) *NodeHealthStats {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	stats, exists := hc.stats[url]
	if !exists {
		return nil
	}

	// Return a copy
	copy := *stats
	copy.recentLatency = nil // don't expose internal slice
	return &copy
}

// GetAllStats returns copies of all node stats.
func (hc *HealthCollector) GetAllStats() []NodeHealthStats {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make([]NodeHealthStats, 0, len(hc.stats))
	for _, stats := range hc.stats {
		copy := *stats
		copy.recentLatency = nil
		result = append(result, copy)
	}

	// Sort by health score descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].HealthScore > result[j].HealthScore
	})

	return result
}

// GetHealthiestURL returns the URL with the highest health score from provided URLs.
// If no stats exist, returns empty string.
func (hc *HealthCollector) GetHealthiestURL(urls []string) string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var bestURL string
	var bestScore float64 = -1

	for _, url := range urls {
		stats, exists := hc.stats[url]
		if !exists {
			// Unknown node gets neutral score; still consider it
			if bestScore < 0.5 {
				bestScore = 0.5
				bestURL = url
			}
			continue
		}
		if stats.HealthScore > bestScore {
			bestScore = stats.HealthScore
			bestURL = url
		}
	}

	return bestURL
}

// RankURLsByHealth returns URLs sorted by health score (healthiest first).
func (hc *HealthCollector) RankURLsByHealth(urls []string) []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	type urlScore struct {
		url   string
		score float64
	}

	scores := make([]urlScore, 0, len(urls))
	for _, url := range urls {
		stats, exists := hc.stats[url]
		score := 0.5 // neutral for unknown
		if exists {
			score = stats.HealthScore
		}
		scores = append(scores, urlScore{url: url, score: score})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	result := make([]string, len(scores))
	for i, s := range scores {
		result[i] = s.url
	}
	return result
}

// Reset clears all collected statistics.
func (hc *HealthCollector) Reset() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.stats = make(map[string]*NodeHealthStats)
}

// SetCircuitState marks whether a node's circuit breaker is open.
func (hc *HealthCollector) SetCircuitState(url string, open bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	stats, exists := hc.stats[url]
	if !exists {
		stats = &NodeHealthStats{
			URL:           url,
			MinLatencyMs:  -1,
			recentLatency: make([]time.Duration, 0, recentLatencySamples),
			maxSamples:    hc.maxSamples,
		}
		hc.stats[url] = stats
	}
	stats.CircuitOpen = open
}
