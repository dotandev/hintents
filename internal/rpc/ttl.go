// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"crypto/sha256"
	"encoding/base64"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// ttlKeyForEntryKey derives the TTL ledger key that accompanies a contract data
// or contract code entry.
//
// The TTL of such an entry is kept in a separate ledger entry keyed by the
// SHA-256 of the entry's own key XDR.
func ttlKeyForEntryKey(entryKey xdr.LedgerKey) (xdr.LedgerKey, bool) {
	switch entryKey.Type {
	case xdr.LedgerEntryTypeContractData, xdr.LedgerEntryTypeContractCode:
	default:
		return xdr.LedgerKey{}, false
	}

	raw, err := entryKey.MarshalBinary()
	if err != nil {
		return xdr.LedgerKey{}, false
	}

	return xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeTtl,
		Ttl:  &xdr.LedgerKeyTtl{KeyHash: xdr.Hash(sha256.Sum256(raw))},
	}, true
}

// ttlEntryFor builds the TTL ledger entry pair implied by a getLedgerEntries
// result, if that result describes an entry that carries one.
//
// Soroban RPC reports an entry's expiry as a `liveUntilLedgerSeq` field beside
// the entry rather than as the TTL ledger entry the host expects to find in
// storage. Without that entry the host aborts any TTL-aware host function —
// `extend_current_contract_instance_and_code_ttl`, for one — with "unexpected
// contract entry without TTL", so the replay state has to carry it explicitly.
func ttlEntryFor(result LedgerEntryResult) (keyXDR string, entryXDR string, ok bool) {
	if result.LiveUntilLedger <= 0 {
		return "", "", false
	}

	rawKey, err := base64.StdEncoding.DecodeString(result.Key)
	if err != nil {
		return "", "", false
	}

	var entryKey xdr.LedgerKey
	if err := entryKey.UnmarshalBinary(rawKey); err != nil {
		return "", "", false
	}

	ttlKey, applicable := ttlKeyForEntryKey(entryKey)
	if !applicable {
		return "", "", false
	}

	ttlEntry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: xdr.Uint32(result.LastModifiedLedger),
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeTtl,
			Ttl: &xdr.TtlEntry{
				KeyHash:            ttlKey.Ttl.KeyHash,
				LiveUntilLedgerSeq: xdr.Uint32(result.LiveUntilLedger),
			},
		},
	}

	encodedKey, err := EncodeLedgerKey(ttlKey)
	if err != nil {
		return "", "", false
	}
	encodedEntry, err := EncodeLedgerEntry(ttlEntry)
	if err != nil {
		return "", "", false
	}

	return encodedKey, encodedEntry, true
}

// ttlKeysFor returns the TTL keys that accompany the given entry keys, so a
// cached lookup can pull them alongside the entries they belong to.
func ttlKeysFor(entryKeys []string) map[string]string {
	byEntryKey := make(map[string]string, len(entryKeys))

	for _, encoded := range entryKeys {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}

		var entryKey xdr.LedgerKey
		if err := entryKey.UnmarshalBinary(raw); err != nil {
			continue
		}

		ttlKey, applicable := ttlKeyForEntryKey(entryKey)
		if !applicable {
			continue
		}

		if encodedTTL, err := EncodeLedgerKey(ttlKey); err == nil {
			byEntryKey[encoded] = encodedTTL
		}
	}

	return byEntryKey
}
