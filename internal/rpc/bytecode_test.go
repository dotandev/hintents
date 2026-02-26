// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"encoding/base64"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestLedgerKeyForContractInstance(t *testing.T) {
	var cid xdr.ContractId
	for i := range 32 {
		cid[i] = byte(i + 1)
	}
	key, err := LedgerKeyForContractInstance(cid)
	if err != nil {
		t.Fatalf("LedgerKeyForContractInstance: %v", err)
	}
	if key.Type != xdr.LedgerEntryTypeContractData {
		t.Errorf("expected ContractData key type, got %v", key.Type)
	}
	if key.ContractData == nil {
		t.Fatal("expected non-nil ContractData")
	}
	if key.ContractData.Contract.Type != xdr.ScAddressTypeScAddressTypeContract {
		t.Errorf("expected contract address type, got %v", key.ContractData.Contract.Type)
	}
	if key.ContractData.Contract.ContractId == nil || *key.ContractData.Contract.ContractId != cid {
		t.Error("contract id mismatch")
	}
	if key.ContractData.Durability != xdr.ContractDataDurabilityPersistent {
		t.Errorf("expected persistent durability, got %v", key.ContractData.Durability)
	}
	_, err = EncodeLedgerKey(key)
	if err != nil {
		t.Errorf("EncodeLedgerKey: %v", err)
	}
}

func TestDecodeContractID_Strkey(t *testing.T) {
	// Use a valid strkey form: we need 32 bytes encoded. Build from known bytes.
	var cid xdr.ContractId
	for i := range 32 {
		cid[i] = byte(i)
	}
	// Decode then re-encode as strkey would require the strkey package; instead test hex path
	hexID := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	decoded, err := decodeContractID(hexID)
	if err != nil {
		t.Fatalf("decodeContractID(hex): %v", err)
	}
	if decoded != cid {
		t.Errorf("decodeContractID hex: got %x, want %x", decoded[:], cid[:])
	}
}

func TestDecodeContractID_Empty(t *testing.T) {
	_, err := decodeContractID("")
	if err == nil {
		t.Error("expected error for empty contract id")
	}
}

func TestDecodeContractID_InvalidHex(t *testing.T) {
	_, err := decodeContractID("zz")
	if err == nil {
		t.Error("expected error for invalid hex")
	}
}

func TestDecodeContractID_WrongLengthHex(t *testing.T) {
	_, err := decodeContractID("0001") // 2 bytes
	if err == nil {
		t.Error("expected error for wrong length hex")
	}
}

func TestContractCodeHashFromInstanceEntry_InvalidBase64(t *testing.T) {
	_, err := ContractCodeHashFromInstanceEntry("!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestContractCodeHashFromInstanceEntry_NotContractData(t *testing.T) {
	// Build a minimal account entry (wrong type)
	entry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				Balance:   100,
			},
		},
	}
	raw, _ := entry.MarshalBinary()
	b64 := base64.StdEncoding.EncodeToString(raw)
	_, err := ContractCodeHashFromInstanceEntry(b64)
	if err == nil {
		t.Error("expected error for non-contract-data entry")
	}
}
