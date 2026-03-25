// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
)

// Verify checks the integrity and signature of an AuditLog
func Verify(log *AuditLog) error {
	// 1. Re-calculate Trace Hash using canonical JSON
	// We must marshal the payload exactly as it was during generation.
	// Using canonical JSON ensures deterministic serialization across platforms.
	payloadBytes, err := marshalCanonical(log.Payload)
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}

	hash := sha256.Sum256(payloadBytes)
	calculatedHashHex := hex.EncodeToString(hash[:])

	if calculatedHashHex != log.TraceHash {
		return errors.WrapAuditLogInvalid(fmt.Sprintf("trace hash mismatch: expected %s, got %s", log.TraceHash, calculatedHashHex))
	}

	// 2. Verify Signature
	pubKeyBytes, err := hex.DecodeString(log.PublicKey)
	if err != nil {
		return errors.WrapAuditLogInvalid(fmt.Sprintf("invalid public key hex: %v", err))
	}
	if len(pubKeyBytes) != ed25519.PublicKeySize {
		return errors.WrapAuditLogInvalid("invalid public key size")
	}

	sigBytes, err := hex.DecodeString(log.Signature)
	if err != nil {
		return errors.WrapAuditLogInvalid(fmt.Sprintf("invalid signature hex: %v", err))
	}

	// Verify the signature against the hash of the payload
	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), hash[:], sigBytes) {
		return errors.WrapAuditLogInvalid("signature verification failed")
	}

	return nil
}
