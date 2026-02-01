// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Verify checks the integrity and signature of an AuditLog
func Verify(log *AuditLog) error {
	// 1. Re-calculate Trace Hash
	// We must marshal the payload exactly as it was during generation.
	// Since we use standard json.Marshal in both places on the same struct,
	// it should be deterministic for this tool's usage.
	payloadBytes, err := json.Marshal(log.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	hash := sha256.Sum256(payloadBytes)
	calculatedHashHex := hex.EncodeToString(hash[:])

	if calculatedHashHex != log.TraceHash {
		return fmt.Errorf("trace hash mismatch: expected %s, got %s", log.TraceHash, calculatedHashHex)
	}

	// 2. Verify Signature
	pubKeyBytes, err := hex.DecodeString(log.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key hex: %w", err)
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size")
	}

	sigBytes, err := hex.DecodeString(log.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}

	// Verify the signature against the hash of the payload
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), hash[:], sigBytes) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}
