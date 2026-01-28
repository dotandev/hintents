package cmd

import (
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
