// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package signer

import "fmt"

// Signer is the generic interface for cryptographic signing operations.
// Implementations may hold keys in memory (InMemorySigner) or delegate
// to an external PKCS#11 hardware security module (Pkcs11Signer).
type Signer interface {
	// Sign produces a digital signature over the provided data.
	Sign(data []byte) ([]byte, error)

	// PublicKey returns the raw public key bytes associated with the
	// signing key.
	PublicKey() ([]byte, error)

	// Algorithm returns the signing algorithm name (e.g. "ed25519").
	Algorithm() string
}

// SignerError represents an error originating from a signing operation.
type SignerError struct {
	Op  string
	Msg string
	Err error
}

func (e *SignerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Msg)
}

func (e *SignerError) Unwrap() error {
	return e.Err
}
