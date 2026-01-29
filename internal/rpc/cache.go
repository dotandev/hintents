// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/stellar/go/xdr"
)

// HashLedgerKey generates a deterministic SHA-256 hash of a Stellar LedgerKey.
// This hash is used as a cache file name to prevent redundant RPC calls.
//
// The function serializes the LedgerKey to its canonical XDR binary format
// and computes a SHA-256 hash of the result. This ensures:
// - Deterministic output: same key always produces the same hash
// - Collision resistance: different keys produce different hashes
// - Cross-platform consistency: XDR binary format is platform-independent
//
// Returns a 64-character hexadecimal string representing the SHA-256 hash.
func HashLedgerKey(key xdr.LedgerKey) (string, error) {
	// Serialize the LedgerKey to XDR binary format
	xdrBytes, err := key.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal LedgerKey to XDR: %w", err)
	}

	// Compute SHA-256 hash of the XDR bytes
	hash := sha256.Sum256(xdrBytes)

	// Convert to hexadecimal string
	return hex.EncodeToString(hash[:]), nil
}
