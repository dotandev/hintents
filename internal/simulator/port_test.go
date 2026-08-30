// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"testing"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortTransactionState(t *testing.T) {
	accountID := xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 100,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: accountID,
				Balance:   1000000,
				SeqNum:    xdr.SequenceNumber(100),
			},
		},
	}

	entryXDR, err := rpc.EncodeLedgerEntry(entry)
	require.NoError(t, err)

	sourceEntries := map[string]string{
		"dummyKeyXDR": entryXDR,
	}

	header := &rpc.LedgerHeaderResponse{
		Sequence:        100,
		ProtocolVersion: 20,
	}

	config := PortConfig{
		TargetSequence:        200,
		TargetProtocolVersion: 21,
		SequenceOffsets: map[string]int64{
			"GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H": 10,
		},
	}

	portedState, err := PortTransactionState(header, sourceEntries, config)
	require.NoError(t, err)
	require.NotNil(t, portedState)

	assert.Equal(t, uint32(200), portedState.Sequence)
	assert.Equal(t, uint32(21), portedState.ProtocolVersion)
	assert.Contains(t, portedState.Entries, "dummyKeyXDR")

	// Verify the ported entry
	portedEntryBytes, err := base64.StdEncoding.DecodeString(portedState.Entries["dummyKeyXDR"])
	require.NoError(t, err)

	var portedEntry xdr.LedgerEntry
	err = portedEntry.UnmarshalBinary(portedEntryBytes)
	require.NoError(t, err)

	assert.Equal(t, xdr.Uint32(200), portedEntry.LastModifiedLedgerSeq)
	assert.Equal(t, xdr.SequenceNumber(110), portedEntry.Data.Account.SeqNum)
}

func TestPortTransactionState_NilHeader(t *testing.T) {
	config := PortConfig{TargetSequence: 200}
	_, err := PortTransactionState(nil, map[string]string{}, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source header is required")
}
