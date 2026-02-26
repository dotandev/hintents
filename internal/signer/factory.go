// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package signer

import "os"

// NewFromEnv creates a Signer based on the ERST_SIGNER_TYPE environment
// variable. When the variable is absent or set to "software", an
// InMemorySigner is returned using the hex key from
// ERST_SOFTWARE_PRIVATE_KEY_HEX. When set to "pkcs11", a Pkcs11Signer
// is created from ERST_PKCS11_* environment variables.
func NewFromEnv() (Signer, error) {
	signerType := os.Getenv("ERST_SIGNER_TYPE")
	if signerType == "" {
		signerType = "software"
	}

	switch signerType {
	case "software":
		keyHex := os.Getenv("ERST_SOFTWARE_PRIVATE_KEY_HEX")
		if keyHex == "" {
			return nil, &SignerError{Op: "factory", Msg: "ERST_SOFTWARE_PRIVATE_KEY_HEX is required for software signer"}
		}
		return NewInMemorySigner(keyHex)

	case "pkcs11":
		cfg, err := Pkcs11ConfigFromEnv()
		if err != nil {
			return nil, err
		}
		return NewPkcs11Signer(*cfg)

	default:
		return nil, &SignerError{Op: "factory", Msg: "unsupported ERST_SIGNER_TYPE: " + signerType}
	}
}
