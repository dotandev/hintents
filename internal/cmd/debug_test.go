// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLoadOverrideState(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantEntries int
		wantErr     bool
	}{
		{
			name: "valid override with entries",
			content: `{
				"ledger_entries": {
					"key1": "value1",
					"key2": "value2"
				}
			}`,
			wantEntries: 2,
			wantErr:     false,
		},
		{
			name: "empty ledger entries",
			content: `{
				"ledger_entries": {}
			}`,
			wantEntries: 0,
			wantErr:     false,
		},
		{
			name: "null ledger entries",
			content: `{
				"ledger_entries": null
			}`,
			wantEntries: 0,
			wantErr:     false,
		},
		{
			name:        "invalid json",
			content:     `{invalid json}`,
			wantEntries: 0,
			wantErr:     true,
		},
		{
			name: "missing ledger_entries field",
			content: `{
				"other_field": "value"
			}`,
			wantEntries: 0,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "override.json")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			entries, err := loadOverrideState(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadOverrideState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(entries) != tt.wantEntries {
				t.Errorf("loadOverrideState() got %d entries, want %d", len(entries), tt.wantEntries)
			}
		})
	}
}

func TestLoadOverrideState_FileNotFound(t *testing.T) {
	_, err := loadOverrideState("/nonexistent/path/to/file.json")
	if err == nil {
		t.Error("loadOverrideState() expected error for nonexistent file, got nil")
	}
}

func TestLoadOverrideState_RealWorldExample(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "override.json")

	override := OverrideData{
		LedgerEntries: map[string]string{
			"AAAAAAAAAAC6hsKutUTv8P4rkKBTPJIKJvhqEMH3L9sEqKnG9nT/bQ==": "AAAABgAAAAFv8F+E0D/BE04jR47s+JhGi1Q/T/yxfC8UgG88j68rAAAAAAAAAAB+SCAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA=",
			"test_account_balance": "base64_encoded_balance_data",
		},
	}

	data, err := json.MarshalIndent(override, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := loadOverrideState(tmpFile)
	if err != nil {
		t.Fatalf("loadOverrideState() unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("loadOverrideState() got %d entries, want 2", len(entries))
	}

	expectedKey := "AAAAAAAAAAC6hsKutUTv8P4rkKBTPJIKJvhqEMH3L9sEqKnG9nT/bQ=="
	if val, ok := entries[expectedKey]; !ok {
		t.Errorf("loadOverrideState() missing expected key %s", expectedKey)
	} else if val != "AAAABgAAAAFv8F+E0D/BE04jR47s+JhGi1Q/T/yxfC8UgG88j68rAAAAAAAAAAB+SCAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA=" {
		t.Errorf("loadOverrideState() wrong value for key %s", expectedKey)
	}
}

func TestOverrideData_JSONMarshaling(t *testing.T) {
	original := OverrideData{
		LedgerEntries: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded OverrideData
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.LedgerEntries) != len(original.LedgerEntries) {
		t.Errorf("decoded entries count = %d, want %d", len(decoded.LedgerEntries), len(original.LedgerEntries))
	}

	for key, val := range original.LedgerEntries {
		if decoded.LedgerEntries[key] != val {
			t.Errorf("decoded[%s] = %s, want %s", key, decoded.LedgerEntries[key], val)
		}
	}
}

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
