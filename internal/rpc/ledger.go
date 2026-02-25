// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"encoding/base64"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerEntryPair represents a ledger key-entry pair for simulation
type LedgerEntryPair struct {
	KeyXDR   string
	EntryXDR string
}

// LedgerHeaderResponse contains essential ledger header information
// needed for transaction replay simulation. This structure provides
// all the metadata required to recreate the blockchain state at the
// time a transaction was executed.
type LedgerHeaderResponse struct {
	// Core ledger identifiers
	Sequence uint32 // Ledger sequence number
	Hash     string // Ledger hash (hex-encoded SHA-256)
	PrevHash string // Previous ledger hash

	// Timing information
	CloseTime time.Time // When the ledger closed

	// Protocol and network parameters
	ProtocolVersion uint32 // Stellar protocol version
	BaseFee         int32  // Base fee in stroops (1 stroop = 0.0000001 XLM)
	BaseReserve     int32  // Base reserve in stroops
	MaxTxSetSize    int32  // Maximum transaction set size

	// Network state
	TotalCoins string // Total lumens in circulation
	FeePool    string // Fee pool amount

	// XDR data
	HeaderXDR string // Base64-encoded LedgerHeader XDR

	// Transaction statistics
	SuccessfulTxCount int32 // Number of successful transactions
	FailedTxCount     int32 // Number of failed transactions
	OperationCount    int32 // Total operations in ledger
}

// FromHorizonLedger converts a Horizon ledger response to our internal structure.
// This provides a clean abstraction layer between the Horizon API and our
// internal representation, making it easier to add alternative data sources
// (like Soroban RPC) in the future.
func FromHorizonLedger(hl hProtocol.Ledger) *LedgerHeaderResponse {
	failedTxCount := int32(0)
	if hl.FailedTransactionCount != nil {
		failedTxCount = *hl.FailedTransactionCount
	}

	return &LedgerHeaderResponse{
		Sequence:          uint32(hl.Sequence),
		Hash:              hl.Hash,
		PrevHash:          hl.PrevHash,
		CloseTime:         hl.ClosedAt,
		ProtocolVersion:   uint32(hl.ProtocolVersion),
		BaseFee:           hl.BaseFee,
		BaseReserve:       hl.BaseReserve,
		MaxTxSetSize:      hl.MaxTxSetSize,
		TotalCoins:        hl.TotalCoins,
		FeePool:           hl.FeePool,
		HeaderXDR:         hl.HeaderXDR,
		SuccessfulTxCount: hl.SuccessfulTransactionCount,
		FailedTxCount:     failedTxCount,
		OperationCount:    hl.OperationCount,
	}
}

// EncodeLedgerKey encodes a LedgerKey to base64 XDR
func EncodeLedgerKey(key xdr.LedgerKey) (string, error) {
	xdrBytes, err := key.MarshalBinary()
	if err != nil {
		return "", errors.WrapMarshalFailed(err)
	}
	return base64.StdEncoding.EncodeToString(xdrBytes), nil
}

// EncodeLedgerEntry encodes a LedgerEntry to base64 XDR
func EncodeLedgerEntry(entry xdr.LedgerEntry) (string, error) {
	xdrBytes, err := entry.MarshalBinary()
	if err != nil {
		return "", errors.WrapMarshalFailed(err)
	}
	return base64.StdEncoding.EncodeToString(xdrBytes), nil
}

// ExtractLedgerEntriesFromMeta extracts ledger entries from TransactionResultMeta
// This provides the state that was present when the transaction executed
func ExtractLedgerEntriesFromMeta(resultMetaXDR string) (map[string]string, error) {
	// Decode the result meta XDR
	metaBytes, err := base64.StdEncoding.DecodeString(resultMetaXDR)
	if err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "result meta")
	}

	var resultMeta xdr.TransactionResultMeta
	if err := resultMeta.UnmarshalBinary(metaBytes); err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "result meta binary")
	}

	entries := make(map[string]string)

	// Extract entries from TransactionMeta
	switch resultMeta.TxApplyProcessing.V {
	case 0:
		if resultMeta.TxApplyProcessing.Operations != nil {
			extractFromLedgerEntryChanges(*resultMeta.TxApplyProcessing.Operations, entries)
		}

	case 1:
		if resultMeta.TxApplyProcessing.V1 != nil {
			extractFromLedgerEntryChanges(resultMeta.TxApplyProcessing.V1.Operations, entries)
		}

	case 2:
		if v2 := resultMeta.TxApplyProcessing.V2; v2 != nil {
			extractFromLedgerEntryChanges(v2.Operations, entries)
			// Also extract from TxChangesBefore and TxChangesAfter
			extractFromChanges(v2.TxChangesBefore, entries)
			extractFromChanges(v2.TxChangesAfter, entries)
		}

	case 3:
		if v3 := resultMeta.TxApplyProcessing.V3; v3 != nil {
			extractFromLedgerEntryChanges(v3.Operations, entries)
			// Also extract from TxChangesBefore and TxChangesAfter
			extractFromChanges(v3.TxChangesBefore, entries)
			extractFromChanges(v3.TxChangesAfter, entries)
		}
	}

	return entries, nil
}

// extractFromLedgerEntryChanges processes operation-level changes
func extractFromLedgerEntryChanges(operations []xdr.OperationMeta, entries map[string]string) {
	for _, op := range operations {
		extractFromChanges(op.Changes, entries)
	}
}

// extractFromChanges processes individual ledger entry changes
func extractFromChanges(changes xdr.LedgerEntryChanges, entries map[string]string) {
	for _, change := range changes {
		switch change.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			if change.Created != nil {
				addEntry(*change.Created, entries)
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			if change.Updated != nil {
				addEntry(*change.Updated, entries)
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryState:
			if change.State != nil {
				addEntry(*change.State, entries)
			}
		}
	}
}

// addEntry adds a ledger entry to the map
func addEntry(entry xdr.LedgerEntry, entries map[string]string) {
	// Generate the key from the entry
	key := ledgerKeyFromEntry(entry)
	if key == nil {
		return
	}

	keyXDR, err := EncodeLedgerKey(*key)
	if err != nil {
		return
	}

	entryXDR, err := EncodeLedgerEntry(entry)
	if err != nil {
		return
	}

	entries[keyXDR] = entryXDR
}

// ledgerKeyFromEntry generates a LedgerKey from a LedgerEntry
func ledgerKeyFromEntry(entry xdr.LedgerEntry) *xdr.LedgerKey {
	switch entry.Data.Type {
	case xdr.LedgerEntryTypeAccount:
		if entry.Data.Account != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.LedgerKeyAccount{
					AccountId: entry.Data.Account.AccountId,
				},
			}
		}

	case xdr.LedgerEntryTypeTrustline:
		if entry.Data.TrustLine != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeTrustline,
				TrustLine: &xdr.LedgerKeyTrustLine{
					AccountId: entry.Data.TrustLine.AccountId,
					Asset:     entry.Data.TrustLine.Asset,
				},
			}
		}

	case xdr.LedgerEntryTypeOffer:
		if entry.Data.Offer != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeOffer,
				Offer: &xdr.LedgerKeyOffer{
					SellerId: entry.Data.Offer.SellerId,
					OfferId:  entry.Data.Offer.OfferId,
				},
			}
		}

	case xdr.LedgerEntryTypeData:
		if entry.Data.Data != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeData,
				Data: &xdr.LedgerKeyData{
					AccountId: entry.Data.Data.AccountId,
					DataName:  entry.Data.Data.DataName,
				},
			}
		}

	case xdr.LedgerEntryTypeClaimableBalance:
		if entry.Data.ClaimableBalance != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeClaimableBalance,
				ClaimableBalance: &xdr.LedgerKeyClaimableBalance{
					BalanceId: entry.Data.ClaimableBalance.BalanceId,
				},
			}
		}

	case xdr.LedgerEntryTypeLiquidityPool:
		if entry.Data.LiquidityPool != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeLiquidityPool,
				LiquidityPool: &xdr.LedgerKeyLiquidityPool{
					LiquidityPoolId: entry.Data.LiquidityPool.LiquidityPoolId,
				},
			}
		}

	case xdr.LedgerEntryTypeContractData:
		if entry.Data.ContractData != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeContractData,
				ContractData: &xdr.LedgerKeyContractData{
					Contract:   entry.Data.ContractData.Contract,
					Key:        entry.Data.ContractData.Key,
					Durability: entry.Data.ContractData.Durability,
				},
			}
		}

	case xdr.LedgerEntryTypeContractCode:
		if entry.Data.ContractCode != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeContractCode,
				ContractCode: &xdr.LedgerKeyContractCode{
					Hash: entry.Data.ContractCode.Hash,
				},
			}
		}

	case xdr.LedgerEntryTypeConfigSetting:
		if entry.Data.ConfigSetting != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeConfigSetting,
				ConfigSetting: &xdr.LedgerKeyConfigSetting{
					ConfigSettingId: entry.Data.ConfigSetting.ConfigSettingId,
				},
			}
		}

	case xdr.LedgerEntryTypeTtl:
		if entry.Data.Ttl != nil {
			return &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeTtl,
				Ttl: &xdr.LedgerKeyTtl{
					KeyHash: entry.Data.Ttl.KeyHash,
				},
			}
		}
	}

	return nil
}
