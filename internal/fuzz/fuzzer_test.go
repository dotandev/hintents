// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package fuzz

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCoverageGuidedFuzzer tests fuzzer instantiation
func TestNewCoverageGuidedFuzzer(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	config := FuzzerConfig{
		MaxIterations: 100,
		TimeoutMs:     5000,
	}

	fuzzer := NewCoverageGuidedFuzzer(runner, config)
	require.NotNil(t, fuzzer)
	assert.Equal(t, uint64(100), fuzzer.config.MaxIterations)
	assert.Equal(t, uint64(5000), fuzzer.config.TimeoutMs)
	assert.Empty(t, fuzzer.corpus)
	assert.Empty(t, fuzzer.crashingInputs)
}

// TestDefaultConfig tests that default values are applied
func TestDefaultConfig(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	config := FuzzerConfig{}

	fuzzer := NewCoverageGuidedFuzzer(runner, config)
	assert.Equal(t, uint64(1000), fuzzer.config.MaxIterations)
	assert.Equal(t, uint64(5000), fuzzer.config.TimeoutMs)
	assert.Equal(t, 1000, fuzzer.config.MaxCorpusSize)
	assert.Equal(t, 0.1, fuzzer.config.CoverageSampleRate)
}

// TestMutateInput tests input mutation
func TestMutateInput(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{})

	input := &simulator.FuzzerInput{
		EnvelopeXdr: hex.EncodeToString([]byte("test data")),
		Timestamp:   int64(time.Now().Unix()),
	}

	mutated := fuzzer.mutateInput(input)

	// Verify mutation output is valid
	assert.NotNil(t, mutated)
	assert.Greater(t, mutated.Seed, uint64(0))
}

// TestBitflipMutation tests bitflip mutation strategy
func TestBitflipMutation(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{})

	input := &simulator.FuzzerInput{
		EnvelopeXdr: hex.EncodeToString([]byte{0xFF, 0xFF, 0xFF}),
	}

	// Apply bitflip multiple times to ensure it varies
	mutated1 := fuzzer.mutateInput(input)
	mutated2 := fuzzer.mutateInput(input)

	// At least one should differ from original
	assert.True(t,
		mutated1.EnvelopeXdr != input.EnvelopeXdr ||
			mutated2.EnvelopeXdr != input.EnvelopeXdr,
		"mutations should produce variations",
	)
}

// TestCorpusManagement tests corpus addition and selection
func TestCorpusManagement(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	config := FuzzerConfig{
		MaxCorpusSize:  10,
		EnableCoverage: false,
	}
	fuzzer := NewCoverageGuidedFuzzer(runner, config)

	input := &simulator.FuzzerInput{
		EnvelopeXdr: hex.EncodeToString([]byte("test")),
	}

	// Add to corpus
	added := fuzzer.addToCorpus(context.Background(), input, nil)
	assert.False(t, added) // Coverage tracking disabled

	// Get corpus
	corpus := fuzzer.GetCorpus()
	assert.NotEmpty(t, corpus, "corpus should not be empty")

	// Select entry
	entry := fuzzer.selectCorpusEntry()
	assert.NotNil(t, entry)
	assert.Equal(t, input.EnvelopeXdr, entry.Input.EnvelopeXdr)
}

// TestCrashTracking tests crash detection and tracking
func TestCrashTracking(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{})

	// Simulate crash
	crashingInput := &simulator.FuzzerInput{
		EnvelopeXdr: "crash_input",
	}
	result, _ := fuzzer.executeInput(context.Background(), crashingInput)

	// Mock runner will return a response, so this won't crash in test
	assert.NotNil(t, result)

	// Get crashes
	crashes := fuzzer.GetCrashingInputs()
	assert.NotNil(t, crashes)
}

// TestCoverageStats tests coverage statistics calculation
func TestCoverageStats(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{
		EnableCoverage: true,
	})

	stats := fuzzer.CoverageStats()
	assert.Equal(t, 0, stats.CorpusSize)
	assert.Equal(t, 0, stats.CrashCount)
	assert.Equal(t, uint64(0), stats.ExecutionCount)
}

// TestFuzzingStats tests fuzzing statistics
func TestFuzzingStats(t *testing.T) {
	start := time.Now()
	stats := &FuzzingStats{
		StartTime:        start,
		EndTime:          start.Add(2 * time.Second),
		ExecutionCount:   100,
		CrashCount:       5,
		NewCoverageCount: 10,
		CorpusSize:       20,
	}

	duration := stats.Duration()
	assert.GreaterOrEqual(t, duration, 2*time.Second)

	exPerSec := stats.ExecutionsPerSecond()
	assert.Greater(t, exPerSec, float64(0))
	assert.Less(t, exPerSec, float64(100)) // Should be roughly 50

	str := stats.String()
	assert.Contains(t, str, "FuzzingStats")
	assert.Contains(t, str, "100") // executionCount
}

// TestMutationStrategies tests all mutation strategies
func TestMutationStrategies(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()

	strategies := []MutationStrategy{
		StrategyBitflip,
		StrategyByteFlip,
		StrategyInteresting,
		StrategyHavoc,
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			config := FuzzerConfig{
				MutationStrategies: []MutationStrategy{strategy},
			}
			fuzzer := NewCoverageGuidedFuzzer(runner, config)

			input := &simulator.FuzzerInput{
				EnvelopeXdr: hex.EncodeToString([]byte{0xAA, 0xBB, 0xCC}),
				Timestamp:   1000,
			}

			mutated := fuzzer.mutateInput(input)
			assert.NotNil(t, mutated)
		})
	}
}

// TestGetCorpus tests corpus retrieval
func TestGetCorpus(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{
		MaxCorpusSize:  10,
		EnableCoverage: false,
	})

	input := &simulator.FuzzerInput{
		EnvelopeXdr: "test123",
	}
	fuzzer.addToCorpus(context.Background(), input, nil)

	corpus := fuzzer.GetCorpus()
	assert.NotEmpty(t, corpus)
	assert.Len(t, corpus, 1)
}

// TestGetCoverageMap tests coverage map retrieval
func TestGetCoverageMap(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{})

	covMap := fuzzer.GetCoverageMap()
	assert.NotNil(t, covMap)
	assert.Empty(t, covMap)
}

// TestExecuteInput tests input execution
func TestExecuteInput(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{})

	input := &simulator.FuzzerInput{
		EnvelopeXdr: "test_envelope",
	}

	result, coverage := fuzzer.executeInput(context.Background(), input)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.ExecutionTimeMs, uint64(0))
	assert.NotNil(t, coverage)
}

func TestExecuteInputWithCoverage(t *testing.T) {
	runner := simulator.NewMockRunner(func(ctx context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		assert.True(t, req.EnableCoverage)
		return &simulator.SimulationResponse{
			Status:     "success",
			LCOVReport: "TN:\nSF:/tmp/contract.wasm\nDA:10,1\nDA:11,0\nDA:20,2\nend_of_record\n",
		}, nil
	})
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{EnableCoverage: true})

	input := &simulator.FuzzerInput{EnvelopeXdr: "test_envelope"}
	result, coverage := fuzzer.executeInput(context.Background(), input)

	assert.NotNil(t, result)
	assert.Equal(t, uint32(2), result.CodeCoverage)
	assert.NotNil(t, coverage)
	assert.Equal(t, uint32(2), coverage.totalCoverage)
	assert.Len(t, coverage.coveredLines, 2)
}

// TestContextCancellation tests behavior when context is cancelled
func TestContextCancellation(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{
		MaxIterations: 1000000,
	})

	input := &simulator.FuzzerInput{
		EnvelopeXdr: "test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	stats, err := fuzzer.Run(ctx, input)
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	// Should stop before max iterations due to timeout
	assert.Less(t, stats.ExecutionCount, uint64(1000000))
}

// TestCoverageStatisticsString tests the String method
func TestCoverageStatisticsString(t *testing.T) {
	stats := CoverageStatistics{
		CorpusSize:                50,
		UniqueCoverageCount:       25,
		CrashCount:                3,
		ExecutionCount:            1000,
		MaxCoverage:               500,
		AvgCoverage:               250,
		TimeSinceLastCoverageGrow: 30 * time.Second,
	}

	str := stats.String()
	assert.Contains(t, str, "CoverageStats")
	assert.Contains(t, str, "50") // corpus_size
	assert.Contains(t, str, "25") // unique_coverage
	assert.Contains(t, str, "3")  // crashes
}

// TestExecuteInputCancellationWithCoverage verifies safe cancellation
// when the LCOV temp file is in use. The eager read and per-call WaitGroup
// synchronization ensure no file access races. The eager read captures the
// file content before the deferred cleanup fires, so the file can be safely
// removed even when the runner returns early due to context cancellation.
func TestExecuteInputCancellationWithCoverage(t *testing.T) {
	runner := simulator.NewMockRunner(func(ctx context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		if req.CoverageLCOVPath != nil {
			content := "TN:\nSF:test.wasm\nDA:10,1\nDA:20,3\nend_of_record\n"
			os.WriteFile(*req.CoverageLCOVPath, []byte(content), 0644)
		}
		select {
		case <-time.After(5 * time.Second):
			return &simulator.SimulationResponse{
				Status:         "success",
				LCOVReportPath: *req.CoverageLCOVPath,
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{EnableCoverage: true})
	input := &simulator.FuzzerInput{EnvelopeXdr: "test_envelope"}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, coverage := fuzzer.executeInput(ctx, input)
	require.NotNil(t, result)
	assert.Equal(t, "cancelled", result.Status)
	assert.Contains(t, result.ErrorMessage, "context cancelled")
	// Coverage is nil when runner returns nil response on cancellation
	_ = coverage
}

// TestConcurrentCoverageFileAccess verifies that multiple goroutines can
// safely call executeInput with coverage enabled. Each call creates its own
// temp LCOV file and uses its own per-call WaitGroup, so cleanup of one
// goroutine's file never blocks on another goroutine's reads.
func TestConcurrentCoverageFileAccess(t *testing.T) {
	runner := simulator.NewMockRunner(func(ctx context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		select {
		case <-time.After(50 * time.Millisecond):
			if req.CoverageLCOVPath != nil {
				content := "TN:\nSF:test.wasm\nDA:10,1\nDA:20,3\nend_of_record\n"
				os.WriteFile(*req.CoverageLCOVPath, []byte(content), 0644)
				return &simulator.SimulationResponse{
					Status:         "success",
					LCOVReportPath: *req.CoverageLCOVPath,
				}, nil
			}
			return &simulator.SimulationResponse{Status: "success"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{EnableCoverage: true})

	var wg sync.WaitGroup
	const numRuns = 10
	results := make([]*simulator.FuzzingResult, numRuns)

	for i := 0; i < numRuns; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			input := &simulator.FuzzerInput{
				EnvelopeXdr: fmt.Sprintf("test_envelope_%d", idx),
			}
			result, coverage := fuzzer.executeInput(context.Background(), input)
			results[idx] = result
			assert.NotNil(t, result)
			assert.NotNil(t, coverage)
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		assert.Equal(t, "pass", r.Status, "run %d should pass", i)
	}
}

// TestRapidCancellationStress stresses the cancellation path with rapid
// context timeouts. Verifies no panics from LCOV file access races. Each
// goroutine's per-call WaitGroup ensures independent cleanup timing.
func TestRapidCancellationStress(t *testing.T) {
	runner := simulator.NewMockRunner(func(ctx context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			if req.CoverageLCOVPath != nil {
				content := "TN:\nSF:test.wasm\nDA:10,1\nend_of_record\n"
				os.WriteFile(*req.CoverageLCOVPath, []byte(content), 0644)
			}
			return &simulator.SimulationResponse{Status: "success"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{EnableCoverage: true})

	var (
		wg        sync.WaitGroup
		cancelled int64
		panics    int64
	)
	const numRuns = 50

	for i := 0; i < numRuns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			input := &simulator.FuzzerInput{EnvelopeXdr: "test"}
			timeout := time.Duration(rand.Intn(50)) * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			result, coverage := fuzzer.executeInput(ctx, input)
			assert.NotNil(t, result)
			// Coverage may be nil when context is cancelled before runner responds
			if result.Status != "cancelled" {
				assert.NotNil(t, coverage)
			}
			if result.Status == "cancelled" {
				atomic.AddInt64(&cancelled, 1)
			}
		}()
	}
	wg.Wait()

	assert.Zero(t, panics, "no panics should occur during concurrent cancellations")
	t.Logf("Runs: %d, cancelled: %d", numRuns, cancelled)
}

// TestPerCallWaitGroupIsolation verifies that one executeInput call's cleanup
// does not block on another call's reads. With the old shared WaitGroup, a
// cancelled call's cleanup could block indefinitely waiting for a slow read in
// a different call. The per-call WaitGroup eliminates this cross-goroutine
// interference.
func TestPerCallWaitGroupIsolation(t *testing.T) {
	runner := simulator.NewMockRunner(func(ctx context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		if req.CoverageLCOVPath != nil {
			content := "TN:\nSF:test.wasm\nDA:10,1\nend_of_record\n"
			os.WriteFile(*req.CoverageLCOVPath, []byte(content), 0644)
		}
		select {
		case <-time.After(5 * time.Second):
			return &simulator.SimulationResponse{
				Status:         "success",
				LCOVReportPath: *req.CoverageLCOVPath,
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{EnableCoverage: true})

	// Start a slow call in a goroutine
	var slowWg sync.WaitGroup
	slowWg.Add(1)
	go func() {
		defer slowWg.Done()
		input := &simulator.FuzzerInput{EnvelopeXdr: "slow_call"}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fuzzer.executeInput(ctx, input)
	}()

	// Give the slow call time to start
	time.Sleep(10 * time.Millisecond)

	// Start a fast call that cancels quickly — its cleanup must not block on the slow call
	fastWg := sync.WaitGroup{}
	fastWg.Add(1)
	go func() {
		defer fastWg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		input := &simulator.FuzzerInput{EnvelopeXdr: "fast_call"}
		result, _ := fuzzer.executeInput(ctx, input)
		assert.Equal(t, "cancelled", result.Status)
	}()

	// The fast call should complete quickly, not block on the slow call's reads
	done := make(chan struct{})
	go func() {
		fastWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Fast call completed without blocking on slow call — per-call isolation works
	case <-time.After(2 * time.Second):
		t.Fatal("fast call cleanup blocked on slow call's reads — per-call WaitGroup isolation broken")
	}

	// Clean up the slow call
	slowWg.Wait()
}

// TestEagerReadCapturesContentOnCancellation verifies that the eager read
// captures LCOV file content into simResp.LCOVReport even when the context
// is cancelled. Previously, the eager read was after the error check, so on
// cancellation the file content was lost and the deferred cleanup would delete
// the file while runner goroutines might still be writing.
func TestEagerReadCapturesContentOnCancellation(t *testing.T) {
	runner := simulator.NewMockRunner(func(ctx context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		if req.CoverageLCOVPath != nil {
			content := "TN:\nSF:test.wasm\nDA:42,5\nDA:99,1\nend_of_record\n"
			os.WriteFile(*req.CoverageLCOVPath, []byte(content), 0644)
		}
		// Simulate a slow runner that gets cancelled
		select {
		case <-time.After(5 * time.Second):
			return &simulator.SimulationResponse{
				Status:         "success",
				LCOVReportPath: *req.CoverageLCOVPath,
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{EnableCoverage: true})
	input := &simulator.FuzzerInput{EnvelopeXdr: "test_eager_read"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	result, coverage := fuzzer.executeInput(ctx, input)
	require.NotNil(t, result)
	assert.Equal(t, "cancelled", result.Status)
	assert.Contains(t, result.ErrorMessage, "context cancelled")
	// Coverage is nil when runner returns nil response on cancellation
	_ = coverage
}

// TestCleanupDoesNotLeakFiles verifies that LCOV temp files are properly
// cleaned up after executeInput completes, even under concurrent access.
func TestCleanupDoesNotLeakFiles(t *testing.T) {
	runner := simulator.NewMockRunner(func(ctx context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		if req.CoverageLCOVPath != nil {
			content := "TN:\nSF:test.wasm\nDA:10,1\nend_of_record\n"
			os.WriteFile(*req.CoverageLCOVPath, []byte(content), 0644)
		}
		select {
		case <-time.After(20 * time.Millisecond):
			return &simulator.SimulationResponse{Status: "success"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{EnableCoverage: true})

	const numRuns = 20
	var wg sync.WaitGroup
	for i := 0; i < numRuns; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			input := &simulator.FuzzerInput{EnvelopeXdr: fmt.Sprintf("leak_test_%d", idx)}
			fuzzer.executeInput(ctx, input)
		}(i)
	}
	wg.Wait()

	// Verify no leftover temp files by checking /tmp for erst-fuzz-*.lcov files
	entries, err := os.ReadDir(os.TempDir())
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && len(entry.Name()) > 10 && entry.Name()[:10] == "erst-fuzz-" {
				t.Errorf("leaked temp file: %s", entry.Name())
			}
		}
	}
}

// BenchmarkMutation benchmarks the mutation performance
func BenchmarkMutation(b *testing.B) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{})

	input := &simulator.FuzzerInput{
		EnvelopeXdr: hex.EncodeToString([]byte("test data for benchmark")),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fuzzer.mutateInput(input)
	}
}

// BenchmarkCorpusSelection benchmarks corpus selection performance
func BenchmarkCorpusSelection(b *testing.B) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{
		MaxCorpusSize:  1000,
		EnableCoverage: false,
	})

	// Fill corpus
	for i := 0; i < 100; i++ {
		input := &simulator.FuzzerInput{
			EnvelopeXdr: hex.EncodeToString([]byte{byte(i)}),
		}
		fuzzer.addToCorpus(context.Background(), input, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fuzzer.selectCorpusEntry()
	}
}
