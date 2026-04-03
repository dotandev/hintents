// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"encoding/base64"
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// buildValidEntryB64 decodes a base64-encoded XDR LedgerKey, constructs a
// valid LedgerEntry whose key fields match, and returns it as base64 XDR.
// This is used by mock servers so that VerifyLedgerEntryHash passes.
func buildValidEntryB64(keyB64 string) string {
	keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		panic(fmt.Sprintf("buildValidEntryB64: bad base64: %v", err))
	}
	var lk xdr.LedgerKey
	if err := xdr.SafeUnmarshal(keyBytes, &lk); err != nil {
		panic(fmt.Sprintf("buildValidEntryB64: bad XDR key: %v", err))
	}

	var entry xdr.LedgerEntry
	entry.LastModifiedLedgerSeq = 100

	switch lk.Type {
	case xdr.LedgerEntryTypeAccount:
		entry.Data = xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: lk.Account.AccountId,
				Balance:   1000,
			},
		}
	case xdr.LedgerEntryTypeContractCode:
		entry.Data = xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.ContractCodeEntry{
				Hash: lk.ContractCode.Hash,
				Code: []byte{0xCA, 0xFE},
			},
		}
	case xdr.LedgerEntryTypeContractData:
		entry.Data = xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractData,
			ContractData: &xdr.ContractDataEntry{
				Contract:   lk.ContractData.Contract,
				Key:        lk.ContractData.Key,
				Durability: lk.ContractData.Durability,
			},
		}
	case xdr.LedgerEntryTypeTrustline:
		entry.Data = xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.TrustLineEntry{
				AccountId: lk.TrustLine.AccountId,
				Asset:     lk.TrustLine.Asset,
				Balance:   500,
				Limit:     1000,
			},
		}
	default:
		panic(fmt.Sprintf("buildValidEntryB64: unsupported key type %v", lk.Type))
	}

	eb, err := entry.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("buildValidEntryB64: marshal failed: %v", err))
	}
	return base64.StdEncoding.EncodeToString(eb)
}
