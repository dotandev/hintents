// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyLedgerEntryHash_ValidKey(t *testing.T) {
	// Create a valid LedgerKey for a contract data entry
	contractID := xdr.ContractId([32]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
	})

	contractAddr := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}

	symbol := xdr.ScSymbol("COUNTER")
	keyVal := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &symbol,
	}

	ledgerKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract:   contractAddr,
			Key:        keyVal,
			Durability: xdr.ContractDataDurability(xdr.ContractDataDurabilityPersistent),
		},
	}

	// Marshal to XDR and encode to base64
	xdrBytes, err := ledgerKey.MarshalBinary()
	require.NoError(t, err)
	keyB64 := base64.StdEncoding.EncodeToString(xdrBytes)

	// Verify the hash
	err = VerifyLedgerEntryHash(keyB64, keyB64)
	assert.NoError(t, err)
}

func TestVerifyLedgerEntryHash_KeyMismatch(t *testing.T) {
	// Create two different keys
	key1 := createTestLedgerKey(t, 1)
	key2 := createTestLedgerKey(t, 2)

	err := VerifyLedgerEntryHash(key1, key2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key mismatch")
}

func TestVerifyLedgerEntryHash_InvalidBase64(t *testing.T) {
	invalidB64 := "not-valid-base64!!!"

	err := VerifyLedgerEntryHash(invalidB64, invalidB64)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode")
}

func TestVerifyLedgerEntryHash_InvalidXDR(t *testing.T) {
	// Valid base64 but invalid XDR content
	invalidXDR := base64.StdEncoding.EncodeToString([]byte("invalid xdr data"))

	err := VerifyLedgerEntryHash(invalidXDR, invalidXDR)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestVerifyLedgerEntries_AllValid(t *testing.T) {
	key1 := createTestLedgerKey(t, 1)
	key2 := createTestLedgerKey(t, 2)
	key3 := createTestLedgerKey(t, 3)

	requestedKeys := []string{key1, key2, key3}
	returnedEntries := map[string]string{
		key1: "value1",
		key2: "value2",
		key3: "value3",
	}

	err := VerifyLedgerEntries(requestedKeys, returnedEntries)
	assert.NoError(t, err)
}

func TestVerifyLedgerEntries_MissingKey(t *testing.T) {
	key1 := createTestLedgerKey(t, 1)
	key2 := createTestLedgerKey(t, 2)

	requestedKeys := []string{key1, key2}
	returnedEntries := map[string]string{
		key1: "value1",
		// key2 is missing
	}

	err := VerifyLedgerEntries(requestedKeys, returnedEntries)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in response")
}

func TestVerifyLedgerEntries_EmptyRequest(t *testing.T) {
	err := VerifyLedgerEntries([]string{}, map[string]string{})
	assert.NoError(t, err)
}

func TestVerifyLedgerEntries_NilMap(t *testing.T) {
	key1 := createTestLedgerKey(t, 1)

	err := VerifyLedgerEntries([]string{key1}, nil)
	assert.Error(t, err)
}

func TestVerifyLedgerEntryHash_DifferentKeyTypes(t *testing.T) {
	tests := []struct {
		name      string
		createKey func() xdr.LedgerKey
	}{
		{
			name: "Account key",
			createKey: func() xdr.LedgerKey {
				accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
				return xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeAccount,
					Account: &xdr.LedgerKeyAccount{
						AccountId: accountID,
					},
				}
			},
		},
		{
			name: "ContractCode key",
			createKey: func() xdr.LedgerKey {
				codeHash := xdr.Hash([32]byte{
					0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8,
					0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf, 0xe0,
					0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8,
					0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xf0,
				})
				return xdr.LedgerKey{
					Type:         xdr.LedgerEntryTypeContractCode,
					ContractCode: &xdr.LedgerKeyContractCode{Hash: codeHash},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.createKey()
			xdrBytes, err := key.MarshalBinary()
			require.NoError(t, err)
			keyB64 := base64.StdEncoding.EncodeToString(xdrBytes)

			err = VerifyLedgerEntryHash(keyB64, keyB64)
			assert.NoError(t, err)
		})
	}
}

func TestVerifyLedgerEntries_LargeSet(t *testing.T) {
	const numKeys = 100

	requestedKeys := make([]string, numKeys)
	returnedEntries := make(map[string]string, numKeys)

	for i := 0; i < numKeys; i++ {
		key := createTestLedgerKey(t, i)
		requestedKeys[i] = key
		returnedEntries[key] = "value"
	}

	err := VerifyLedgerEntries(requestedKeys, returnedEntries)
	assert.NoError(t, err)
}

func TestVerifyLedgerEntryHash_EmptyKey(t *testing.T) {
	err := VerifyLedgerEntryHash("", "")
	assert.Error(t, err)
}

func TestVerifyLedgerEntryHash_WhitespaceKey(t *testing.T) {
	err := VerifyLedgerEntryHash("   ", "   ")
	assert.Error(t, err)
}

// createTestLedgerKey creates a unique test ledger key with the given seed
func createTestLedgerKey(t *testing.T, seed int) string {
	t.Helper()

	// Create a unique contract ID based on seed
	var contractID xdr.ContractId
	for i := 0; i < 32; i++ {
		contractID[i] = byte((seed + i) % 256)
	}

	contractAddr := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}

	symbol := xdr.ScSymbol("COUNTER")
	keyVal := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &symbol,
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

// BenchmarkVerifyLedgerEntryHash benchmarks the hash verification performance
func BenchmarkVerifyLedgerEntryHash(b *testing.B) {
	key := createTestLedgerKey(&testing.T{}, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyLedgerEntryHash(key, key)
	}
}

// BenchmarkVerifyLedgerEntries benchmarks verification of multiple entries
func BenchmarkVerifyLedgerEntries(b *testing.B) {
	sizes := []int{10, 50, 100, 500}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			requestedKeys := make([]string, size)
			returnedEntries := make(map[string]string, size)

			for i := 0; i < size; i++ {
				key := createTestLedgerKey(&testing.T{}, i)
				requestedKeys[i] = key
				returnedEntries[key] = "value"
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = VerifyLedgerEntries(requestedKeys, returnedEntries)
			}
		})
	}
}
