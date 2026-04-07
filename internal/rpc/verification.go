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
func validateLedgerKeyXDR(keyB64 string) error {
	keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
// VerifyLedgerEntryHash cryptographically verifies that a returned ledger entry
// matches the expected hash derived from its key. This ensures data integrity
// before feeding entries to the simulator.
//
// The verification process:
// 1. Decode the base64-encoded XDR key
// 2. Unmarshal it into a LedgerKey structure
// 3. Compute SHA-256 hash of the key's binary representation
// 4. Compare with the hash of the returned entry's key
//
// Returns an error if verification fails or if XDR decoding fails.
// VerifyLedgerEntryHash cryptographically verifies that a returned ledger entry
// matches the expected hash derived from its key. This ensures data integrity
// before feeding entries to the simulator.
func VerifyLedgerEntryHash(requestedKeyB64 string, result LedgerEntryResult) error {
	if requestedKeyB64 != result.Key {
		return errors.WrapValidationError(
			fmt.Sprintf("ledger entry key mismatch: requested %s but received %s",
				requestedKeyB64, result.Key))
	}

	// Decode the base64-encoded XDR entry
	entryBytes, err := base64.StdEncoding.DecodeString(result.Xdr)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to decode ledger entry: %v", err))
	}

	// Unmarshal into LedgerEntry to validate structure
	var ledgerEntry xdr.LedgerEntry
	if err := xdr.SafeUnmarshal(entryBytes, &ledgerEntry); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to unmarshal ledger entry: %v", err))
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

	// Decode the base64-encoded XDR key for logging
	keyBytes, err := base64.StdEncoding.DecodeString(requestedKeyB64)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to decode ledger key: %v", err))
	}

	var ledgerKey xdr.LedgerKey
	if err := xdr.SafeUnmarshal(keyBytes, &ledgerKey); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to unmarshal ledger key: %v", err))
	}

	hash := sha256.Sum256(keyBytes)
	hashHex := hex.EncodeToString(hash[:])

	logger.Logger.Debug("Ledger entry hash verified",
		"key_hash", hashHex,
		"key_type", ledgerKey.Type.String())

	return nil
}

// VerifyLedgerEntryHash cryptographically verifies that a returned ledger entry key
// matches the requested key and that the key is structurally valid XDR. This ensures
// data integrity before feeding entries to the simulator.
//
// The verification process:
//  1. Compare requestedKeyB64 and returnedKeyB64 — reject mismatches immediately
//  2. Decode the base64-encoded XDR key
//  3. Unmarshal it into a LedgerKey structure
//  4. Emit a SHA-256 hash of the key for debug traceability
//
// Returns an error if the keys differ, if base64 decoding fails, or if XDR is malformed.
func VerifyLedgerEntryHash(requestedKeyB64, returnedKeyB64 string) error {
	if requestedKeyB64 != returnedKeyB64 {
		return errors.WrapValidationError(
			fmt.Sprintf("ledger entry key mismatch: requested %s but received %s",
				requestedKeyB64, returnedKeyB64))
	}
	return validateLedgerKeyXDR(requestedKeyB64)
}

// VerifyLedgerEntries validates all returned ledger entries against their requested keys.
// Call this after fetching entries from the RPC layer to ensure data integrity before
// passing the state to the simulator.
//
// Parameters:
//   - requestedKeys: slice of base64-encoded XDR LedgerKey strings that were requested
//   - returnedEntries: map of key→XDR-entry pairs returned from the RPC
//
// Returns an error if any key is absent from the response or fails XDR structural validation.
// func VerifyLedgerEntries(requestedKeys []string, returnedEntries map[string]string) error {
//   - returnedEntries: slice of LedgerEntryResult returned from the RPC
//
// Returns an error if any entry fails verification or if keys are missing.
func VerifyLedgerEntries(requestedKeys []string, returnedEntries []LedgerEntryResult) error {
	if len(requestedKeys) == 0 {
		return nil
	}

	// Build a fast lookup map
	returnedMap := make(map[string]LedgerEntryResult, len(returnedEntries))
	for _, entry := range returnedEntries {
		returnedMap[entry.Key] = entry
	}

	// Check that all requested keys are present in the response
// 	for _, requestedKey := range requestedKeys {
	for _, requestedKey := range requestedKeys {
		// Verify the server returned an entry for this key.
		// Because getLedgerEntriesAttempt indexes entries by the server-returned key
		// (entry.Key), presence here already confirms requestedKey == entry.Key.
		if _, exists := returnedEntries[requestedKey]; !exists {
	// Build a fast lookup map
	returnedMap := make(map[string]LedgerEntryResult, len(returnedEntries))
	for _, entry := range returnedEntries {
		returnedMap[entry.Key] = entry
	}

	// Check that all requested keys are present in the response
	for _, requestedKey := range requestedKeys {
		entry, exists := returnedMap[requestedKey]
		if !exists {
			return errors.WrapValidationError(
				fmt.Sprintf("requested ledger entry not found in response: %s", requestedKey))
		}

		// Validate the structural integrity of the key XDR.
		// We call validateLedgerKeyXDR directly rather than VerifyLedgerEntryHash to
		// avoid a self-comparison (requestedKey, requestedKey) that makes the key-
		// equality branch unreachable — the presence check above already guarantees
		// the returned key equals the requested key.
		if err := validateLedgerKeyXDR(requestedKey); err != nil {
		// Verify the hash of the returned entry
		if err := VerifyLedgerEntryHash(requestedKey, entry); err != nil {
			return fmt.Errorf("verification failed for key %s: %w", requestedKey, err)
		}
	}

	logger.Logger.Info("All ledger entries verified successfully", "count", len(requestedKeys))
	return nil
}
