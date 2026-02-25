// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuzzerInput(t *testing.T) {
	t.Run("creates input with ledger entries", func(t *testing.T) {
		input := &FuzzerInput{
			EnvelopeXdr: "abc123",
			LedgerEntries: map[string]string{
				"key1": "value1",
			},
			Timestamp: 12345,
		}

		assert.Equal(t, "abc123", input.EnvelopeXdr)
		assert.Equal(t, 1, len(input.LedgerEntries))
		assert.Equal(t, int64(12345), input.Timestamp)
	})

	t.Run("input with arguments", func(t *testing.T) {
		input := &FuzzerInput{
			EnvelopeXdr: "xyz",
			Args:        []string{"arg1", "arg2"},
		}

		assert.Equal(t, 2, len(input.Args))
	})
}

func TestFuzzingConfig(t *testing.T) {
	t.Run("sets default values", func(t *testing.T) {
		config := FuzzingConfig{}
		harness := NewFuzzingHarness(&MockRunner{}, config)

		assert.Equal(t, 1000, int(harness.Config.MaxIterations))
		assert.Equal(t, uint64(5000), harness.Config.TimeoutMs)
		assert.Equal(t, 256*1024, harness.Config.MaxInputSize)
	})

	t.Run("respects custom values", func(t *testing.T) {
		config := FuzzingConfig{
			MaxIterations: 5000,
			TimeoutMs:     10000,
			MaxInputSize:  1024,
		}
		harness := NewFuzzingHarness(&MockRunner{}, config)

		assert.Equal(t, uint64(5000), harness.Config.MaxIterations)
		assert.Equal(t, uint64(10000), harness.Config.TimeoutMs)
		assert.Equal(t, 1024, harness.Config.MaxInputSize)
	})
}

func TestNewFuzzingHarness(t *testing.T) {
	t.Run("creates harness with runner", func(t *testing.T) {
		mockRunner := &MockRunner{}
		config := FuzzingConfig{MaxIterations: 100}

		harness := NewFuzzingHarness(mockRunner, config)

		assert.NotNil(t, harness)
		assert.Equal(t, mockRunner, harness.Runner)
		assert.Equal(t, uint64(100), harness.Config.MaxIterations)
		assert.Equal(t, 0, len(harness.Results))
		assert.Equal(t, 0, len(harness.CrashingInputs))
	})
}

func TestFuzzingHarness_Fuzz(t *testing.T) {
	t.Run("validates base input", func(t *testing.T) {
		mockRunner := &MockRunner{}
		harness := NewFuzzingHarness(mockRunner, FuzzingConfig{})

		results, crashes, err := harness.Fuzz(nil)

		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Nil(t, crashes)
	})

	t.Run("respects iteration count", func(t *testing.T) {
		mockRunner := &MockRunner{
			RunFunc: func(req *SimulationRequest) (*SimulationResponse, error) {
				return &SimulationResponse{Status: "success"}, nil
			},
		}
		config := FuzzingConfig{MaxIterations: 10}
		harness := NewFuzzingHarness(mockRunner, config)

		input := &FuzzerInput{
			EnvelopeXdr:   "abc123",
			LedgerEntries: make(map[string]string),
		}

		results, _, err := harness.Fuzz(input)

		require.NoError(t, err)
		assert.Equal(t, 10, len(results))
	})
}

func TestFuzzingHarness_FuzzXDR(t *testing.T) {
	t.Run("validates XDR encoding", func(t *testing.T) {
		mockRunner := &MockRunner{}
		harness := NewFuzzingHarness(mockRunner, FuzzingConfig{})

		result, err := harness.FuzzXDR("not-hex")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("validates empty XDR", func(t *testing.T) {
		mockRunner := &MockRunner{}
		harness := NewFuzzingHarness(mockRunner, FuzzingConfig{})

		result, err := harness.FuzzXDR("")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("validates input size", func(t *testing.T) {
		mockRunner := &MockRunner{}
		config := FuzzingConfig{MaxInputSize: 10}
		harness := NewFuzzingHarness(mockRunner, config)

		// Create XDR string larger than limit
		largeXDR := ""
		for i := 0; i < 20; i++ {
			largeXDR += "ab"
		}

		result, err := harness.FuzzXDR(largeXDR)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("accepts valid XDR", func(t *testing.T) {
		mockRunner := &MockRunner{
			RunFunc: func(req *SimulationRequest) (*SimulationResponse, error) {
				return &SimulationResponse{Status: "success"}, nil
			},
		}
		harness := NewFuzzingHarness(mockRunner, FuzzingConfig{})

		validXDR := hex.EncodeToString([]byte("test data"))
		result, err := harness.FuzzXDR(validXDR)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "pass", result.Status)
	})
}

func TestFuzzingHarness_MutateInput(t *testing.T) {
	mockRunner := &MockRunner{}
	harness := NewFuzzingHarness(mockRunner, FuzzingConfig{})

	t.Run("creates new instance on mutation", func(t *testing.T) {
		original := &FuzzerInput{
			EnvelopeXdr: "abc123",
			Timestamp:   1000,
			LedgerEntries: map[string]string{
				"key1": hex.EncodeToString([]byte("value1")),
			},
		}

		mutated := harness.mutateInput(original, 42)

		assert.NotEqual(t, mutated.EnvelopeXdr, mutated.Seed)
		assert.Equal(t, uint64(42), mutated.Seed)
	})

	t.Run("preserves ledger entry structure", func(t *testing.T) {
		original := &FuzzerInput{
			EnvelopeXdr:   "test",
			LedgerEntries: make(map[string]string),
			Args:          []string{},
		}

		mutated := harness.mutateInput(original, 1)

		assert.NotNil(t, mutated.LedgerEntries)
		assert.NotNil(t, mutated.Args)
	})
}

func TestFuzzingHarness_MutateHexString(t *testing.T) {
	mockRunner := &MockRunner{}
	harness := NewFuzzingHarness(mockRunner, FuzzingConfig{})

	t.Run("mutates valid hex string", func(t *testing.T) {
		original := hex.EncodeToString([]byte("test"))
		rng := rand.New(rand.NewSource(12345))

		mutated := harness.mutateHexString(original, rng)

		// Should be different (with very high probability)
		assert.NotEqual(t, original, mutated)

		// Should still be valid hex
		_, err := hex.DecodeString(mutated)
		assert.NoError(t, err)
	})

	t.Run("handles invalid hex gracefully", func(t *testing.T) {
		invalid := "not-hex"
		rng := rand.New(rand.NewSource(1))

		mutated := harness.mutateHexString(invalid, rng)

		// Should return original if can't decode
		assert.Equal(t, invalid, mutated)
	})
}

func TestFuzzingHarness_TestFuzzerInput(t *testing.T) {
	t.Run("reports successful execution", func(t *testing.T) {
		mockRunner := &MockRunner{
			RunFunc: func(req *SimulationRequest) (*SimulationResponse, error) {
				return &SimulationResponse{Status: "success"}, nil
			},
		}
		harness := NewFuzzingHarness(mockRunner, FuzzingConfig{})

		input := &FuzzerInput{
			EnvelopeXdr:   "abc123",
			LedgerEntries: make(map[string]string),
			Seed:          42,
		}

		result := harness.testFuzzerInput(input)

		assert.Equal(t, "pass", result.Status)
		assert.Equal(t, uint64(42), result.Seed)
	})

	t.Run("detects errors", func(t *testing.T) {
		mockRunner := &MockRunner{
			RunFunc: func(req *SimulationRequest) (*SimulationResponse, error) {
				return &SimulationResponse{Status: "error", Error: "test error"}, nil
			},
		}
		harness := NewFuzzingHarness(mockRunner, FuzzingConfig{})

		input := &FuzzerInput{
			EnvelopeXdr:   "xyz",
			LedgerEntries: make(map[string]string),
		}

		result := harness.testFuzzerInput(input)

		assert.Equal(t, "error", result.Status)
		assert.NotEmpty(t, result.ErrorMessage)
	})
}

func TestFuzzingHarness_CorpusCoverage(t *testing.T) {
	t.Run("calculates statistics", func(t *testing.T) {
		harness := &FuzzingHarness{
			Results: []FuzzingResult{
				{Status: "pass", CodeCoverage: 50},
				{Status: "pass", CodeCoverage: 60},
				{Status: "crash", CodeCoverage: 40},
			},
		}

		avgCov, passes, crashes := harness.CorpusCoverage()

		assert.Equal(t, uint32(50), avgCov) // (50+60+40)/3
		assert.Equal(t, 2, passes)
		assert.Equal(t, 1, crashes)
	})

	t.Run("handles empty results", func(t *testing.T) {
		harness := &FuzzingHarness{
			Results: []FuzzingResult{},
		}

		avgCov, passes, crashes := harness.CorpusCoverage()

		assert.Equal(t, uint32(0), avgCov)
		assert.Equal(t, 0, passes)
		assert.Equal(t, 0, crashes)
	})
}

func TestFuzzingHarness_Summary(t *testing.T) {
	t.Run("formats summary correctly", func(t *testing.T) {
		harness := &FuzzingHarness{
			Results: []FuzzingResult{
				{Status: "pass"},
				{Status: "pass"},
				{Status: "crash"},
			},
			CrashingInputs: []FuzzerInput{{}, {}},
		}

		summary := harness.Summary()

		assert.Contains(t, summary, "3")       // Total Runs
		assert.Contains(t, summary, "2")       // Passes and Unique Crashes
		assert.Contains(t, summary, "Summary") // Summary header
	})
}
