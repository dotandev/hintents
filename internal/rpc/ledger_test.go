// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/errors"

	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/support/render/problem"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Ledger Entry Encoding Tests
// =============================================================================

func TestEncodeLedgerKey(t *testing.T) {
	// Create a test account key
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: accountID,
		},
	}

	encoded, err := EncodeLedgerKey(key)
	if err != nil {
		t.Fatalf("Failed to encode ledger key: %v", err)
	}

	// Verify it's valid base64
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("Encoded key is not valid base64: %v", err)
	}

	// Verify we can decode it back
	var decodedKey xdr.LedgerKey
	if unmarshalErr := decodedKey.UnmarshalBinary(decoded); unmarshalErr != nil {
		t.Fatalf("Failed to unmarshal decoded key: %v", unmarshalErr)
	}

	if decodedKey.Type != xdr.LedgerEntryTypeAccount {
		t.Errorf("Expected Account type, got %v", decodedKey.Type)
	}
}

func TestEncodeLedgerEntry(t *testing.T) {
	// Create a test account entry
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 12345,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: accountID,
				Balance:   1000000,
				SeqNum:    xdr.SequenceNumber(100),
			},
		},
	}

	encoded, err := EncodeLedgerEntry(entry)
	if err != nil {
		t.Fatalf("Failed to encode ledger entry: %v", err)
	}

	// Verify it's valid base64
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("Encoded entry is not valid base64: %v", err)
	}

	// Verify we can decode it back
	var decodedEntry xdr.LedgerEntry
	if unmarshalErr := decodedEntry.UnmarshalBinary(decoded); unmarshalErr != nil {
		t.Fatalf("Failed to unmarshal decoded entry: %v", unmarshalErr)
	}

	if decodedEntry.Data.Type != xdr.LedgerEntryTypeAccount {
		t.Errorf("Expected Account type, got %v", decodedEntry.Data.Type)
	}

	if decodedEntry.Data.Account.Balance != 1000000 {
		t.Errorf("Expected balance 1000000, got %d", decodedEntry.Data.Account.Balance)
	}
}

func TestLedgerKeyFromEntry_Account(t *testing.T) {
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	entry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: accountID,
				Balance:   1000000,
			},
		},
	}

	key := ledgerKeyFromEntry(entry)
	if key == nil {
		t.Fatal("Expected non-nil key")
		return
	}

	if key.Type != xdr.LedgerEntryTypeAccount {
		t.Errorf("Expected Account type, got %v", key.Type)
	}

	if key.Account == nil {
		t.Fatal("Expected non-nil Account key")
	}

	if key.Account.AccountId.Address() != accountID.Address() {
		t.Errorf("Account ID mismatch")
	}
}

func TestLedgerKeyFromEntry_ContractData(t *testing.T) {
	contractID := xdr.Hash([32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32})

	entry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.ContractDataEntry{
				Contract:   xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: (*xdr.ContractId)(&contractID)},
				Key:        xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: uint32Ptr(42)},
				Durability: xdr.ContractDataDurabilityPersistent,
				Val:        xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: uint64Ptr(1000)},
			},
		},
	}

	key := ledgerKeyFromEntry(entry)
	require.NotNil(t, key)
	assert.Equal(t, xdr.LedgerEntryTypeContractData, key.Type)
	require.NotNil(t, key.ContractData)
	assert.Equal(t, xdr.ContractDataDurabilityPersistent, key.ContractData.Durability)
}

func TestLedgerKeyFromEntry_ContractCodeLedger(t *testing.T) {
	codeHash := xdr.Hash([32]byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200, 210, 220, 230, 240, 250, 255, 254, 253, 252, 251, 250, 249})

	entry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.ContractCodeEntry{
				Hash: codeHash,
				Code: []byte{0x00, 0x61, 0x73, 0x6d}, // WASM magic
			},
		},
	}

	key := ledgerKeyFromEntry(entry)
	if key == nil {
		t.Fatal("Expected non-nil key")
		return
	}

	if key.Type != xdr.LedgerEntryTypeContractCode {
		t.Errorf("Expected ContractCode type, got %v", key.Type)
	}

	if key.ContractCode == nil {
		t.Fatal("Expected non-nil ContractCode key")
	}

	if key.ContractCode.Hash != codeHash {
		t.Errorf("Hash mismatch")
	}
}

func TestExtractFromChanges(t *testing.T) {
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 100,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: accountID,
				Balance:   5000000,
			},
		},
	}

	changes := xdr.LedgerEntryChanges{
		{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryCreated,
			Created: &entry,
		},
	}

	entries := make(map[string]string)
	extractFromChanges(changes, entries)

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	// Verify we can decode the entry
	for keyXDR, entryXDR := range entries {
		// Decode key
		keyBytes, err := base64.StdEncoding.DecodeString(keyXDR)
		if err != nil {
			t.Fatalf("Failed to decode key: %v", err)
		}

		var key xdr.LedgerKey
		if unmarshalErr := key.UnmarshalBinary(keyBytes); unmarshalErr != nil {
			t.Fatalf("Failed to unmarshal key: %v", unmarshalErr)
		}

		if key.Type != xdr.LedgerEntryTypeAccount {
			t.Errorf("Expected Account type, got %v", key.Type)
		}

		// Decode entry
		entryBytes, err := base64.StdEncoding.DecodeString(entryXDR)
		if err != nil {
			t.Fatalf("Failed to decode entry: %v", err)
		}

		var decodedEntry xdr.LedgerEntry
		if unmarshalErr := decodedEntry.UnmarshalBinary(entryBytes); unmarshalErr != nil {
			t.Fatalf("Failed to unmarshal entry: %v", unmarshalErr)
		}

		if decodedEntry.Data.Account.Balance != 5000000 {
			t.Errorf("Expected balance 5000000, got %d", decodedEntry.Data.Account.Balance)
		}
	}
}

func TestExtractFromChanges_MultipleTypes(t *testing.T) {
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")

	accountEntry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: accountID,
				Balance:   1000000,
			},
		},
	}

	contractID := xdr.Hash([32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32})
	contractEntry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.ContractDataEntry{
				Contract:   xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: (*xdr.ContractId)(&contractID)},
				Key:        xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: uint32Ptr(100)},
				Durability: xdr.ContractDataDurabilityPersistent,
				Val:        xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: uint64Ptr(999)},
			},
		},
	}

	changes := xdr.LedgerEntryChanges{
		{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryCreated,
			Created: &accountEntry,
		},
		{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
			Updated: &contractEntry,
		},
	}

	entries := make(map[string]string)
	extractFromChanges(changes, entries)

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}

// =============================================================================
// Ledger Header Tests
// =============================================================================

// newMockLedgerClient wraps a mockHorizonClient in a Client with AltURLs
// populated so the failover loop runs at least once in each test.
func newMockLedgerClient(mock *mockHorizonClient, network Network) *Client {
	return &Client{
		Horizon:    mock,
		Network:    network,
		HorizonURL: "mock://horizon",
		AltURLs:    []string{"mock://horizon"},
	}
}

// TestGetLedgerHeader_Success tests successful ledger header retrieval
func TestGetLedgerHeader_Success(t *testing.T) {
	closeTime := time.Now().UTC()
	expectedSequence := uint32(12345678)
	failedTxCount := int32(5)

	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	// Override LedgerDetail to return test data
	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{
			Sequence:                   int32(expectedSequence),
			Hash:                       "abc123hash",
			PrevHash:                   "prev456hash",
			ClosedAt:                   closeTime,
			ProtocolVersion:            20,
			BaseFee:                    100,
			BaseReserve:                5000000,
			MaxTxSetSize:               1000,
			TotalCoins:                 "1000000000000",
			FeePool:                    "1000000",
			HeaderXDR:                  "AAAA...",
			SuccessfulTransactionCount: 50,
			FailedTransactionCount:     &failedTxCount,
			OperationCount:             200,
		}, nil
	}

	client := newMockLedgerClient(mock, Testnet)
	ctx := context.Background()

	header, err := client.GetLedgerHeader(ctx, expectedSequence)
	require.NoError(t, err)
	require.NotNil(t, header)

	// Verify all fields
	assert.Equal(t, expectedSequence, header.Sequence)
	assert.Equal(t, "abc123hash", header.Hash)
	assert.Equal(t, "prev456hash", header.PrevHash)
	assert.Equal(t, closeTime, header.CloseTime)
	assert.Equal(t, uint32(20), header.ProtocolVersion)
	assert.Equal(t, int32(100), header.BaseFee)
	assert.Equal(t, int32(5000000), header.BaseReserve)
	assert.Equal(t, int32(1000), header.MaxTxSetSize)
	assert.Equal(t, "1000000000000", header.TotalCoins)
	assert.Equal(t, "1000000", header.FeePool)
	assert.Equal(t, "AAAA...", header.HeaderXDR)
	assert.Equal(t, int32(50), header.SuccessfulTxCount)
	assert.Equal(t, int32(5), header.FailedTxCount)
	assert.Equal(t, int32(200), header.OperationCount)
}

// TestGetLedgerHeader_NotFound tests handling of non-existent ledgers
func TestGetLedgerHeader_NotFound(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, &horizonclient.Error{
			Problem: problem.P{
				Status: 404,
				Detail: "Ledger not found",
			},
		}
	}

	client := newMockLedgerClient(mock, Testnet)
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 999999999)
	require.Error(t, err)
	assert.True(t, IsLedgerNotFound(err), "should be ledger not found error")

	var erstErr *errors.ErstError
	require.True(t, errors.As(err, &erstErr))
	assert.Equal(t, errors.ErstLedgerNotFound, erstErr.Code)
	assert.Contains(t, erstErr.Message, "not found")
}

// TestGetLedgerHeader_Archived tests handling of archived ledgers
func TestGetLedgerHeader_Archived(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, &horizonclient.Error{
			Problem: problem.P{
				Status: 410,
				Detail: "Ledger has been archived",
			},
		}
	}

	client := newMockLedgerClient(mock, Testnet)
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 1)
	require.Error(t, err)
	assert.True(t, IsLedgerArchived(err), "should be ledger archived error")

	var erstErr *errors.ErstError
	require.True(t, errors.As(err, &erstErr))
	assert.Equal(t, errors.ErstLedgerArchived, erstErr.Code)
	assert.Contains(t, erstErr.Message, "archived")
}

// TestGetLedgerHeader_RateLimit tests handling of rate limit errors
func TestGetLedgerHeader_RateLimit(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, &horizonclient.Error{
			Problem: problem.P{
				Status: 429,
				Detail: "Rate limit exceeded",
			},
		}
	}

	client := newMockLedgerClient(mock, Testnet)
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 12345)
	require.Error(t, err)
	assert.True(t, IsRateLimitError(err), "should be rate limit error")

	var erstErr *errors.ErstError
	require.True(t, errors.As(err, &erstErr))
	assert.Equal(t, errors.ErstRateLimitExceeded, erstErr.Code)
	assert.Contains(t, erstErr.Message, "rate limit")
}

// TestGetLedgerHeader_Timeout tests context timeout handling
func TestGetLedgerHeader_Timeout(t *testing.T) {
	var testCtx context.Context
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		select {
		case <-time.After(2 * time.Second):
			return hProtocol.Ledger{}, nil
		case <-testCtx.Done():
			return hProtocol.Ledger{}, testCtx.Err()
		}
	}

	client := newMockLedgerClient(mock, Testnet)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	testCtx = ctx

	_, err := client.GetLedgerHeader(ctx, 12345)
	assert.Error(t, err)
}

// TestGetLedgerHeader_GenericError tests handling of generic errors
func TestGetLedgerHeader_GenericError(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{}, errors.New("network error")
	}

	client := newMockLedgerClient(mock, Testnet)
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 12345)
	require.Error(t, err)
	assert.False(t, IsLedgerNotFound(err))
	assert.False(t, IsLedgerArchived(err))
	assert.False(t, IsRateLimitError(err))
	assert.True(t, errors.Is(err, errors.ErrRPCConnectionFailed))
}

// TestGetLedgerHeader_DifferentNetworks tests that the client works with different networks
func TestGetLedgerHeader_DifferentNetworks(t *testing.T) {
	networks := []Network{Testnet, Mainnet, Futurenet}

	for _, network := range networks {
		t.Run(string(network), func(t *testing.T) {
			mock := &mockHorizonClient{
				TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
					return hProtocol.Transaction{}, nil
				},
			}

			mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
				return hProtocol.Ledger{
					Sequence:        int32(sequence),
					Hash:            "test_hash",
					ProtocolVersion: 20,
				}, nil
			}

			client := newMockLedgerClient(mock, network)
			ctx := context.Background()

			header, err := client.GetLedgerHeader(ctx, 12345)
			require.NoError(t, err)
			assert.NotNil(t, header)
			assert.Equal(t, uint32(12345), header.Sequence)
		})
	}
}

// TestGetLedgerHeader_ContextWithDeadline tests that existing context deadlines are respected
func TestGetLedgerHeader_ContextWithDeadline(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{
			Sequence: int32(sequence),
			Hash:     "test_hash",
		}, nil
	}

	client := newMockLedgerClient(mock, Testnet)

	// Create context with deadline
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	header, err := client.GetLedgerHeader(ctx, 12345)
	require.NoError(t, err)
	assert.NotNil(t, header)
}

// TestGetLedgerHeader_ContextWithoutDeadline tests that a default timeout is added
func TestGetLedgerHeader_ContextWithoutDeadline(t *testing.T) {
	mock := &mockHorizonClient{
		TransactionDetailFunc: func(hash string) (hProtocol.Transaction, error) {
			return hProtocol.Transaction{}, nil
		},
	}

	mock.LedgerDetailFunc = func(sequence uint32) (hProtocol.Ledger, error) {
		return hProtocol.Ledger{
			Sequence: int32(sequence),
			Hash:     "test_hash",
		}, nil
	}

	client := newMockLedgerClient(mock, Testnet)

	// Create context without deadline
	ctx := context.Background()

	header, err := client.GetLedgerHeader(ctx, 12345)
	require.NoError(t, err)
	assert.NotNil(t, header)
}

func uint32Ptr(i uint32) *xdr.Uint32 {
	v := xdr.Uint32(i)
	return &v
}

func uint64Ptr(i uint64) *xdr.Uint64 {
	v := xdr.Uint64(i)
	return &v
}

// =============================================================================
// Offline Ledger Mock Tests (simulator/mock.go format)
// =============================================================================

// ledgerOverrideManifest mirrors simulator.LedgerOverrideManifest. The helpers
// below duplicate simulator/mock.go because importing that package would create
// an import cycle (simulator/port.go imports rpc).
type ledgerOverrideManifest struct {
	LedgerEntries map[string]string `json:"ledger_entries,omitempty"`
}

func loadLedgerOverrideManifest(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest ledgerOverrideManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return manifest.LedgerEntries, nil
}

func parseLedgerOverrideFlags(entries []string) (map[string]string, error) {
	overrides := make(map[string]string)
	for _, entry := range entries {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid ledger override format: %q, expected key:value", entry)
		}
		overrides[parts[0]] = parts[1]
	}
	return overrides, nil
}

func mergeLedgerOverrides(base map[string]string, overrides map[string]string) map[string]string {
	if len(overrides) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]string)
	}
	for key, value := range overrides {
		base[key] = value
	}
	return base
}

func makeContractDataLedgerPair(t *testing.T) (keyXDR, entryXDR string) {
	t.Helper()

	contractID := xdr.Hash([32]byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31})
	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 54321,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.ContractDataEntry{
				Contract:   xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: (*xdr.ContractId)(&contractID)},
				Key:        xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: uint32Ptr(7)},
				Durability: xdr.ContractDataDurabilityPersistent,
				Val:        xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: uint64Ptr(9001)},
			},
		},
	}

	key := ledgerKeyFromEntry(entry)
	require.NotNil(t, key)

	var err error
	keyXDR, err = EncodeLedgerKey(*key)
	require.NoError(t, err)
	entryXDR, err = EncodeLedgerEntry(entry)
	require.NoError(t, err)
	return keyXDR, entryXDR
}

func TestLedgerOverrideManifest_LoadAndMerge(t *testing.T) {
	keyXDR, entryXDR := makeContractDataLedgerPair(t)

	manifestPath := filepath.Join(t.TempDir(), "ledger_override.json")
	manifest := ledgerOverrideManifest{
		LedgerEntries: map[string]string{
			keyXDR: entryXDR,
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, data, 0644))

	loaded, err := loadLedgerOverrideManifest(manifestPath)
	require.NoError(t, err)
	require.Equal(t, entryXDR, loaded[keyXDR])

	flagOverrides, err := parseLedgerOverrideFlags([]string{
		"overrideKey:overrideValue",
	})
	require.NoError(t, err)

	merged := mergeLedgerOverrides(loaded, flagOverrides)
	require.Equal(t, entryXDR, merged[keyXDR])
	require.Equal(t, "overrideValue", merged["overrideKey"])
}

func TestGetLedgerEntries_OfflineMockOverrides(t *testing.T) {
	keyXDR, entryXDR := makeContractDataLedgerPair(t)

	overrides, err := parseLedgerOverrideFlags([]string{
		keyXDR + ":" + entryXDR,
	})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req GetLedgerEntriesRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		reqKeys := req.Params[0].([]interface{})
		entries := make([]LedgerEntryResult, len(reqKeys))
		for i, key := range reqKeys {
			keyStr := key.(string)
			value, ok := overrides[keyStr]
			if !ok {
				t.Fatalf("unexpected ledger key requested: %s", keyStr)
			}
			entries[i] = LedgerEntryResult{
				Key:                keyStr,
				Xdr:                value,
				LastModifiedLedger: 54321,
				LiveUntilLedger:    54400,
			}
		}

		resp := GetLedgerEntriesResponse{Jsonrpc: "2.0", ID: 1}
		resp.Result.Entries = entries
		resp.Result.LatestLedger = 54321
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	client := &Client{
		Network:      Testnet,
		HorizonURL:   server.URL,
		SorobanURL:   server.URL,
		CacheEnabled: false,
		AltURLs:      []string{server.URL},
	}

	result, err := client.GetLedgerEntries(context.Background(), []string{keyXDR})
	require.NoError(t, err)
	require.Equal(t, entryXDR, result[keyXDR])
}

func TestGetLedgerEntries_OfflineMockManifestMerge(t *testing.T) {
	baseKey, baseEntry := makeContractDataLedgerPair(t)
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	accountKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: accountID,
		},
	}
	accountKeyXDR, err := EncodeLedgerKey(accountKey)
	require.NoError(t, err)
	accountEntryXDR := buildValidEntryB64(accountKeyXDR)

	manifestPath := filepath.Join(t.TempDir(), "ledger_override.json")
	manifest := ledgerOverrideManifest{
		LedgerEntries: map[string]string{
			baseKey: baseEntry,
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, data, 0644))

	manifestEntries, err := loadLedgerOverrideManifest(manifestPath)
	require.NoError(t, err)

	flagOverrides, err := parseLedgerOverrideFlags([]string{
		accountKeyXDR + ":" + accountEntryXDR,
	})
	require.NoError(t, err)

	overrides := mergeLedgerOverrides(manifestEntries, flagOverrides)
	require.Len(t, overrides, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req GetLedgerEntriesRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		reqKeys := req.Params[0].([]interface{})
		entries := make([]LedgerEntryResult, len(reqKeys))
		for i, key := range reqKeys {
			keyStr := key.(string)
			value, ok := overrides[keyStr]
			require.True(t, ok, "missing override for key %s", keyStr)
			entries[i] = LedgerEntryResult{
				Key:                keyStr,
				Xdr:                value,
				LastModifiedLedger: 100,
				LiveUntilLedger:    200,
			}
		}

		resp := GetLedgerEntriesResponse{Jsonrpc: "2.0", ID: 1}
		resp.Result.Entries = entries
		resp.Result.LatestLedger = 100
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	client := &Client{
		Network:      Testnet,
		HorizonURL:   server.URL,
		SorobanURL:   server.URL,
		CacheEnabled: false,
		AltURLs:      []string{server.URL},
	}

	result, err := client.GetLedgerEntries(context.Background(), []string{baseKey, accountKeyXDR})
	require.NoError(t, err)
	require.Equal(t, baseEntry, result[baseKey])
	require.Equal(t, accountEntryXDR, result[accountKeyXDR])
}
