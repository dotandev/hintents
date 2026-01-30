// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"testing"

	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetContractIDFromEnvelope(t *testing.T) {
	// 1. Create a dummy transaction with InvokeHostFunction
	contractID := xdr.Hash{0x01, 0x02, 0x03, 0x04} // ... padding
	scAddress := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: (*xdr.ContractId)(&contractID),
	}

	op := xdr.Operation{
		Body: xdr.OperationBody{
			Type: xdr.OperationTypeInvokeHostFunction,
			InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
				HostFunction: xdr.HostFunction{
					Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
					InvokeContract: &xdr.InvokeContractArgs{
						ContractAddress: scAddress,
						FunctionName:    "test",
						Args:            nil,
					},
				},
			},
		},
	}

	tx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				SourceAccount: xdr.MuxedAccount{
					Type:    xdr.CryptoKeyTypeKeyTypeEd25519,
					Ed25519: &xdr.Uint256{1, 2, 3, 4},
				},
				Fee:        100,
				SeqNum:     1,
				Operations: []xdr.Operation{op},
			},
		},
	}

	// Marshal to bytes then Base64
	bytes, err := tx.MarshalBinary()
	require.NoError(t, err)
	b64 := base64.StdEncoding.EncodeToString(bytes)

	// 2. Test the function
	extractedID, err := getContractIDFromEnvelope(b64)
	require.NoError(t, err)
	assert.Equal(t, contractID, *extractedID)
}

func TestInjectNewCode(t *testing.T) {
	entries := make(map[string]string)
	contractID := xdr.Hash{0xAA, 0xBB}
	newCode := []byte("mock-wasm-code")

	// 1. Inject
	err := injectNewCode(entries, contractID, newCode)
	require.NoError(t, err)

	// 2. Verify Map
	assert.Len(t, entries, 1)

	// 3. Verify Key
	expectedKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractCode,
		ContractCode: &xdr.LedgerKeyContractCode{
			Hash: contractID,
		},
	}
	keyBytes, _ := expectedKey.MarshalBinary()
	expectedKeyB64 := base64.StdEncoding.EncodeToString(keyBytes)

	valB64, ok := entries[expectedKeyB64]
	assert.True(t, ok, "Entry should exist for calculated key")

	// 4. Verify Value
	valBytes, _ := base64.StdEncoding.DecodeString(valB64)
	var entry xdr.LedgerEntry
	err = xdr.SafeUnmarshal(valBytes, &entry)
	require.NoError(t, err)

	assert.Equal(t, xdr.LedgerEntryTypeContractCode, entry.Data.Type)
	assert.Equal(t, newCode, entry.Data.ContractCode.Code)
}
