// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
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

func fetchContractCodeEntry(ctx context.Context, c *Client, codeHash xdr.Hash) (string, string, error) {
	codeKey := xdr.LedgerKey{
		Type:         xdr.LedgerEntryTypeContractCode,
		ContractCode: &xdr.LedgerKeyContractCode{Hash: codeHash},
	}
	codeKeyB64, err := EncodeLedgerKey(codeKey)
	if err != nil {
		return "", "", fmt.Errorf("encode code key: %w", err)
	}

	codeEntries, err := c.GetLedgerEntries(ctx, []string{codeKeyB64})
	if err != nil {
		return "", "", fmt.Errorf("get ledger entries (code): %w", err)
	}
	codeEntry, ok := codeEntries[codeKeyB64]
	if !ok || codeEntry == "" {
		return "", "", fmt.Errorf("contract code not found for hash %x", codeHash)
	}

	return codeKeyB64, codeEntry, nil
}

// WasmBytesFromContractCodeEntry parses a base64-encoded ContractCode ledger entry
// and returns the raw WASM bytecode.
func WasmBytesFromContractCodeEntry(entryXDR string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(entryXDR)
	if err != nil {
		return nil, fmt.Errorf("decode contract code entry: %w", err)
	}
	var entry xdr.LedgerEntry
	if err := entry.UnmarshalBinary(raw); err != nil {
		return nil, fmt.Errorf("unmarshal ledger entry: %w", err)
	}
	if entry.Data.Type != xdr.LedgerEntryTypeContractCode || entry.Data.ContractCode == nil {
		return nil, fmt.Errorf("not a contract code entry")
	}
	return entry.Data.ContractCode.Code, nil
}

// ApplyWasmOverrideToLedgerEntries rewrites the contract instance for contractIDStr
// to point at the supplied local WASM and injects a matching ContractCode entry.
// The returned string is the hex-encoded hash of the injected WASM.
func ApplyWasmOverrideToLedgerEntries(entries map[string]string, contractIDStr string, wasm []byte) (string, error) {
	if len(wasm) == 0 {
		return "", fmt.Errorf("local wasm is empty")
	}
	if entries == nil {
		return "", fmt.Errorf("ledger entries map is nil")
	}

	cid, err := decodeContractID(contractIDStr)
	if err != nil {
		return "", err
	}

	instanceKey, err := LedgerKeyForContractInstance(cid)
	if err != nil {
		return "", fmt.Errorf("build instance key: %w", err)
	}
	instanceKeyB64, err := EncodeLedgerKey(instanceKey)
	if err != nil {
		return "", fmt.Errorf("encode instance key: %w", err)
	}

	instanceEntryXDR, ok := entries[instanceKeyB64]
	if !ok || instanceEntryXDR == "" {
		return "", fmt.Errorf("contract instance not found for %s", contractIDStr)
	}

	instanceEntry, err := decodeLedgerEntry(instanceEntryXDR)
	if err != nil {
		return "", fmt.Errorf("decode instance entry: %w", err)
	}
	if instanceEntry.Data.Type != xdr.LedgerEntryTypeContractData || instanceEntry.Data.ContractData == nil {
		return "", fmt.Errorf("contract instance entry for %s is not contract data", contractIDStr)
	}
	if instanceEntry.Data.ContractData.Val.Type != xdr.ScValTypeScvContractInstance || instanceEntry.Data.ContractData.Val.Instance == nil {
		return "", fmt.Errorf("contract instance entry for %s does not contain a contract instance", contractIDStr)
	}

	exec := instanceEntry.Data.ContractData.Val.Instance.Executable
	if exec.Type != xdr.ContractExecutableTypeContractExecutableWasm {
		return "", fmt.Errorf("contract %s executable type %v is not WASM", contractIDStr, exec.Type)
	}

	newHash := xdr.Hash(sha256.Sum256(wasm))
	exec.WasmHash = &newHash
	instanceEntry.Data.ContractData.Val.Instance.Executable = exec

	updatedInstanceXDR, err := EncodeLedgerEntry(instanceEntry)
	if err != nil {
		return "", fmt.Errorf("encode updated instance entry: %w", err)
	}
	entries[instanceKeyB64] = updatedInstanceXDR

	codeKey := xdr.LedgerKey{
		Type:         xdr.LedgerEntryTypeContractCode,
		ContractCode: &xdr.LedgerKeyContractCode{Hash: newHash},
	}
	codeKeyB64, err := EncodeLedgerKey(codeKey)
	if err != nil {
		return "", fmt.Errorf("encode code key: %w", err)
	}

	codeEntry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: instanceEntry.LastModifiedLedgerSeq,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeContractCode,
			ContractCode: &xdr.ContractCodeEntry{
				Hash: newHash,
				Code: wasm,
				Ext:  xdr.ContractCodeEntryExt{V: 0},
			},
		},
		Ext: xdr.LedgerEntryExt{V: 0},
	}
	codeEntryXDR, err := EncodeLedgerEntry(codeEntry)
	if err != nil {
		return "", fmt.Errorf("encode updated contract code entry: %w", err)
	}
	entries[codeKeyB64] = codeEntryXDR

	return hex.EncodeToString(newHash[:]), nil
}

func decodeLedgerEntry(entryXDR string) (xdr.LedgerEntry, error) {
	raw, err := base64.StdEncoding.DecodeString(entryXDR)
	if err != nil {
		return xdr.LedgerEntry{}, fmt.Errorf("decode ledger entry: %w", err)
	}
	var entry xdr.LedgerEntry
	if err := entry.UnmarshalBinary(raw); err != nil {
		return xdr.LedgerEntry{}, fmt.Errorf("unmarshal ledger entry: %w", err)
	}
	return entry, nil
}

// FetchHistoricalContractBytecode retrieves the WASM bytecode for a contract as it
// appeared in a specific transaction's result metadata. This enables auditing a
// contract's code at a particular version by referencing the deploy or upgrade
// transaction hash. It returns the raw WASM bytes and the hex-encoded code hash.
func FetchHistoricalContractBytecode(ctx context.Context, c *Client, contractIDStr string, txHash string) ([]byte, string, error) {
	txResp, err := c.GetTransaction(ctx, txHash)
	if err != nil {
		return nil, "", fmt.Errorf("fetch transaction %s: %w", txHash, err)
	}
	if txResp.ResultMetaXdr == "" {
		return nil, "", errors.WrapValidationError(fmt.Sprintf("transaction %s has no result metadata", txHash))
	}

	entries, err := ExtractLedgerEntriesFromMeta(txResp.ResultMetaXdr)
	if err != nil {
		return nil, "", fmt.Errorf("extract ledger entries from meta: %w", err)
	}

	cid, err := decodeContractID(contractIDStr)
	if err != nil {
		return nil, "", err
	}

	instanceKey, err := LedgerKeyForContractInstance(cid)
	if err != nil {
		return nil, "", fmt.Errorf("build instance key: %w", err)
	}
	instanceKeyB64, err := EncodeLedgerKey(instanceKey)
	if err != nil {
		return nil, "", fmt.Errorf("encode instance key: %w", err)
	}

	instanceEntry, ok := entries[instanceKeyB64]
	if !ok {
		return nil, "", errors.WrapValidationError(
			fmt.Sprintf("contract instance not found in transaction metadata for %s", contractIDStr),
		)
	}

	codeHash, err := ContractCodeHashFromInstanceEntry(instanceEntry)
	if err != nil {
		return nil, "", fmt.Errorf("get code hash from instance: %w", err)
	}

	codeKey := xdr.LedgerKey{
		Type:         xdr.LedgerEntryTypeContractCode,
		ContractCode: &xdr.LedgerKeyContractCode{Hash: codeHash},
	}
	codeKeyB64, err := EncodeLedgerKey(codeKey)
	if err != nil {
		return nil, "", fmt.Errorf("encode code key: %w", err)
	}

	codeEntry, ok := entries[codeKeyB64]
	if !ok {
		return nil, "", errors.WrapValidationError(
			fmt.Sprintf("contract code not found in transaction metadata for hash %x", codeHash),
		)
	}

	wasmBytes, err := WasmBytesFromContractCodeEntry(codeEntry)
	if err != nil {
		return nil, "", fmt.Errorf("extract wasm bytes: %w", err)
	}

	hashHex := hex.EncodeToString(codeHash[:])
	logger.Logger.Debug("Fetched historical contract bytecode",
		"contract_id", contractIDStr,
		"tx_hash", txHash,
		"code_hash", hashHex,
	)
	return wasmBytes, hashHex, nil
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
		if existingMap == nil {
			existingMap = make(map[string]string)
		}

		cid, err := decodeContractID(id)
		if err != nil {
			logger.Logger.Warn("Failed to decode contract id for trace", "contract_id", id, "error", err)
			continue
		}
		instanceKey, err := LedgerKeyForContractInstance(cid)
		if err != nil {
			logger.Logger.Warn("Failed to build contract instance key for trace", "contract_id", id, "error", err)
			continue
		}
		instanceKeyB64, err := EncodeLedgerKey(instanceKey)
		if err != nil {
			logger.Logger.Warn("Failed to encode contract instance key for trace", "contract_id", id, "error", err)
			continue
		}

		if instanceEntry, ok := existingMap[instanceKeyB64]; ok && instanceEntry != "" {
			codeHash, err := ContractCodeHashFromInstanceEntry(instanceEntry)
			if err != nil {
				logger.Logger.Warn("Failed to derive contract code hash from existing replay state", "contract_id", id, "error", err)
				continue
			}

			codeKey := xdr.LedgerKey{
				Type:         xdr.LedgerEntryTypeContractCode,
				ContractCode: &xdr.LedgerKeyContractCode{Hash: codeHash},
			}
			codeKeyB64, err := EncodeLedgerKey(codeKey)
			if err != nil {
				logger.Logger.Warn("Failed to encode contract code key for trace", "contract_id", id, "error", err)
				continue
			}
			if codeEntry, ok := existingMap[codeKeyB64]; ok && codeEntry != "" {
				continue
			}

			codeKeyB64, codeEntry, err := fetchContractCodeEntry(ctx, c, codeHash)
			if err != nil {
				logger.Logger.Warn("Failed to fetch missing contract code for trace", "contract_id", id, "error", err)
				continue
			}
			if _, ok := existingMap[codeKeyB64]; !ok || existingMap[codeKeyB64] == "" {
				existingMap[codeKeyB64] = codeEntry
			}
			continue
		}

		fetched, err := FetchContractBytecode(ctx, c, id)
		if err != nil {
			logger.Logger.Warn("Failed to fetch contract bytecode for trace", "contract_id", id, "error", err)
			continue
		}
		for k, v := range fetched {
			if _, ok := existingMap[k]; ok && existingMap[k] != "" {
				continue
			}
			existingMap[k] = v
		}
	}
	return existingMap, nil
}
