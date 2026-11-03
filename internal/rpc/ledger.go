// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"encoding/base64"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/state"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerEntryPair represents a ledger key-entry pair for simulation
type LedgerEntryPair struct {
	KeyXDR   string
	EntryXDR string
}

// LedgerHeaderResponse contains essential ledger header information needed
// for transaction replay simulation. The type lives in internal/types so that
// packages such as internal/state can use it without importing this package
// (which would create an import cycle through the cache layer).

// LedgerHeaderResponse contains essential ledger header information
// needed for transaction replay simulation. This structure provides
// all the metadata required to recreate the blockchain state at the
// time a transaction was executed.
type LedgerHeaderResponse = state.LedgerHeaderResponse

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

// decodeTransactionMeta decodes a base64 result_meta_xdr field. Soroban RPC and
// Horizon both return a TransactionMeta directly; the TransactionResultMeta
// wrapper belongs to stellar-core's ledger-close-meta stream and is accepted
// here only so callers holding that shape still work.
func decodeTransactionMeta(resultMetaXDR string) (*xdr.TransactionMeta, error) {
	metaBytes, err := base64.StdEncoding.DecodeString(resultMetaXDR)
	if err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "result meta")
	}

	var meta xdr.TransactionMeta
	if err := meta.UnmarshalBinary(metaBytes); err != nil {
		var resultMeta xdr.TransactionResultMeta
		if errRM := resultMeta.UnmarshalBinary(metaBytes); errRM == nil {
			return &resultMeta.TxApplyProcessing, nil
		}
		return nil, errors.WrapUnmarshalFailed(err, "result meta binary")
	}

	return &meta, nil
}

// PreTxLedgerState is the ledger state as it stood immediately BEFORE a
// transaction was applied — the state a faithful replay has to start from.
//
// KnownAbsent holds keys the metadata proves did not exist yet, because they
// appear only as Created changes. Those are not gaps to go and fill: a replay
// must leave them unset, or a contract branching on absence takes the opposite
// branch to the one it originally took.
type PreTxLedgerState struct {
	Entries     map[string]string
	KnownAbsent map[string]struct{}
}

// preTxAccumulator collects pre-transaction state while walking changes.
//
// Ledger entry changes are emitted in chronological order, so the FIRST thing
// said about a key is the truth about it before the transaction ran. Every
// branch below therefore refuses to overwrite a key already resolved.
type preTxAccumulator struct {
	entries map[string]string
	created map[string]struct{}
}

func newPreTxAccumulator() *preTxAccumulator {
	return &preTxAccumulator{
		entries: make(map[string]string),
		created: make(map[string]struct{}),
	}
}

// resolved reports whether the pre-transaction truth for key is already known,
// either as a value or as proven absence.
func (a *preTxAccumulator) resolved(key string) bool {
	if _, ok := a.entries[key]; ok {
		return true
	}
	_, ok := a.created[key]
	return ok
}

func (a *preTxAccumulator) collectOperations(operations []xdr.OperationMeta) {
	for _, op := range operations {
		a.collectChanges(op.Changes)
	}
}

// collectChanges records what each key held before this transaction ran.
//
// stellar-core emits the pre-image (State) immediately followed by the
// post-image (Updated) for the same key, so a plain last-write-wins map ends up
// holding post-state — the exact opposite of what a replay needs. Taking the
// first observation instead makes State win naturally, while still falling back
// to Updated for metadata that omits the pre-image.
func (a *preTxAccumulator) collectChanges(changes xdr.LedgerEntryChanges) {
	for _, change := range changes {
		switch change.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryState:
			if change.State == nil {
				continue
			}
			if key, entry, ok := encodeKeyedEntry(*change.State); ok && !a.resolved(key) {
				a.entries[key] = entry
			}

		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			if change.Updated == nil {
				continue
			}
			if key, entry, ok := encodeKeyedEntry(*change.Updated); ok && !a.resolved(key) {
				a.entries[key] = entry
			}

		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			if change.Created == nil {
				continue
			}
			// A creation says the entry was absent beforehand. Record that as a
			// fact about the pre-state rather than seeding the replay with a
			// value that did not exist yet.
			if key, _, ok := encodeKeyedEntry(*change.Created); ok && !a.resolved(key) {
				a.created[key] = struct{}{}
			}

		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			// A removal is preceded by its State pre-image, which is what the
			// replay needs; the key alone carries no value to record.
		}
	}
}

// ExtractPreTxLedgerState reconstructs the pre-transaction ledger state from a
// base64 result_meta_xdr field.
//
// The result is only ever as complete as the metadata. Soroban RPC reports
// contract state under SorobanMeta rather than as operation-level ledger entry
// changes, so for a Soroban transaction this routinely comes back EMPTY — a
// legitimate outcome, not an error. Callers replaying a transaction must not
// treat it as the whole read set; resolve the envelope's declared footprint
// with Client.ResolveReplayState instead.
//
// Changes recorded after the operations ran (TxChangesAfter, e.g. fee refunds)
// describe state the transaction had already modified, so they are not a source
// of pre-transaction values and are skipped.
func ExtractPreTxLedgerState(resultMetaXDR string) (*PreTxLedgerState, error) {
	meta, err := decodeTransactionMeta(resultMetaXDR)
	if err != nil {
		return nil, err
	}

	acc := newPreTxAccumulator()

	switch meta.V {
	case 0:
		if meta.Operations != nil {
			acc.collectOperations(*meta.Operations)
		}
	case 1:
		if v1 := meta.V1; v1 != nil {
			acc.collectChanges(v1.TxChanges)
			acc.collectOperations(v1.Operations)
		}
	case 2:
		if v2 := meta.V2; v2 != nil {
			acc.collectChanges(v2.TxChangesBefore)
			acc.collectOperations(v2.Operations)
		}
	case 3:
		if v3 := meta.V3; v3 != nil {
			acc.collectChanges(v3.TxChangesBefore)
			acc.collectOperations(v3.Operations)
		}
	}

	return &PreTxLedgerState{Entries: acc.entries, KnownAbsent: acc.created}, nil
}

// ExtractLedgerEntriesFromMeta returns the pre-transaction value of every ledger
// entry the metadata accounts for.
//
// Note for replay callers: the metadata is not the transaction's read set, and
// for Soroban transactions it is frequently empty. Prefer
// Client.ResolveReplayState, which starts from the declared footprint.
func ExtractLedgerEntriesFromMeta(resultMetaXDR string) (map[string]string, error) {
	state, err := ExtractPreTxLedgerState(resultMetaXDR)
	if err != nil {
		return nil, err
	}
	return state.Entries, nil
}

// encodeKeyedEntry encodes a ledger entry and its derived key as base64 XDR.
func encodeKeyedEntry(entry xdr.LedgerEntry) (key string, value string, ok bool) {
	derived := ledgerKeyFromEntry(entry)
	if derived == nil {
		return "", "", false
	}

	keyXDR, err := EncodeLedgerKey(*derived)
	if err != nil {
		return "", "", false
	}

	entryXDR, err := EncodeLedgerEntry(entry)
	if err != nil {
		return "", "", false
	}

	return keyXDR, entryXDR, true
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

// ExtractPostStateLedgerEntries extracts the ledger entries representing the
// state *after* a transaction was applied. It reads the "Updated" and "Created"
// change records from TxChangesAfter and operation-level changes, giving the
// caller a map of XDR key → XDR entry that reflects post-transaction state.
//
// Removed entries (LedgerEntryChangeTypeLedgerEntryRemoved) are represented as
// an empty string value so callers can distinguish "entry deleted" from "entry
// not present in the diff".
func ExtractPostStateLedgerEntries(resultMetaXDR string) (map[string]string, error) {
	metaBytes, err := base64.StdEncoding.DecodeString(resultMetaXDR)
	if err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "result meta")
	}

	var meta xdr.TransactionMeta
	if err := meta.UnmarshalBinary(metaBytes); err != nil {
		var resultMeta xdr.TransactionResultMeta
		if errRM := resultMeta.UnmarshalBinary(metaBytes); errRM == nil {
			meta = resultMeta.TxApplyProcessing
		} else {
			return nil, errors.WrapUnmarshalFailed(err, "result meta binary")
		}
	}

	after := make(map[string]string)

	switch meta.V {
	case 0:
		if meta.Operations != nil {
			extractPostStateFromOperations(*meta.Operations, after)
		}
	case 1:
		if v1 := meta.V1; v1 != nil {
			extractPostStateFromOperations(v1.Operations, after)
		}
	case 2:
		if v2 := meta.V2; v2 != nil {
			extractPostStateFromOperations(v2.Operations, after)
			extractPostStateFromChanges(v2.TxChangesAfter, after)
		}
	case 3:
		if v3 := meta.V3; v3 != nil {
			extractPostStateFromOperations(v3.Operations, after)
			extractPostStateFromChanges(v3.TxChangesAfter, after)
		}
	}

	return after, nil
}

// extractPostStateFromOperations collects post-state entries from operation-level changes.
func extractPostStateFromOperations(operations []xdr.OperationMeta, after map[string]string) {
	for _, op := range operations {
		extractPostStateFromChanges(op.Changes, after)
	}
}

// extractPostStateFromChanges processes a LedgerEntryChanges slice and records
// the final state of each entry. Created and Updated entries are stored as their
// XDR value; Removed entries are stored as an empty string.
func extractPostStateFromChanges(changes xdr.LedgerEntryChanges, after map[string]string) {
	for _, change := range changes {
		switch change.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			if change.Created != nil {
				addEntry(*change.Created, after)
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			if change.Updated != nil {
				addEntry(*change.Updated, after)
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			if change.Removed != nil {
				keyXDR, err := EncodeLedgerKey(*change.Removed)
				if err == nil {
					after[keyXDR] = "" // empty value signals deletion
				}
			}
		}
	}
}
