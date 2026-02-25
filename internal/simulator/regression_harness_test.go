// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegressionTestResult(t *testing.T) {
	t.Run("creates result with required fields", func(t *testing.T) {
		result := RegressionTestResult{
			TransactionHash: "abc123",
			Status:          "pass",
			EventCountMatch: true,
			TrapsMatch:      true,
		}

		assert.Equal(t, "abc123", result.TransactionHash)
		assert.Equal(t, "pass", result.Status)
		assert.True(t, result.EventCountMatch)
		assert.True(t, result.TrapsMatch)
	})

	t.Run("result can hold error message", func(t *testing.T) {
		result := RegressionTestResult{
			Status:       "error",
			ErrorMessage: "test error",
		}

		assert.Equal(t, "error", result.Status)
		assert.Equal(t, "test error", result.ErrorMessage)
	})
}

func TestRegressionTestSuite(t *testing.T) {
	t.Run("creates empty suite", func(t *testing.T) {
		suite := &RegressionTestSuite{
			TotalTests: 10,
			Results:    make([]RegressionTestResult, 0),
		}

		assert.Equal(t, 10, suite.TotalTests)
		assert.Equal(t, 0, len(suite.Results))
	})

	t.Run("adds results thread-safely", func(t *testing.T) {
		suite := &RegressionTestSuite{
			TotalTests: 5,
			Results:    make([]RegressionTestResult, 0, 5),
		}

		for i := 0; i < 5; i++ {
			result := RegressionTestResult{
				TransactionHash: "tx-" + string(rune(i)),
				Status:          "pass",
			}
			suite.addResult(result)
		}

		assert.Equal(t, 5, len(suite.Results))
	})

	t.Run("summary formats correctly", func(t *testing.T) {
		suite := &RegressionTestSuite{
			TotalTests:  10,
			PassedTests: 8,
			FailedTests: 1,
			ErrorTests:  1,
		}

		summary := suite.Summary()
		assert.Contains(t, summary, "10")
		assert.Contains(t, summary, "8")
		assert.Contains(t, summary, "80.0%")
	})

	t.Run("failed results filters correctly", func(t *testing.T) {
		suite := &RegressionTestSuite{
			Results: []RegressionTestResult{
				{TransactionHash: "tx1", Status: "pass"},
				{TransactionHash: "tx2", Status: "fail"},
				{TransactionHash: "tx3", Status: "error"},
				{TransactionHash: "tx4", Status: "pass"},
			},
		}

		failed := suite.FailedResults()
		assert.Equal(t, 2, len(failed))
		assert.Equal(t, "tx2", failed[0].TransactionHash)
		assert.Equal(t, "tx3", failed[1].TransactionHash)
	})
}

func TestNewRegressionHarness(t *testing.T) {
	t.Run("creates harness with sensible defaults", func(t *testing.T) {
		mockRunner := &MockRunner{}
		harness := NewRegressionHarness(mockRunner, nil, 0)

		assert.Equal(t, mockRunner, harness.Runner)
		assert.Equal(t, 4, harness.MaxWorkers) // Default worker count
		assert.False(t, harness.Verbose)
	})

	t.Run("respects custom worker count", func(t *testing.T) {
		harness := NewRegressionHarness(&MockRunner{}, nil, 8)
		assert.Equal(t, 8, harness.MaxWorkers)
	})
}

func TestRegressionHarness_RunRegressionTests(t *testing.T) {
	t.Run("validates count parameter", func(t *testing.T) {
		harness := NewRegressionHarness(&MockRunner{}, nil, 2)

		suite, err := harness.RunRegressionTests(context.Background(), 0, nil, 0)
		assert.Error(t, err)
		assert.Nil(t, suite)

		suite, err = harness.RunRegressionTests(context.Background(), -1, nil, 0)
		assert.Error(t, err)
		assert.Nil(t, suite)
	})

	t.Run("handles empty transaction list", func(t *testing.T) {
		harness := NewRegressionHarness(&MockRunner{}, nil, 2)

		suite, err := harness.RunRegressionTests(context.Background(), 10, nil, 0)
		assert.Error(t, err)
		assert.Nil(t, suite)
		assert.Contains(t, err.Error(), "no failed transactions found")
	})
}

func TestRegressionHarness_TestTransaction(t *testing.T) {
	t.Run("returns error when RPCClient is nil", func(t *testing.T) {
		mockRunner := &MockRunner{
			RunFunc: func(req *SimulationRequest) (*SimulationResponse, error) {
				return &SimulationResponse{Status: "error"}, nil
			},
		}
		harness := NewRegressionHarness(mockRunner, nil, 2)

		result := harness.testTransaction(context.Background(), "invalid", nil)

		// Should have an error message because RPCClient is nil
		assert.NotEmpty(t, result.ErrorMessage)
		assert.Equal(t, "error", result.Status)
	})
}

func TestExtractLedgerKeysFromXDR(t *testing.T) {
	t.Run("handles empty XDR", func(t *testing.T) {
		keys, err := extractLedgerKeysFromXDR("")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(keys))
	})

	t.Run("returns empty slice for non-empty XDR", func(t *testing.T) {
		// TODO: Implement full XDR parsing
		// For now, returns empty as placeholder
		keys, err := extractLedgerKeysFromXDR("AAAAAgAA...")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(keys))
	})
}

func TestRegressionTestSuite_ConcurrentAddition(t *testing.T) {
	suite := &RegressionTestSuite{
		TotalTests: 100,
		Results:    make([]RegressionTestResult, 0, 100),
	}

	// Simulate concurrent additions
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(idx int) {
			result := RegressionTestResult{
				TransactionHash: "tx-" + string(rune(idx)),
				Status:          "pass",
			}
			suite.addResult(result)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	assert.Equal(t, 100, len(suite.Results))
}
