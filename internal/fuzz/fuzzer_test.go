// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package fuzz

import (
	"context"
	"encoding/hex"
	"os"
	"sync"
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
	defer fuzzer.cleanupCoverageTemp()

	input := &simulator.FuzzerInput{EnvelopeXdr: "test_envelope"}
	result, coverage := fuzzer.executeInput(context.Background(), input)

	assert.NotNil(t, result)
	assert.Equal(t, uint32(2), result.CodeCoverage)
	assert.NotNil(t, coverage)
	assert.Equal(t, uint32(2), coverage.totalCoverage)
	assert.Len(t, coverage.coveredLines, 2)
}

// TestCoverageTempFileReuse verifies the fuzzer creates a single LCOV temp file
// and reuses it across every iteration (instead of one per iteration), then
// removes it when the campaign ends.
func TestCoverageTempFileReuse(t *testing.T) {
	var (
		mu        sync.Mutex
		seenPaths []string
	)

	runner := simulator.NewMockRunner(func(ctx context.Context, req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
		require.True(t, req.EnableCoverage)
		require.NotNil(t, req.CoverageLCOVPath)

		// The reused temp file must exist for the duration of the run.
		_, statErr := os.Stat(*req.CoverageLCOVPath)
		assert.NoError(t, statErr, "coverage temp file should exist during execution")

		mu.Lock()
		seenPaths = append(seenPaths, *req.CoverageLCOVPath)
		mu.Unlock()

		return &simulator.SimulationResponse{
			Status:     "success",
			LCOVReport: "TN:\nSF:/tmp/contract.wasm\nDA:10,1\nDA:20,1\nend_of_record\n",
		}, nil
	})

	const iterations = 8
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{
		MaxIterations:  iterations,
		EnableCoverage: true,
	})

	seed := &simulator.FuzzerInput{
		EnvelopeXdr:   hex.EncodeToString([]byte("seed input")),
		LedgerEntries: map[string]string{},
	}

	_, err := fuzzer.Run(context.Background(), seed)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	require.Greater(t, len(seenPaths), 1, "expected multiple simulator executions")
	for _, p := range seenPaths {
		assert.Equal(t, seenPaths[0], p, "every iteration must reuse the same temp file")
	}

	// After the campaign ends the reusable temp file must be cleaned up.
	_, statErr := os.Stat(seenPaths[0])
	assert.True(t, os.IsNotExist(statErr), "coverage temp file should be removed after Run")
	assert.Empty(t, fuzzer.coverageTmpPath, "temp path should be reset after cleanup")
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

// TestGetCrashingInputsDeepCopy verifies that mutating a returned crashing input
// does not affect the fuzzer's internal state (isolation guarantee).
func TestGetCrashingInputsDeepCopy(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{})

	// Inject a crashing input directly so we have a known, stable value.
	original := &simulator.FuzzerInput{
		EnvelopeXdr: "deadbeef",
		LedgerEntries: map[string]string{
			"key1": "value1",
		},
		Args:      []string{"arg1", "arg2"},
		Timestamp: 1000,
		Seed:      42,
	}
	fuzzer.mu.Lock()
	fuzzer.crashingInputs = append(fuzzer.crashingInputs, original)
	fuzzer.mu.Unlock()

	// Retrieve a copy and mutate it.
	crashes := fuzzer.GetCrashingInputs()
	require.Len(t, crashes, 1)

	returned := crashes[0]
	returned.EnvelopeXdr = "mutated"
	returned.LedgerEntries["key1"] = "mutated_value"
	returned.LedgerEntries["new_key"] = "new_value"
	returned.Args[0] = "mutated_arg"

	// Internal state must be unchanged.
	fuzzer.mu.RLock()
	internal := fuzzer.crashingInputs[0]
	fuzzer.mu.RUnlock()

	assert.Equal(t, "deadbeef", internal.EnvelopeXdr, "EnvelopeXdr should not be mutated")
	assert.Equal(t, "value1", internal.LedgerEntries["key1"], "LedgerEntries value should not be mutated")
	assert.NotContains(t, internal.LedgerEntries, "new_key", "new key should not appear in internal map")
	assert.Equal(t, "arg1", internal.Args[0], "Args should not be mutated")
}

// TestGetCorpusDeepCopy verifies that mutating a returned corpus entry does not
// affect the fuzzer's internal corpus state (isolation guarantee).
func TestGetCorpusDeepCopy(t *testing.T) {
	runner := simulator.NewDefaultMockRunner()
	fuzzer := NewCoverageGuidedFuzzer(runner, FuzzerConfig{
		MaxCorpusSize:  10,
		EnableCoverage: false,
	})

	// Seed the corpus with a known input.
	input := &simulator.FuzzerInput{
		EnvelopeXdr: "cafebabe",
		LedgerEntries: map[string]string{
			"entry1": "original",
		},
		Args:      []string{"a", "b"},
		Timestamp: 2000,
	}
	fuzzer.addToCorpus(context.Background(), input, nil)

	corpus := fuzzer.GetCorpus()
	require.Len(t, corpus, 1)

	returned := corpus[0]
	require.NotNil(t, returned.Input)

	// Mutate the returned copy.
	returned.Input.EnvelopeXdr = "mutated_xdr"
	returned.Input.LedgerEntries["entry1"] = "mutated"
	returned.Input.LedgerEntries["extra"] = "extra_value"
	returned.Input.Args = append(returned.Input.Args, "c")

	// Internal corpus entry must be unchanged.
	fuzzer.mu.RLock()
	internalEntry := fuzzer.corpus[0]
	fuzzer.mu.RUnlock()

	assert.Equal(t, "cafebabe", internalEntry.Input.EnvelopeXdr, "EnvelopeXdr should not be mutated")
	assert.Equal(t, "original", internalEntry.Input.LedgerEntries["entry1"], "LedgerEntries should not be mutated")
	assert.NotContains(t, internalEntry.Input.LedgerEntries, "extra", "extra key should not appear in internal map")
	assert.Len(t, internalEntry.Input.Args, 2, "Args slice length should not change")
}

// TestFuzzerInputDeepCopy verifies that FuzzerInput.DeepCopy produces a fully
// independent copy with no shared underlying maps or slices.
func TestFuzzerInputDeepCopy(t *testing.T) {
	original := &simulator.FuzzerInput{
		EnvelopeXdr: "aabbcc",
		LedgerEntries: map[string]string{
			"k": "v",
		},
		Args:      []string{"x"},
		Timestamp: 999,
		Seed:      7,
	}

	cp := original.DeepCopy()

	// Values equal on creation.
	assert.Equal(t, original.EnvelopeXdr, cp.EnvelopeXdr)
	assert.Equal(t, original.LedgerEntries, cp.LedgerEntries)
	assert.Equal(t, original.Args, cp.Args)
	assert.Equal(t, original.Timestamp, cp.Timestamp)
	assert.Equal(t, original.Seed, cp.Seed)

	// Mutating the copy does not affect the original.
	cp.EnvelopeXdr = "changed"
	cp.LedgerEntries["k"] = "changed"
	cp.Args[0] = "changed"

	assert.Equal(t, "aabbcc", original.EnvelopeXdr)
	assert.Equal(t, "v", original.LedgerEntries["k"])
	assert.Equal(t, "x", original.Args[0])
}

// TestFuzzerInputDeepCopyNilFields verifies DeepCopy handles nil maps and slices safely.
func TestFuzzerInputDeepCopyNilFields(t *testing.T) {
	original := &simulator.FuzzerInput{
		EnvelopeXdr:   "test",
		LedgerEntries: nil,
		Args:          nil,
	}

	cp := original.DeepCopy()
	assert.Nil(t, cp.LedgerEntries)
	assert.Nil(t, cp.Args)
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
