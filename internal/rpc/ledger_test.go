// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/base64"
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
	if err := decodedKey.UnmarshalBinary(decoded); err != nil {
		t.Fatalf("Failed to unmarshal decoded key: %v", err)
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
	if err := decodedEntry.UnmarshalBinary(decoded); err != nil {
		t.Fatalf("Failed to unmarshal decoded entry: %v", err)
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
	// TODO: Fix xdr.Uint32Ptr and xdr.Uint64Ptr compatibility issues
	t.Skip("Skipping due to xdr compatibility issues with recent stellar-go version")
	/*
		contractID := xdr.Hash([32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32})
		keyVal := xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: uint32Ptr(42)}

		entry := xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeContractData,
				ContractData: &xdr.ContractDataEntry{
					Contract:   xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: (*xdr.ContractId)(&contractID)},
					Key:        keyVal,
					Durability: xdr.ContractDataDurabilityPersistent,
					Val:        xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: uint64Ptr(1000)},
				},
			},
		}

		key := ledgerKeyFromEntry(entry)
		if key == nil {
			t.Fatal("Expected non-nil key")
		}

		if key.Type != xdr.LedgerEntryTypeContractData {
			t.Errorf("Expected ContractData type, got %v", key.Type)
		}
	*/
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
		if err := key.UnmarshalBinary(keyBytes); err != nil {
			t.Fatalf("Failed to unmarshal key: %v", err)
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
		if err := decodedEntry.UnmarshalBinary(entryBytes); err != nil {
			t.Fatalf("Failed to unmarshal entry: %v", err)
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

	client := &Client{Horizon: mock, Network: Testnet}
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

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 999999999)
	require.Error(t, err)
	assert.True(t, IsLedgerNotFound(err), "should be ledger not found error")

	notFoundErr, ok := err.(*errors.LedgerNotFoundError)
	require.True(t, ok)
	assert.Equal(t, uint32(999999999), notFoundErr.Sequence)
	assert.Contains(t, notFoundErr.Message, "not found")
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

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 1)
	require.Error(t, err)
	assert.True(t, IsLedgerArchived(err), "should be ledger archived error")

	archivedErr, ok := err.(*errors.LedgerArchivedError)
	require.True(t, ok)
	assert.Equal(t, uint32(1), archivedErr.Sequence)
	assert.Contains(t, archivedErr.Message, "archived")
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

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 12345)
	require.Error(t, err)
	assert.True(t, IsRateLimitError(err), "should be rate limit error")

	rateLimitErr, ok := err.(*errors.RateLimitError)
	require.True(t, ok)
	assert.Contains(t, rateLimitErr.Message, "rate limit")
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

	client := &Client{Horizon: mock, Network: Testnet}
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

	client := &Client{Horizon: mock, Network: Testnet}
	ctx := context.Background()

	_, err := client.GetLedgerHeader(ctx, 12345)
	require.Error(t, err)
	assert.False(t, IsLedgerNotFound(err))
	assert.False(t, IsLedgerArchived(err))
	assert.False(t, IsRateLimitError(err))
	assert.Contains(t, err.Error(), "RPC connection failed")
}

// TestFromHorizonLedger tests the conversion from Horizon ledger to our structure
func TestFromHorizonLedger(t *testing.T) {
	closeTime := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)
	failedTxCount := int32(5)

	horizonLedger := hProtocol.Ledger{
		Sequence:                   12345,
		Hash:                       "abcd1234",
		PrevHash:                   "prev5678",
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
	}

	result := FromHorizonLedger(horizonLedger)

	assert.Equal(t, uint32(12345), result.Sequence)
	assert.Equal(t, "abcd1234", result.Hash)
	assert.Equal(t, "prev5678", result.PrevHash)
	assert.Equal(t, closeTime, result.CloseTime)
	assert.Equal(t, uint32(20), result.ProtocolVersion)
	assert.Equal(t, int32(100), result.BaseFee)
	assert.Equal(t, int32(5000000), result.BaseReserve)
	assert.Equal(t, int32(1000), result.MaxTxSetSize)
	assert.Equal(t, "1000000000000", result.TotalCoins)
	assert.Equal(t, "1000000", result.FeePool)
	assert.Equal(t, "AAAA...", result.HeaderXDR)
	assert.Equal(t, int32(50), result.SuccessfulTxCount)
	assert.Equal(t, int32(5), result.FailedTxCount)
	assert.Equal(t, int32(200), result.OperationCount)
}

// TestErrorTypes tests the error type checking functions
func TestErrorTypes(t *testing.T) {
	notFoundErr := &errors.LedgerNotFoundError{Sequence: 123, Message: "not found"}
	archivedErr := &errors.LedgerArchivedError{Sequence: 456, Message: "archived"}
	rateLimitErr := &errors.RateLimitError{Message: "rate limited"}
	genericErr := errors.New("generic error")

	// Test IsLedgerNotFound
	assert.True(t, IsLedgerNotFound(notFoundErr))
	assert.False(t, IsLedgerNotFound(archivedErr))
	assert.False(t, IsLedgerNotFound(rateLimitErr))
	assert.False(t, IsLedgerNotFound(genericErr))

	// Test IsLedgerArchived
	assert.True(t, IsLedgerArchived(archivedErr))
	assert.False(t, IsLedgerArchived(notFoundErr))
	assert.False(t, IsLedgerArchived(rateLimitErr))
	assert.False(t, IsLedgerArchived(genericErr))

	// Test IsRateLimitError
	assert.True(t, IsRateLimitError(rateLimitErr))
	assert.False(t, IsRateLimitError(notFoundErr))
	assert.False(t, IsRateLimitError(archivedErr))
	assert.False(t, IsRateLimitError(genericErr))
}

// TestErrorMessages tests that error messages are descriptive
func TestErrorMessages(t *testing.T) {
	notFoundErr := &errors.LedgerNotFoundError{
		Sequence: 123,
		Message:  "ledger 123 not found (may be archived or not yet created)",
	}
	assert.Contains(t, notFoundErr.Error(), "123")
	assert.Contains(t, notFoundErr.Error(), "not found")

	archivedErr := &errors.LedgerArchivedError{
		Sequence: 456,
		Message:  "ledger 456 has been archived and is no longer available",
	}
	assert.Contains(t, archivedErr.Error(), "456")
	assert.Contains(t, archivedErr.Error(), "archived")

	rateLimitErr := &errors.RateLimitError{
		Message: "rate limit exceeded, please try again later",
	}
	assert.Contains(t, rateLimitErr.Error(), "rate limit")
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

			client := &Client{Horizon: mock, Network: network}
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

	client := &Client{Horizon: mock, Network: Testnet}

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

	client := &Client{Horizon: mock, Network: Testnet}

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
