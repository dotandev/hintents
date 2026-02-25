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
	keyB64 := createTestLedgerKey(t, 1)
	err := VerifyLedgerEntryHash(keyB64, keyB64)
	assert.NoError(t, err)
}

func TestVerifyLedgerEntryHash_KeyMismatch(t *testing.T) {
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

func createTestLedgerKey(t *testing.T, seed int) string {
	t.Helper()

	var hash xdr.Hash
	for i := 0; i < len(hash); i++ {
		hash[i] = byte((seed + i) % 256)
	}

	ledgerKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractCode,
		ContractCode: &xdr.LedgerKeyContractCode{
			Hash: hash,
		},
	}

	xdrBytes, err := ledgerKey.MarshalBinary()
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(xdrBytes)
}

func BenchmarkVerifyLedgerEntryHash(b *testing.B) {
	key := createTestLedgerKey(&testing.T{}, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyLedgerEntryHash(key, key)
	}
}

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
