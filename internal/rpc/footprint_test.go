// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contractDataEntry builds a distinct ContractData ledger entry and its key.
func contractDataEntry(t *testing.T, seed byte, val uint64) (xdr.LedgerKey, xdr.LedgerEntry) {
	t.Helper()

	var raw [32]byte
	for i := range raw {
		raw[i] = seed
	}
	contractID := xdr.Hash(raw)

	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 1000,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.ContractDataEntry{
				Contract:   xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: (*xdr.ContractId)(&contractID)},
				Key:        xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: uint32Ptr(1)},
				Durability: xdr.ContractDataDurabilityPersistent,
				Val:        xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: uint64Ptr(val)},
			},
		},
	}

	key := ledgerKeyFromEntry(entry)
	require.NotNil(t, key)
	return *key, entry
}

// sorobanEnvelope encodes a transaction envelope declaring the given footprint.
func sorobanEnvelope(t *testing.T, readOnly, readWrite []xdr.LedgerKey) string {
	t.Helper()

	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				SourceAccount: xdr.MustMuxedAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				Fee:           100,
				SeqNum:        1,
				Ext: xdr.TransactionExt{
					V: 1,
					SorobanData: &xdr.SorobanTransactionData{
						Resources: xdr.SorobanResources{
							Footprint: xdr.LedgerFootprint{
								ReadOnly:  readOnly,
								ReadWrite: readWrite,
							},
						},
					},
				},
			},
		},
	}

	raw, err := envelope.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

// classicEnvelope encodes a transaction envelope with no Soroban data at all.
func classicEnvelope(t *testing.T) string {
	t.Helper()

	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				SourceAccount: xdr.MustMuxedAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				Fee:           100,
				SeqNum:        1,
			},
		},
	}

	raw, err := envelope.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

// metaV3 encodes a V3 transaction meta carrying the given operation changes.
// Soroban RPC returns this shape, and for a contract call it very often holds
// no ledger entry changes at all.
func metaV3(t *testing.T, changes xdr.LedgerEntryChanges) string {
	t.Helper()

	meta := xdr.TransactionMeta{
		V: 3,
		V3: &xdr.TransactionMetaV3{
			Operations: []xdr.OperationMeta{{Changes: changes}},
		},
	}

	raw, err := meta.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestExtractFootprintKeys_ReadOnlyAndReadWrite(t *testing.T) {
	roKey, _ := contractDataEntry(t, 1, 10)
	rwKey, _ := contractDataEntry(t, 2, 20)

	keys, err := ExtractFootprintKeys(sorobanEnvelope(t, []xdr.LedgerKey{roKey}, []xdr.LedgerKey{rwKey}))
	require.NoError(t, err)
	require.Len(t, keys, 2)

	encodedRO, err := EncodeLedgerKey(roKey)
	require.NoError(t, err)
	encodedRW, err := EncodeLedgerKey(rwKey)
	require.NoError(t, err)

	// Read-only keys come first; they are the ones metadata never reports and
	// the ones whose absence the host complains about.
	assert.Equal(t, encodedRO, keys[0])
	assert.Equal(t, encodedRW, keys[1])
}

func TestExtractFootprintKeys_DeduplicatesAcrossSections(t *testing.T) {
	key, _ := contractDataEntry(t, 3, 30)

	keys, err := ExtractFootprintKeys(sorobanEnvelope(t, []xdr.LedgerKey{key}, []xdr.LedgerKey{key}))
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}

func TestExtractFootprintKeys_ClassicTransactionHasNone(t *testing.T) {
	keys, err := ExtractFootprintKeys(classicEnvelope(t))
	require.NoError(t, err)
	assert.Empty(t, keys)
}

// A Soroban transaction's contract state is reported under SorobanMeta rather
// than as ledger entry changes, so this is the shape that used to leave the
// simulator replaying against an empty ledger.
func TestExtractPreTxLedgerState_SorobanMetaCarriesNoChanges(t *testing.T) {
	state, err := ExtractPreTxLedgerState(metaV3(t, xdr.LedgerEntryChanges{}))
	require.NoError(t, err)
	assert.Empty(t, state.Entries)
	assert.Empty(t, state.KnownAbsent)
}

func TestResolveReplayState_MetadataCoversFootprint(t *testing.T) {
	key, entry := contractDataEntry(t, 4, 40)
	meta := metaV3(t, xdr.LedgerEntryChanges{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: &entry},
	})

	// No RPC endpoints are configured, so reaching the network at all would
	// fail — which is the point: a fully covered footprint must not need it.
	entries, err := (&Client{}).ResolveReplayState(
		context.Background(),
		sorobanEnvelope(t, []xdr.LedgerKey{key}, nil),
		meta,
	)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestResolveReplayState_ClassicTransactionUsesMetadataAlone(t *testing.T) {
	_, entry := contractDataEntry(t, 5, 50)
	meta := metaV3(t, xdr.LedgerEntryChanges{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: &entry},
	})

	entries, err := (&Client{}).ResolveReplayState(context.Background(), classicEnvelope(t), meta)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

// The failure this whole path exists to prevent: a footprint key that metadata
// does not cover must stop the replay with a message naming it, not reach the
// simulator and come back as an opaque host error.
func TestResolveReplayState_UncoveredFootprintKeyFails(t *testing.T) {
	key, _ := contractDataEntry(t, 6, 60)

	_, err := (&Client{}).ResolveReplayState(
		context.Background(),
		sorobanEnvelope(t, []xdr.LedgerKey{key}, nil),
		metaV3(t, xdr.LedgerEntryChanges{}),
	)
	require.Error(t, err)
}

// A key the metadata proves did not exist before the transaction is not a gap
// in the replay state, so it must not be reported as one.
func TestResolveReplayState_CreatedKeyIsNotTreatedAsMissing(t *testing.T) {
	key, entry := contractDataEntry(t, 7, 70)
	meta := metaV3(t, xdr.LedgerEntryChanges{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated, Created: &entry},
	})

	entries, err := (&Client{}).ResolveReplayState(
		context.Background(),
		sorobanEnvelope(t, nil, []xdr.LedgerKey{key}),
		meta,
	)
	require.NoError(t, err)
	assert.Empty(t, entries, "an entry created by the transaction must not be seeded into its own replay")
}

func TestIncompleteFootprintError_NamesEntryTypes(t *testing.T) {
	dataKey, _ := contractDataEntry(t, 8, 80)
	encodedData, err := EncodeLedgerKey(dataKey)
	require.NoError(t, err)

	codeKey := xdr.LedgerKey{
		Type:         xdr.LedgerEntryTypeContractCode,
		ContractCode: &xdr.LedgerKeyContractCode{Hash: xdr.Hash([32]byte{9})},
	}
	encodedCode, err := EncodeLedgerKey(codeKey)
	require.NoError(t, err)

	msg := (&IncompleteFootprintError{Keys: []string{encodedData, encodedCode}}).Error()
	assert.Contains(t, msg, "1 ContractCode")
	assert.Contains(t, msg, "1 ContractData")
	assert.True(t, strings.Contains(msg, "outside of the footprint"))
}
