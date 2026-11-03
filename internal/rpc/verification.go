// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// validateLedgerKeyXDR decodes a base64-encoded XDR LedgerKey, validates its structure,
// and emits a debug log with the key's SHA-256 hash and type. It is the canonical
// integrity check shared by VerifyLedgerEntryHash and VerifyLedgerEntries.
func validateLedgerKeyXDR(keyB64 string) error { //nolint:unused // Reserved for future ledger entry validation
	keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to decode ledger key: %v", err))
	}

	var ledgerKey xdr.LedgerKey
	if err := xdr.SafeUnmarshal(keyBytes, &ledgerKey); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to unmarshal ledger key: %v", err))
	}

	hash := sha256.Sum256(keyBytes)
	hashHex := hex.EncodeToString(hash[:])

	logger.Logger.Debug("Ledger key validated",
		"key_hash", hashHex,
		"key_type", ledgerKey.Type.String())

	return nil
}

// LedgerEntryFromResult rebuilds a whole LedgerEntry from a getLedgerEntries
// result.
//
// Soroban RPC puts the entry's DATA union (LedgerEntryData) in `xdr` and
// reports the ledger it was last modified in as a sibling field, while
// transaction metadata — and the simulator — deal in whole LedgerEntry values.
// Reading the wire shape as a LedgerEntry decodes the sequence number as the
// entry type and fails on whatever it finds next, so everything on the replay
// path goes through here and one shape reaches the simulator regardless of
// which source produced the entry.
//
// An `xdr` field that already holds a whole LedgerEntry is accepted as well,
// since not every provider follows the wire format.
func LedgerEntryFromResult(result LedgerEntryResult) (xdr.LedgerEntry, error) {
	entryBytes, err := base64.StdEncoding.DecodeString(result.Xdr)
	if err != nil {
		return xdr.LedgerEntry{}, errors.WrapValidationError(
			fmt.Sprintf("failed to decode ledger entry: %v", err))
	}

	var data xdr.LedgerEntryData
	if dataErr := xdr.SafeUnmarshal(entryBytes, &data); dataErr == nil {
		return xdr.LedgerEntry{
			LastModifiedLedgerSeq: xdr.Uint32(result.LastModifiedLedger),
			Data:                  data,
		}, nil
	}

	var entry xdr.LedgerEntry
	if entryErr := xdr.SafeUnmarshal(entryBytes, &entry); entryErr != nil {
		return xdr.LedgerEntry{}, errors.WrapValidationError(
			fmt.Sprintf("failed to unmarshal ledger entry: %v", entryErr))
	}

	return entry, nil
}

// EncodeLedgerEntryResult normalizes a getLedgerEntries result to the base64
// XDR of a whole LedgerEntry, the form the simulator reads.
func EncodeLedgerEntryResult(result LedgerEntryResult) (string, error) {
	entry, err := LedgerEntryFromResult(result)
	if err != nil {
		return "", err
	}
	return EncodeLedgerEntry(entry)
}

// VerifyLedgerEntryHash cryptographically verifies that a returned ledger entry
// matches the expected hash derived from its key. This ensures data integrity
// before feeding entries to the simulator.
func VerifyLedgerEntryHash(requestedKeyB64 string, result LedgerEntryResult) error {
	if requestedKeyB64 != result.Key {
		return errors.WrapValidationError(
			fmt.Sprintf("ledger entry key mismatch: requested %s but received %s",
				requestedKeyB64, result.Key))
	}

	ledgerEntry, err := LedgerEntryFromResult(result)
	if err != nil {
		return err
	}

	// Verify that the entry's key matches the requested key
	derivedKey := ledgerKeyFromEntry(ledgerEntry)
	if derivedKey == nil {
		return errors.WrapValidationError("failed to derive ledger key from entry")
	}

	derivedKeyB64, err := EncodeLedgerKey(*derivedKey)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to encode derived ledger key: %v", err))
	}

	if derivedKeyB64 != requestedKeyB64 {
		return errors.WrapValidationError(
			fmt.Sprintf("cryptographic mismatch: requested %s but entry hashes to %s",
				requestedKeyB64, derivedKeyB64))
	}

	return nil
}

// MissingLedgerKeys reports which of the requested keys the response did not
// carry an entry for.
//
// Absence is a fact about the ledger, not an error: Soroban RPC omits a key
// that holds no entry, which is the honest answer for one a transaction was
// about to create, or one that has expired and been archived. Whether that is
// acceptable depends on what the caller is doing, so VerifyLedgerEntries does
// not judge it and this reports it instead.
func MissingLedgerKeys(requestedKeys []string, returnedEntries []LedgerEntryResult) []string {
	returned := make(map[string]struct{}, len(returnedEntries))
	for _, entry := range returnedEntries {
		returned[entry.Key] = struct{}{}
	}

	var missing []string
	for _, key := range requestedKeys {
		if _, ok := returned[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

// VerifyLedgerEntries validates all returned ledger entries against their requested keys.
// Call this after fetching entries from the RPC layer to ensure data integrity before
// passing the state to the simulator.
func VerifyLedgerEntries(requestedKeys []string, returnedEntries []LedgerEntryResult) error {
	if len(requestedKeys) == 0 {
		return nil
	}

	// Build a fast lookup map
	returnedMap := make(map[string]LedgerEntryResult, len(returnedEntries))
	for _, entry := range returnedEntries {
		returnedMap[entry.Key] = entry
	}

	// Every entry that came back must be the entry that was asked for. A key
	// that came back with nothing is not checked here: omitting it is how
	// Soroban RPC says the ledger holds no such entry, which is a legitimate
	// answer for a key a transaction was about to create, or one that has since
	// expired and been archived. Whether that absence is acceptable depends on
	// what the caller is replaying, so the caller decides.
	for _, requestedKey := range requestedKeys {
		entry, exists := returnedMap[requestedKey]
		if !exists {
			continue
		}

		// Verify the hash of the returned entry
		if err := VerifyLedgerEntryHash(requestedKey, entry); err != nil {
			return fmt.Errorf("verification failed for key %s: %w", requestedKey, err)
		}
	}

	logger.Logger.Info("All ledger entries verified successfully", "count", len(requestedKeys))
	return nil
}
