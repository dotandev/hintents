// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// PerfBaseline represents the baseline metrics for performance regression tests
type PerfBaseline struct {
	Version    int                       `json:"version"`
	Updated    string                    `json:"updated"`
	Benchmarks map[string]BenchmarkEntry `json:"benchmarks"`
	Threshold  float64                   `json:"threshold_percent"`
}

// BenchmarkEntry represents target metrics for a single benchmark
type BenchmarkEntry struct {
	TargetNsPerOp     int64 `json:"target_ns_per_op"`
	TargetBytesPerOp  int64 `json:"target_bytes_per_op"`
	TargetAllocsPerOp int64 `json:"target_allocs_per_op"`
}

// BenchmarkResult holds measured benchmark results
type BenchmarkResult struct {
	Name         string
	NsPerOp      int64
	BytesPerOp   int64
	AllocsPerOp  int64
	Iterations   int
	MemoryUsedMB float64
}

// loadBaseline loads the performance baseline from the JSON file
func loadBaseline(t *testing.T) *PerfBaseline {
	t.Helper()

	baselinePath := filepath.Join("perf_baseline.json")
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("failed to load baseline: %v", err)
	}

	var baseline PerfBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("failed to parse baseline: %v", err)
	}

	return &baseline
}

// checkRegression compares result against baseline and returns an error if regression exceeds threshold
func checkRegression(result *BenchmarkResult, entry *BenchmarkEntry, threshold float64) error {
	nsDeviation := float64(result.NsPerOp-entry.TargetNsPerOp) / float64(entry.TargetNsPerOp) * 100
	bytesDeviation := float64(result.BytesPerOp-entry.TargetBytesPerOp) / float64(entry.TargetBytesPerOp) * 100
	allocsDeviation := float64(result.AllocsPerOp-entry.TargetAllocsPerOp) / float64(entry.TargetAllocsPerOp) * 100

	var regressions []string

	if nsDeviation > threshold {
		regressions = append(regressions, fmt.Sprintf(
			"execution time regression: %.1f%% (got %d ns/op, baseline %d ns/op)",
			nsDeviation, result.NsPerOp, entry.TargetNsPerOp,
		))
	}

	if bytesDeviation > threshold {
		regressions = append(regressions, fmt.Sprintf(
			"memory usage regression: %.1f%% (got %d B/op, baseline %d B/op)",
			bytesDeviation, result.BytesPerOp, entry.TargetBytesPerOp,
		))
	}

	if allocsDeviation > threshold {
		regressions = append(regressions, fmt.Sprintf(
			"allocation regression: %.1f%% (got %d allocs/op, baseline %d allocs/op)",
			allocsDeviation, result.AllocsPerOp, entry.TargetAllocsPerOp,
		))
	}

	if len(regressions) > 0 {
		return fmt.Errorf("[%s] performance regression detected:\n  %s",
			result.Name, strings.Join(regressions, "\n  "))
	}

	return nil
}

// endlessLoopScenario simulates an endless loop contract execution
func endlessLoopScenario(ctx context.Context, runner RunnerInterface) (*SimulationResponse, error) {
	req := &SimulationRequest{
		EnvelopeXdr:    strings.Repeat("e", 512),
		ResultMetaXdr:  strings.Repeat("m", 1024),
		LedgerSequence: 12345,
		LedgerEntries:  make(map[string]string, 10),
		Profile:        false,
	}

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d-%s", i, strings.Repeat("k", 56))
		req.LedgerEntries[key] = strings.Repeat("v", 128)
	}

	return runner.Run(ctx, req)
}

// memoryHogScenario simulates a memory-intensive contract execution
func memoryHogScenario(ctx context.Context, runner RunnerInterface) (*SimulationResponse, error) {
	req := &SimulationRequest{
		EnvelopeXdr:    strings.Repeat("e", 2048),
		ResultMetaXdr:  strings.Repeat("m", 4096),
		LedgerSequence: 12345,
		LedgerEntries:  make(map[string]string, 100),
		Profile:        false,
	}

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d-%s", i, strings.Repeat("k", 56))
		req.LedgerEntries[key] = strings.Repeat("v", 512)
	}

	return runner.Run(ctx, req)
}

// combinedWorkloadScenario simulates a mixed workload
func combinedWorkloadScenario(ctx context.Context, runner RunnerInterface) (*SimulationResponse, error) {
	req := &SimulationRequest{
		EnvelopeXdr:    strings.Repeat("e", 1024),
		ResultMetaXdr:  strings.Repeat("m", 2048),
		LedgerSequence: 12345,
		LedgerEntries:  make(map[string]string, 50),
		Profile:        false,
		AuthTraceOpts: &AuthTraceOptions{
			Enabled:              true,
			TraceCustomContracts: true,
		},
	}

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key-%d-%s", i, strings.Repeat("k", 56))
		req.LedgerEntries[key] = strings.Repeat("v", 256)
	}

	return runner.Run(ctx, req)
}

// BenchmarkEndlessLoopPerf benchmarks the EndlessLoop scenario
func BenchmarkEndlessLoopPerf(b *testing.B) {
	runner := createPerfMockRunner()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := endlessLoopScenario(ctx, runner)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMemoryHogPerf benchmarks the MemoryHog scenario
func BenchmarkMemoryHogPerf(b *testing.B) {
	runner := createPerfMockRunner()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := memoryHogScenario(ctx, runner)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCombinedWorkloadPerf benchmarks the combined workload scenario
func BenchmarkCombinedWorkloadPerf(b *testing.B) {
	runner := createPerfMockRunner()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := combinedWorkloadScenario(ctx, runner)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestPerfRegressionEndlessLoop tests for performance regression in EndlessLoop scenario
func TestPerfRegressionEndlessLoop(t *testing.T) {
	baseline := loadBaseline(t)
	entry, ok := baseline.Benchmarks["EndlessLoop"]
	if !ok {
		t.Fatal("EndlessLoop baseline not found")
	}

	result := runBenchmarkScenario(t, "EndlessLoop", endlessLoopScenario)

	if err := checkRegression(result, &entry, baseline.Threshold); err != nil {
		t.Error(err)
	} else {
		t.Logf("EndlessLoop: %d ns/op, %d B/op, %d allocs/op (within threshold)",
			result.NsPerOp, result.BytesPerOp, result.AllocsPerOp)
	}
}

// TestPerfRegressionMemoryHog tests for performance regression in MemoryHog scenario
func TestPerfRegressionMemoryHog(t *testing.T) {
	baseline := loadBaseline(t)
	entry, ok := baseline.Benchmarks["MemoryHog"]
	if !ok {
		t.Fatal("MemoryHog baseline not found")
	}

	result := runBenchmarkScenario(t, "MemoryHog", memoryHogScenario)

	if err := checkRegression(result, &entry, baseline.Threshold); err != nil {
		t.Error(err)
	} else {
		t.Logf("MemoryHog: %d ns/op, %d B/op, %d allocs/op (within threshold)",
			result.NsPerOp, result.BytesPerOp, result.AllocsPerOp)
	}
}

// TestPerfRegressionCombinedWorkload tests for performance regression in combined workload
func TestPerfRegressionCombinedWorkload(t *testing.T) {
	baseline := loadBaseline(t)
	entry, ok := baseline.Benchmarks["CombinedWorkload"]
	if !ok {
		t.Fatal("CombinedWorkload baseline not found")
	}

	result := runBenchmarkScenario(t, "CombinedWorkload", combinedWorkloadScenario)

	if err := checkRegression(result, &entry, baseline.Threshold); err != nil {
		t.Error(err)
	} else {
		t.Logf("CombinedWorkload: %d ns/op, %d B/op, %d allocs/op (within threshold)",
			result.NsPerOp, result.BytesPerOp, result.AllocsPerOp)
	}
}

// runBenchmarkScenario executes a scenario multiple times and returns averaged metrics
func runBenchmarkScenario(
	t *testing.T,
	name string,
	scenario func(context.Context, RunnerInterface) (*SimulationResponse, error),
) *BenchmarkResult {
	t.Helper()

	runner := createPerfMockRunner()
	ctx := context.Background()

	const warmupIterations = 10
	const measureIterations = 100

	// Warmup
	for i := 0; i < warmupIterations; i++ {
		_, _ = scenario(ctx, runner)
	}

	// Force GC before measurement
	runtime.GC()

	var memStatsBefore, memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	start := time.Now()
	for i := 0; i < measureIterations; i++ {
		_, err := scenario(ctx, runner)
		if err != nil {
			t.Fatalf("scenario failed: %v", err)
		}
	}
	elapsed := time.Since(start)

	runtime.ReadMemStats(&memStatsAfter)

	nsPerOp := elapsed.Nanoseconds() / int64(measureIterations)
	bytesAllocated := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
	bytesPerOp := int64(bytesAllocated) / int64(measureIterations)
	allocsPerOp := int64(memStatsAfter.Mallocs-memStatsBefore.Mallocs) / int64(measureIterations)

	return &BenchmarkResult{
		Name:         name,
		NsPerOp:      nsPerOp,
		BytesPerOp:   bytesPerOp,
		AllocsPerOp:  allocsPerOp,
		Iterations:   measureIterations,
		MemoryUsedMB: float64(bytesAllocated) / 1024 / 1024,
	}
}

// createPerfMockRunner creates a mock runner that simulates realistic response processing
func createPerfMockRunner() *MockRunner {
	return &MockRunner{
		RunFunc: func(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error) {
			// Simulate processing overhead by creating realistic response
			events := make([]string, 20)
			for i := 0; i < 20; i++ {
				events[i] = fmt.Sprintf("event-%d-%s", i, strings.Repeat("d", 50))
			}

			diagnosticEvents := make([]DiagnosticEvent, 10)
			contractID := strings.Repeat("c", 56)
			for i := 0; i < 10; i++ {
				diagnosticEvents[i] = DiagnosticEvent{
					EventType:                "contract",
					ContractID:               &contractID,
					Topics:                   []string{"topic1", "topic2"},
					Data:                     strings.Repeat("d", 100),
					InSuccessfulContractCall: true,
				}
			}

			return &SimulationResponse{
				Status:           "success",
				Events:           events,
				DiagnosticEvents: diagnosticEvents,
				Logs:             []string{"log1", "log2", "log3"},
				BudgetUsage: &BudgetUsage{
					CPUInstructions:    1000000,
					MemoryBytes:        5000000,
					OperationsCount:    len(req.LedgerEntries),
					CPULimit:           100000000,
					MemoryLimit:        50000000,
					CPUUsagePercent:    1.0,
					MemoryUsagePercent: 10.0,
				},
			}, nil
		},
		CloseFunc: func() error { return nil },
	}
}
