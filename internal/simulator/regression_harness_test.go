// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"encoding/base64"
	"fmt"
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

func TestExtractLedgerKeysFromXDR(t *testing.T) {
	t.Run("handles empty XDR", func(t *testing.T) {
		keys, err := extractLedgerKeysFromXDR("")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(keys))
	})

	t.Run("returns error for invalid base64 or corrupt XDR", func(t *testing.T) {
		_, err := extractLedgerKeysFromXDR("not-base64!@#$")
		assert.Error(t, err)

		_, err = extractLedgerKeysFromXDR("AAAAAgAA...")
		assert.Error(t, err)
	})

	t.Run("parses valid TransactionResultMeta and extracts ledger keys", func(t *testing.T) {
		accID1 := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		entry1 := xdr.LedgerEntry{
			LastModifiedLedgerSeq: 100,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: accID1,
					Balance:   1000,
					SeqNum:    10,
				},
			},
		}

		accID2 := xdr.MustAddress("GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN")
		entry2 := xdr.LedgerEntry{
			LastModifiedLedgerSeq: 101,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: accID2,
					Balance:   2000,
					SeqNum:    20,
				},
			},
		}

		changes := xdr.LedgerEntryChanges{
			{
				Type:    xdr.LedgerEntryChangeTypeLedgerEntryCreated,
				Created: &entry1,
			},
			{
				Type:    xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
				Updated: &entry2,
			},
		}

		txMeta, err := xdr.NewTransactionMeta(1, xdr.TransactionMetaV1{
			TxChanges: changes,
			Operations: []xdr.OperationMeta{
				{
					Changes: changes,
				},
			},
		})
		require.NoError(t, err)

		meta := xdr.TransactionResultMeta{
			FeeProcessing:     changes,
			TxApplyProcessing: txMeta,
		}

		metaBytes, err := meta.MarshalBinary()
		require.NoError(t, err)
		metaB64 := base64.StdEncoding.EncodeToString(metaBytes)

		keys, err := extractLedgerKeysFromXDR(metaB64)
		require.NoError(t, err)
		assert.Equal(t, 2, len(keys), "should extract both unique account ledger keys")

		for _, k := range keys {
			rawKey, err := base64.StdEncoding.DecodeString(k)
			require.NoError(t, err)
			var lk xdr.LedgerKey
			err = lk.UnmarshalBinary(rawKey)
			require.NoError(t, err)
			assert.Equal(t, xdr.LedgerEntryTypeAccount, lk.Type)
		}
	})

	t.Run("extracts keys from transaction envelope", func(t *testing.T) {
		srcID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		dstID := xdr.MustAddress("GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN")

		srcMux := xdr.MuxedAccount{
			Type:    xdr.CryptoKeyTypeKeyTypeEd25519,
			Ed25519: srcID.Ed25519,
		}
		dstMux := xdr.MuxedAccount{
			Type:    xdr.CryptoKeyTypeKeyTypeEd25519,
			Ed25519: dstID.Ed25519,
		}

		payment := xdr.PaymentOp{
			Destination: dstMux,
			Asset:       xdr.Asset{Type: xdr.AssetTypeAssetTypeNative},
			Amount:      100,
		}

		env := xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx: xdr.Transaction{
					SourceAccount: srcMux,
					Fee:           100,
					SeqNum:        1,
					Operations: []xdr.Operation{
						{
							Body: xdr.OperationBody{
								Type:      xdr.OperationTypePayment,
								PaymentOp: &payment,
							},
						},
					},
				},
			},
		}

		rawEnv, err := env.MarshalBinary()
		require.NoError(t, err)
		envB64 := base64.StdEncoding.EncodeToString(rawEnv)

		keys, err := ExtractLedgerKeysFromEnvelope(envB64)
		require.NoError(t, err)
		assert.Equal(t, 2, len(keys), "should extract source account and destination account keys")
	})
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
