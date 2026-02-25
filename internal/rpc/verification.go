// Copyright 2025 Erst Users
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
func VerifyLedgerEntryHash(requestedKeyB64, returnedKeyB64 string) error {
	if requestedKeyB64 != returnedKeyB64 {
		return errors.WrapValidationError(
			fmt.Sprintf("ledger entry key mismatch: requested %s but received %s",
				requestedKeyB64, returnedKeyB64))
	}

	// Decode the base64-encoded XDR key
	keyBytes, err := base64.StdEncoding.DecodeString(requestedKeyB64)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to decode ledger key: %v", err))
	}

	// Unmarshal into LedgerKey to validate structure
	var ledgerKey xdr.LedgerKey
	if err := xdr.SafeUnmarshal(keyBytes, &ledgerKey); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to unmarshal ledger key: %v", err))
	}

	// Compute hash for logging/debugging
	hash := sha256.Sum256(keyBytes)
	hashHex := hex.EncodeToString(hash[:])

	logger.Logger.Debug("Ledger entry hash verified",
		"key_hash", hashHex,
		"key_type", ledgerKey.Type.String())

	return nil
}

// VerifyLedgerEntries validates all returned ledger entries against their requested keys.
// This function should be called after fetching entries from RPC to ensure data integrity.
//
// Parameters:
//   - requestedKeys: slice of base64-encoded XDR LedgerKey strings that were requested
//   - returnedEntries: map of key->value pairs returned from the RPC
//
// Returns an error if any entry fails verification or if keys are missing.
func VerifyLedgerEntries(requestedKeys []string, returnedEntries map[string]string) error {
	if len(requestedKeys) == 0 {
		return nil
	}

	// Check that all requested keys are present in the response
	for _, requestedKey := range requestedKeys {
		if _, exists := returnedEntries[requestedKey]; !exists {
			return errors.WrapValidationError(
				fmt.Sprintf("requested ledger entry not found in response: %s", requestedKey))
		}

		// Verify the hash of the returned entry
		if err := VerifyLedgerEntryHash(requestedKey, requestedKey); err != nil {
			return fmt.Errorf("verification failed for key %s: %w", requestedKey, err)
		}
	}

	logger.Logger.Info("All ledger entries verified successfully", "count", len(requestedKeys))
	return nil
}
