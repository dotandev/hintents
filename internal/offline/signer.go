// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package offline

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dotandev/hintents/internal/errors"
)

// SignEnvelope signs the envelope XDR with the given ed25519 private key and
// appends the signature to the file. The key must be a 64-byte hex-encoded
// ed25519 private key (or 32-byte seed – both forms are accepted).
func SignEnvelope(ef *EnvelopeFile, privateKeyHex string) error {
	privKey, pubKey, err := parsePrivateKey(privateKeyHex)
	if err != nil {
		return err
	}

	// Sign the raw envelope XDR bytes (same content covered by checksum).
	sig := ed25519.Sign(privKey, []byte(ef.EnvelopeXDR))

	pubHex := hex.EncodeToString(pubKey)
	sigHex := hex.EncodeToString(sig)

	// Check for duplicate signatures from the same key.
	for _, existing := range ef.Signatures {
		if existing.PublicKey == pubHex {
			return errors.WrapValidationError(
				fmt.Sprintf("envelope already signed by key %s", pubHex),
			)
		}
	}

	ef.Signatures = append(ef.Signatures, SignatureEntry{
		PublicKey: pubHex,
		Signature: sigHex,
		SignedAt:  time.Now().UTC().Format(time.RFC3339),
	})

	return nil
}

// VerifySignatures checks every signature attached to the envelope.
// Returns nil when all signatures are valid, or an error describing the first
// invalid one.
func VerifySignatures(ef *EnvelopeFile) error {
	if len(ef.Signatures) == 0 {
		return errors.WrapValidationError("no signatures to verify")
	}

	msg := []byte(ef.EnvelopeXDR)

	for i, entry := range ef.Signatures {
		pubBytes, err := hex.DecodeString(entry.PublicKey)
		if err != nil {
			return errors.WrapValidationError(
				fmt.Sprintf("signature %d: invalid public key hex: %v", i, err),
			)
		}

		if len(pubBytes) != ed25519.PublicKeySize {
			return errors.WrapValidationError(
				fmt.Sprintf("signature %d: invalid public key size %d", i, len(pubBytes)),
			)
		}

		sigBytes, err := hex.DecodeString(entry.Signature)
		if err != nil {
			return errors.WrapValidationError(
				fmt.Sprintf("signature %d: invalid signature hex: %v", i, err),
			)
		}

		if !ed25519.Verify(ed25519.PublicKey(pubBytes), msg, sigBytes) {
			return errors.WrapValidationError(
				fmt.Sprintf("signature %d (key %s): verification failed", i, entry.PublicKey),
			)
		}
	}

	return nil
}

// parsePrivateKey accepts a hex-encoded ed25519 private key.
// It handles both 32-byte seeds and 64-byte full keys.
func parsePrivateKey(hexKey string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, nil, errors.WrapValidationError(fmt.Sprintf("invalid private key hex: %v", err))
	}

	switch len(raw) {
	case ed25519.SeedSize: // 32 bytes – derive full key
		priv := ed25519.NewKeyFromSeed(raw)
		pub := priv.Public().(ed25519.PublicKey)
		return priv, pub, nil
	case ed25519.PrivateKeySize: // 64 bytes – full key
		priv := ed25519.PrivateKey(raw)
		pub := priv.Public().(ed25519.PublicKey)
		return priv, pub, nil
	default:
		return nil, nil, errors.WrapValidationError(
			fmt.Sprintf("private key must be 32 or 64 bytes, got %d", len(raw)),
		)
	}
}
