// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
