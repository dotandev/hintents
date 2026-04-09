// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// verification_test.go validates the ledger entry verification logic used to
// ensure RPC responses contain the exact entries the client requested. This
// guards against data corruption, incomplete responses, and tampered results.
package rpc

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerifyLedgerEntryHash_ValidKey verifies that hash verification succeeds
// when sent and received keys are identical (happy path).
func TestVerifyLedgerEntryHash_ValidKey(t *testing.T) {
	// Create a valid LedgerKey for a contract data entry
	contractID := xdr.ContractId([32]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
	})

	contractIDVal := xdr.ContractId(contractID)
	contractAddr := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractIDVal,
	}

	sym := xdr.ScSymbol(fmt.Sprintf("COUNTER_%d", seed))
	keyVal := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &sym,
	}

	// Create the Key
	ledgerKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract:   contractAddr,
			Key:        keyVal,
			Durability: xdr.ContractDataDurability(xdr.ContractDataDurabilityPersistent),
		},
	}

	keyBytes, err := ledgerKey.MarshalBinary()
	require.NoError(t, err)
	keyB64 := base64.StdEncoding.EncodeToString(keyBytes)

	// Create the Entry
	valSym := xdr.ScSymbol("VALUE")
	valVal := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &valSym,
	}

	ledgerEntry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 12345,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.ContractDataEntry{
				Contract:   contractAddr,
				Key:        keyVal,
				Durability: xdr.ContractDataDurability(xdr.ContractDataDurabilityPersistent),
				Val:        valVal,
			},
		},
		Ext: xdr.LedgerEntryExt{V: 0},
	}

	entryBytes, err := ledgerEntry.MarshalBinary()
	require.NoError(t, err)
	entryB64 := base64.StdEncoding.EncodeToString(entryBytes)

	return keyB64, entryB64
}

func TestVerifyLedgerEntryHash_ValidKey(t *testing.T) {
	keyB64, entryB64 := createTestLedgerData(t, 1)

	result := LedgerEntryResult{
		Key: keyB64,
		Xdr: entryB64,
	}

	err := VerifyLedgerEntryHash(keyB64, result)
	assert.NoError(t, err)
}

// TestVerifyLedgerEntryHash_KeyMismatch ensures verification detects when the
// returned key differs from the requested key, catching tampered RPC responses.
func TestVerifyLedgerEntryHash_KeyMismatch(t *testing.T) {
	key1, _ := createTestLedgerData(t, 1)
	key2, entry2 := createTestLedgerData(t, 2)

	result := LedgerEntryResult{
		Key: key2,
		Xdr: entry2,
	}

	err := VerifyLedgerEntryHash(key1, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key mismatch")
}

// TestVerifyLedgerEntryHash_InvalidBase64 validates error handling when the
// base64 encoding is malformed, protecting against garbage data in responses.
func TestVerifyLedgerEntryHash_InvalidBase64(t *testing.T) {
	invalidB64 := "not-valid-base64!!!"

	err := VerifyLedgerEntryHash("AAAA", LedgerEntryResult{Key: "AAAA", Xdr: invalidB64})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode")
}

// TestVerifyLedgerEntryHash_InvalidXDR ensures the function rejects data that
// decodes from base64 but contains invalid XDR binary (corrupted ledger entries).
func TestVerifyLedgerEntryHash_InvalidXDR(t *testing.T) {
	invalidXDR := base64.StdEncoding.EncodeToString([]byte("invalid xdr data"))
	key, _ := createTestLedgerData(t, 1)

	err := VerifyLedgerEntryHash(key, LedgerEntryResult{Key: key, Xdr: invalidXDR})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal ledger entry")
}

// TestVerifyLedgerEntries_AllValid validates that batch verification succeeds
// when all requested keys are present in the returned entries.
func TestVerifyLedgerEntries_AllValid(t *testing.T) {
	k1, e1 := createTestLedgerData(t, 1)
	k2, e2 := createTestLedgerData(t, 2)
	k3, e3 := createTestLedgerData(t, 3)

	requestedKeys := []string{k1, k2, k3}
	returnedEntries := []LedgerEntryResult{
		{Key: k1, Xdr: e1},
		{Key: k2, Xdr: e2},
		{Key: k3, Xdr: e3},
	}

	err := VerifyLedgerEntries(requestedKeys, returnedEntries)
	assert.NoError(t, err)
}

// TestVerifyLedgerEntries_MissingKey ensures verification fails when an RPC
// response omits a requested entry, detecting incomplete responses.
func TestVerifyLedgerEntries_MissingKey(t *testing.T) {
	k1, e1 := createTestLedgerData(t, 1)
	k2, _ := createTestLedgerData(t, 2)

	requestedKeys := []string{k1, k2}
	returnedEntries := []LedgerEntryResult{
		{Key: k1, Xdr: e1},
		// k2 is missing
	}

	err := VerifyLedgerEntries(requestedKeys, returnedEntries)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in response")
}

// TestVerifyLedgerEntries_EmptyRequest validates that requesting zero keys
// succeeds without errors (edge case for empty batches).
func TestVerifyLedgerEntries_EmptyRequest(t *testing.T) {
	err := VerifyLedgerEntries([]string{}, []LedgerEntryResult{})
	assert.NoError(t, err)
}

// TestVerifyLedgerEntries_NilMap validates that a nil response map is rejected,
// guarding against nil pointer dereferences.
func TestVerifyLedgerEntries_NilMap(t *testing.T) {
	key1 := createTestLedgerKey(t, 1)

	err := VerifyLedgerEntries([]string{key1}, nil)
	assert.Error(t, err)
}

// TestVerifyLedgerEntryHash_DifferentKeyTypes validates that verification works
// across different Stellar ledger key types (Account, ContractCode, etc.).
func TestVerifyLedgerEntryHash_DifferentKeyTypes(t *testing.T) {
	tests := []struct {
		name       string
		createData func() (string, string)
	}{
		{
			name: "Account key",
			createData: func() (string, string) {
				accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				key := xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeAccount,
					Account: &xdr.LedgerKeyAccount{
						AccountId: accountID,
					},
				}

				entry := xdr.LedgerEntry{
					LastModifiedLedgerSeq: 123,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: accountID,
							Balance:   1000,
						},
					},
				}

				kb, _ := key.MarshalBinary()
				eb, _ := entry.MarshalBinary()
				return base64.StdEncoding.EncodeToString(kb), base64.StdEncoding.EncodeToString(eb)
			},
		},
		{
			name: "ContractCode key",
			createData: func() (string, string) {
				codeHash := xdr.Hash([32]byte{
					0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8,
					0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf, 0xe0,
					0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8,
					0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xf0,
				})
				key := xdr.LedgerKey{
					Type:         xdr.LedgerEntryTypeContractCode,
					ContractCode: &xdr.LedgerKeyContractCode{Hash: codeHash},
				}

				entry := xdr.LedgerEntry{
					LastModifiedLedgerSeq: 456,
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeContractCode,
						ContractCode: &xdr.ContractCodeEntry{
							Hash: codeHash,
							Code: []byte{1, 2, 3, 4},
						},
					},
				}

				kb, _ := key.MarshalBinary()
				eb, _ := entry.MarshalBinary()
				return base64.StdEncoding.EncodeToString(kb), base64.StdEncoding.EncodeToString(eb)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, entry := tt.createData()

			result := LedgerEntryResult{
				Key: key,
				Xdr: entry,
			}

			err := VerifyLedgerEntryHash(key, result)
			assert.NoError(t, err)
		})
	}
}

// TestVerifyLedgerEntries_LargeSet tests batch verification with 100 entries to
// ensure correctness at scale.
func TestVerifyLedgerEntries_LargeSet(t *testing.T) {
	const numKeys = 100

	requestedKeys := make([]string, numKeys)
	returnedEntries := make([]LedgerEntryResult, numKeys)

	for i := 0; i < numKeys; i++ {
		k, e := createTestLedgerData(t, i)
		requestedKeys[i] = k
		returnedEntries[i] = LedgerEntryResult{Key: k, Xdr: e}
	}

	err := VerifyLedgerEntries(requestedKeys, returnedEntries)
	assert.NoError(t, err)
}

// TestVerifyLedgerEntryHash_EmptyKey validates that empty string keys are rejected.
func TestVerifyLedgerEntryHash_EmptyKey(t *testing.T) {
	err := VerifyLedgerEntryHash("", LedgerEntryResult{})
	assert.Error(t, err)
}

// TestVerifyLedgerEntryHash_WhitespaceKey validates that whitespace-only keys
// are rejected as invalid input.
func TestVerifyLedgerEntryHash_WhitespaceKey(t *testing.T) {
	err := VerifyLedgerEntryHash("   ", LedgerEntryResult{Key: "   "})
	assert.Error(t, err)
}

// createTestLedgerKey builds a deterministic, unique base64-encoded ContractData
// ledger key from the given seed. Each seed produces a distinct contract ID,
// making it suitable for testing verification logic with multiple entries.
func createTestLedgerKey(t *testing.T, seed int) string {
	t.Helper()

	// Create a unique contract ID based on seed
	var contractIDHash xdr.Hash
	for i := 0; i < 32; i++ {
		contractIDHash[i] = byte((seed + i) % 256)
	}

	contractIDVal := xdr.ContractId(contractIDHash)
	contractAddr := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractIDVal,
	}

	sym := xdr.ScSymbol("COUNTER")
	keyVal := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &sym,
	}

	ledgerKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract:   contractAddr,
			Key:        keyVal,
			Durability: xdr.ContractDataDurability(xdr.ContractDataDurabilityPersistent),
		},
	}

	xdrBytes, err := ledgerKey.MarshalBinary()
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(xdrBytes)
}

// BenchmarkVerifyLedgerEntryHash measures single-key verification throughput.
// Run with: go test -bench=BenchmarkVerifyLedgerEntryHash ./internal/rpc
func BenchmarkVerifyLedgerEntryHash(b *testing.B) {
	key, entry := createTestLedgerData(&testing.T{}, 1)
	result := LedgerEntryResult{Key: key, Xdr: entry}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyLedgerEntryHash(key, result)
	}
}

// BenchmarkVerifyLedgerEntries measures batch verification across varying set
// sizes (10, 50, 100, 500) to profile scaling behavior.
// Run with: go test -bench=BenchmarkVerifyLedgerEntries ./internal/rpc
func BenchmarkVerifyLedgerEntries(b *testing.B) {
	sizes := []int{10, 50, 100, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			requestedKeys := make([]string, size)
			returnedEntries := make([]LedgerEntryResult, size)

			for i := 0; i < size; i++ {
				k, e := createTestLedgerData(&testing.T{}, i)
				requestedKeys[i] = k
				returnedEntries[i] = LedgerEntryResult{Key: k, Xdr: e}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = VerifyLedgerEntries(requestedKeys, returnedEntries)
			}
		})
	}
}
