// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				TransactionHash: fmt.Sprintf("tx-%d", i),
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
		assert.Equal(t, DefaultTransactionTimeout, harness.TransactionTimeout)
		assert.False(t, harness.Verbose)
	})

	t.Run("respects custom worker count", func(t *testing.T) {
		harness := NewRegressionHarness(&MockRunner{}, nil, 8)
		assert.Equal(t, 8, harness.MaxWorkers)
		assert.Equal(t, DefaultTransactionTimeout, harness.TransactionTimeout)
	})

	t.Run("can set custom transaction timeout", func(t *testing.T) {
		harness := NewRegressionHarness(&MockRunner{}, nil, 4)
		harness.TransactionTimeout = 5 * time.Second
		assert.Equal(t, 5*time.Second, harness.TransactionTimeout)
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
			RunFunc: func(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error) {
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

// loadGoldenResultMeta reads the result meta XDR from a golden fixture in
// testdata/golden/.
func loadGoldenResultMeta(t *testing.T, name string) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to get current file path")
	path := filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "golden", name)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var fixture GoldenFixture
	require.NoError(t, json.Unmarshal(data, &fixture))
	require.NotEmpty(t, fixture.ResultMetaXdr, "fixture %s has no result_meta_xdr", name)

	return fixture.ResultMetaXdr
}

func TestExtractLedgerKeysFromXDR(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantEmpty bool
	}{
		{
			name:      "empty input",
			input:     "",
			wantEmpty: true,
		},
		{
			name:    "malformed base64",
			input:   "AAAAAgAA...",
			wantErr: true,
		},
		{
			name:    "valid base64 that is not transaction meta",
			input:   base64.StdEncoding.EncodeToString([]byte{0x00, 0x00, 0x00, 0x07}),
			wantErr: true,
		},
		{
			name:  "real testnet result meta",
			input: loadGoldenResultMeta(t, "testnet_255181d41330.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := extractLedgerKeysFromXDR(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, keys)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, keys)

			if tt.wantEmpty {
				assert.Empty(t, keys)
				return
			}

			require.NotEmpty(t, keys)
			assert.True(t, sort.StringsAreSorted(keys), "keys must be sorted")

			seen := make(map[string]bool, len(keys))
			for _, key := range keys {
				assert.False(t, seen[key], "duplicate key %s", key)
				seen[key] = true

				var ledgerKey xdr.LedgerKey
				assert.NoError(t, xdr.SafeUnmarshalBase64(key, &ledgerKey), "key %s is not a valid LedgerKey", key)
			}

			again, err := extractLedgerKeysFromXDR(tt.input)
			require.NoError(t, err)
			assert.Equal(t, keys, again, "output must be deterministic")
		})
	}
}

func TestRegressionTestSuite_ConcurrentAddition(t *testing.T) {
	suite := &RegressionTestSuite{
		TotalTests: 100,
		Results:    make([]RegressionTestResult, 0, 100),
	}

	// Simulate concurrent additions
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			result := RegressionTestResult{
				TransactionHash: fmt.Sprintf("tx-%d", idx),
				Status:          "pass",
			}
			suite.addResult(result)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, 100, len(suite.Results))

	seen := make(map[string]struct{}, len(suite.Results))
	for _, result := range suite.Results {
		seen[result.TransactionHash] = struct{}{}
	}

	assert.Equal(t, 100, len(seen))
}
