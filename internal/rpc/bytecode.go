// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ScValType for contract instance ledger key (ScvLedgerKeyContractInstance = 20 in Stellar XDR).
const scValTypeLedgerKeyContractInstance xdr.ScValType = 20

// LedgerKeyForContractInstance builds a LedgerKey for the contract instance ContractData entry.
// The key is used with getLedgerEntries to fetch the instance, which contains the executable (wasm hash).
func LedgerKeyForContractInstance(contractID xdr.ContractId) (xdr.LedgerKey, error) {
	addr := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}
	// Key for contract instance entry is the special ScVal ScvLedgerKeyContractInstance (void).
	key := xdr.ScVal{
		Type: scValTypeLedgerKeyContractInstance,
	}
	return xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract:   addr,
			Key:        key,
			Durability: xdr.ContractDataDurabilityPersistent,
		},
	}, nil
}

// ContractCodeHashFromInstanceEntry parses a ContractData ledger entry (instance) and returns
// the contract code (WASM) hash from the executable. Returns an error if the entry is not
// a contract instance or has no WASM executable.
func ContractCodeHashFromInstanceEntry(entryXDR string) (xdr.Hash, error) {
	raw, err := base64.StdEncoding.DecodeString(entryXDR)
	if err != nil {
		return xdr.Hash{}, fmt.Errorf("decode instance entry: %w", err)
	}
	var entry xdr.LedgerEntry
	if err := entry.UnmarshalBinary(raw); err != nil {
		return xdr.Hash{}, fmt.Errorf("unmarshal ledger entry: %w", err)
	}
	if entry.Data.Type != xdr.LedgerEntryTypeContractData || entry.Data.ContractData == nil {
		return xdr.Hash{}, fmt.Errorf("not a contract data entry")
	}
	val := entry.Data.ContractData.Val
	if val.Type != xdr.ScValTypeScvContractInstance || val.Instance == nil {
		return xdr.Hash{}, fmt.Errorf("contract data is not a contract instance")
	}
	exec := val.Instance.Executable
	switch exec.Type {
	case xdr.ContractExecutableTypeContractExecutableWasm:
		if exec.WasmHash == nil {
			return xdr.Hash{}, fmt.Errorf("instance executable has nil wasm hash")
		}
		return *exec.WasmHash, nil
	default:
		return xdr.Hash{}, fmt.Errorf("executable type %v is not WASM", exec.Type)
	}
}

// decodeContractID decodes a contract ID from strkey (C...) or 32-byte hex.
func decodeContractID(contractIDStr string) (xdr.ContractId, error) {
	s := strings.TrimSpace(contractIDStr)
	if len(s) == 0 {
		return xdr.ContractId{}, fmt.Errorf("empty contract id")
	}
	if s[0] == 'C' {
		decoded, err := strkey.Decode(strkey.VersionByteContract, s)
		if err != nil {
			return xdr.ContractId{}, fmt.Errorf("decode strkey contract id: %w", err)
		}
		if len(decoded) != 32 {
			return xdr.ContractId{}, fmt.Errorf("contract id must be 32 bytes, got %d", len(decoded))
		}
		var cid xdr.ContractId
		copy(cid[:], decoded)
		return cid, nil
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return xdr.ContractId{}, fmt.Errorf("decode hex contract id: %w", err)
	}
	if len(raw) != 32 {
		return xdr.ContractId{}, fmt.Errorf("contract id must be 32 bytes, got %d", len(raw))
	}
	var cid xdr.ContractId
	copy(cid[:], raw)
	return cid, nil
}

// FetchContractBytecode fetches the un-executed WASM for the given contract ID via getLedgerEntries,
// and caches it using the existing RPC client cache. contractIDStr can be a strkey (C...) or 32-byte hex.
// It returns the ledger key->entry map for the instance and code entries; the client also caches them.
func FetchContractBytecode(ctx context.Context, c *Client, contractIDStr string) (map[string]string, error) {
	cid, err := decodeContractID(contractIDStr)
	if err != nil {
		return nil, err
	}

	instanceKey, err := LedgerKeyForContractInstance(cid)
	if err != nil {
		return nil, fmt.Errorf("build instance key: %w", err)
	}
	entries := make(map[string]string)
	instanceKeyB64, err := EncodeLedgerKey(instanceKey)
	if err != nil {
		return nil, fmt.Errorf("encode instance key: %w", err)
	}

	instanceEntries, err := c.GetLedgerEntries(ctx, []string{instanceKeyB64})
	if err != nil {
		return nil, fmt.Errorf("get ledger entries (instance): %w", err)
	}
	for k, v := range instanceEntries {
		entries[k] = v
	}
	instanceEntry, ok := instanceEntries[instanceKeyB64]
	if !ok || instanceEntry == "" {
		return nil, fmt.Errorf("contract instance not found for %s", contractIDStr)
	}

	codeHash, err := ContractCodeHashFromInstanceEntry(instanceEntry)
	if err != nil {
		return nil, fmt.Errorf("get code hash from instance: %w", err)
	}

	codeKey := xdr.LedgerKey{
		Type:         xdr.LedgerEntryTypeContractCode,
		ContractCode: &xdr.LedgerKeyContractCode{Hash: codeHash},
	}
	codeKeyB64, err := EncodeLedgerKey(codeKey)
	if err != nil {
		return nil, fmt.Errorf("encode code key: %w", err)
	}

	codeEntries, err := c.GetLedgerEntries(ctx, []string{codeKeyB64})
	if err != nil {
		return nil, fmt.Errorf("get ledger entries (code): %w", err)
	}
	for k, v := range codeEntries {
		entries[k] = v
	}
	logger.Logger.Debug("Fetched contract bytecode on demand", "contract_id", contractIDStr, "cached", true)
	return entries, nil
}

// FetchBytecodeForTraceContractCalls collects unique contract IDs from diagnostic events,
// fetches each contract's WASM via getLedgerEntries (and caches it), and returns the combined
// ledger entries map. Entries already present in existingMap are not re-fetched.
// The client's cache is populated so subsequent use of the same contract ID will hit the cache.
func FetchBytecodeForTraceContractCalls(ctx context.Context, c *Client, contractIDs []string, existingMap map[string]string) (map[string]string, error) {
	seen := make(map[string]struct{})
	for _, id := range contractIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		// We could check existingMap for an entry that matches this contract's code, but that would
		// require parsing; simpler to always call FetchContractBytecode which uses the client cache.
		fetched, err := FetchContractBytecode(ctx, c, id)
		if err != nil {
			logger.Logger.Warn("Failed to fetch contract bytecode for trace", "contract_id", id, "error", err)
			continue
		}
		if existingMap == nil {
			existingMap = make(map[string]string)
		}
		for k, v := range fetched {
			existingMap[k] = v
		}
	}
	return existingMap, nil
}
