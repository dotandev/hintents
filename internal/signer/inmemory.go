// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package signer

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

// InMemorySigner holds an Ed25519 private key in process memory and
// implements the Signer interface. This is the default signer for
// backward compatibility with existing callers that pass hex-encoded
// private keys directly.
type InMemorySigner struct {
	privateKey ed25519.PrivateKey
}

// NewInMemorySigner creates an InMemorySigner from a hex-encoded Ed25519
// private key. The key may be either a 32-byte seed or a full 64-byte
// private key.
func NewInMemorySigner(privateKeyHex string) (*InMemorySigner, error) {
	raw, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, &SignerError{Op: "inmemory", Msg: "invalid private key hex", Err: err}
	}

	if len(raw) != ed25519.PrivateKeySize && len(raw) != ed25519.SeedSize {
		return nil, &SignerError{
			Op:  "inmemory",
			Msg: fmt.Sprintf("invalid private key length: %d", len(raw)),
		}
	}

	var priv ed25519.PrivateKey
	if len(raw) == ed25519.SeedSize {
		priv = ed25519.NewKeyFromSeed(raw)
	} else {
		priv = ed25519.PrivateKey(raw)
	}

	return &InMemorySigner{privateKey: priv}, nil
}

// NewInMemorySignerFromKey creates an InMemorySigner from an existing
// ed25519.PrivateKey value.
func NewInMemorySignerFromKey(key ed25519.PrivateKey) *InMemorySigner {
	return &InMemorySigner{privateKey: key}
}

// Sign produces an Ed25519 signature over the provided data.
func (s *InMemorySigner) Sign(data []byte) ([]byte, error) {
	return ed25519.Sign(s.privateKey, data), nil
}

// PublicKey returns the raw Ed25519 public key bytes.
func (s *InMemorySigner) PublicKey() ([]byte, error) {
	pub, ok := s.privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, &SignerError{Op: "inmemory", Msg: "failed to derive public key"}
	}
	return []byte(pub), nil
}

// Algorithm returns "ed25519".
func (s *InMemorySigner) Algorithm() string {
	return "ed25519"
}
