package cmd

import (
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
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

func TestDebugCommand_WithMockRunner(t *testing.T) {
	// Create mock runner
	mockRunner := new(MockRunner)
	
	// Create debug command with mock runner
	debugCmd := NewDebugCommand(mockRunner)
	
	// Verify the command was created successfully
	assert.NotNil(t, debugCmd)
	assert.Equal(t, "debug", debugCmd.Use[:5])
	
	// Verify flags are properly set up
	networkFlag := debugCmd.Flags().Lookup("network")
	assert.NotNil(t, networkFlag)
	assert.Equal(t, "mainnet", networkFlag.DefValue)
	
	rpcURLFlag := debugCmd.Flags().Lookup("rpc-url")
	assert.NotNil(t, rpcURLFlag)
	
	// This test demonstrates that the command can now be tested with a mock
	// without requiring the actual erst-sim binary
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

func TestDebugCommand_BackwardCompatibility(t *testing.T) {
	// Test that the original debugCmd still works (backward compatibility)
	assert.NotNil(t, debugCmd)
	assert.Equal(t, "debug", debugCmd.Use[:5])
	
	// Verify flags are still present
	networkFlag := debugCmd.Flags().Lookup("network")
	assert.NotNil(t, networkFlag)
	assert.Equal(t, "mainnet", networkFlag.DefValue)
	
	rpcURLFlag := debugCmd.Flags().Lookup("rpc-url")
	assert.NotNil(t, rpcURLFlag)
	"encoding/base64"
	"testing"

	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
)

func TestExtractLedgerKeys(t *testing.T) {
	// Create a dummy TransactionResultMeta
	// We'll simulate a LedgerEntryChange (Created)
	
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: xdr.MustAddress("GB7BDSZU2Y27LYNLJLVC6MMDDDPY9KKE73M5MPJ7Z7XG5J5K5M5M5M5M"),
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
			Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated,
			Created: &entry,
		},
	}

	meta := xdr.TransactionResultMeta{
		V: 0,
		Operations: changes,
		Result: xdr.TransactionResultPair{
			Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{
					Code: xdr.TransactionResultCodeTxSuccess,
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
	assert.Len(t, keys, 1)
	
	// Verify key matches
	keyBytes, _ := key.MarshalBinary()
	keyB64 := base64.StdEncoding.EncodeToString(keyBytes)
	assert.Equal(t, keyB64, keys[0])
}
