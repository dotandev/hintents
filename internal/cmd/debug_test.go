// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRunner implements simulator.RunnerInterface for testing
type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Run(req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
	args := m.Called(req)
	return args.Get(0).(*simulator.SimulationResponse), args.Error(1)
}

func TestDebugCommand_Setup(t *testing.T) {
	// Test that the debugCmd is properly initialized
	assert.NotNil(t, debugCmd)
	assert.Equal(t, "debug", debugCmd.Use[:5])

	// Verify flags are properly set up
	networkFlag := debugCmd.Flags().Lookup("network")
	assert.NotNil(t, networkFlag)

	rpcURLFlag := debugCmd.Flags().Lookup("rpc-url")
	assert.NotNil(t, rpcURLFlag)
}

func TestMockRunner_ImplementsInterface(t *testing.T) {
	// Verify MockRunner implements the interface
	var _ simulator.RunnerInterface = (*MockRunner)(nil)

	// Test mock functionality
	mockRunner := new(MockRunner)

	req := &simulator.SimulationRequest{
		EnvelopeXdr:   "test-envelope",
		ResultMetaXdr: "test-meta",
	}
	expectedResp := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"test-event"},
	}

	mockRunner.On("Run", req).Return(expectedResp, nil)

	// Call the mock
	resp, err := mockRunner.Run(req)

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockRunner.AssertExpectations(t)
}

func TestExtractLedgerKeys(t *testing.T) {
	// Create a dummy LedgerEntry
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: xdr.MustAddress("GCRRSYF5JBFPXHN5DCG65A4J3MUYE53QMQ4XMXZ3CNKWFJIJJTGMH6MZ"),
		},
	}

	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 1,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: key.Account.AccountId,
				Balance:   100,
			},
		},
	}

	changes := xdr.LedgerEntryChanges{
		{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryCreated,
			Created: &entry,
		},
	}

	// Create meta structure matching the SDK
	txMeta, err := xdr.NewTransactionMeta(1, xdr.TransactionMetaV1{
		TxChanges: changes,
		Operations: []xdr.OperationMeta{
			{
				Changes: changes,
			},
		},
	})
	assert.NoError(t, err)

	meta := xdr.TransactionResultMeta{
		FeeProcessing:     changes,
		TxApplyProcessing: txMeta,
		Result: xdr.TransactionResultPair{
			TransactionHash: xdr.Hash{1, 2, 3},
			Result: xdr.TransactionResult{
				FeeCharged: 100,
				Result: xdr.TransactionResultResult{
					Code:    xdr.TransactionResultCodeTxSuccess,
					Results: &[]xdr.OperationResult{},
				},
				Ext: xdr.TransactionResultExt{
					V: 0,
				},
			},
		},
	}

	// Marshal to XDR then Base64
	metaBytes, err := meta.MarshalBinary()
	assert.NoError(t, err)
	metaB64 := base64.StdEncoding.EncodeToString(metaBytes)

	// Test extraction
	keys, err := extractLedgerKeys(metaB64)
	assert.NoError(t, err)

	// We should have at least one key (the one from FeeProcessing and one from Operations)
	// Both are the same, so map should de-duplicate.
	assert.GreaterOrEqual(t, len(keys), 1)

	// Verify key matches
	keyBytes, _ := key.MarshalBinary()
	keyB64 := base64.StdEncoding.EncodeToString(keyBytes)

	found := false
	for _, k := range keys {
		if k == keyB64 {
			found = true
			break
		}
	}
	assert.True(t, found, "Key not found in extracted keys")
}
