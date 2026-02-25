// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"encoding/hex"
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
)

// ValidateTransactionHash checks if the provided string is a valid Stellar transaction hash.
// A valid hash must be exactly 64 characters long and contain only hexadecimal characters.
// The check is case-insensitive.
func ValidateTransactionHash(hash string) error {
	if len(hash) != 64 {
		return errors.WrapValidationError(fmt.Sprintf("transaction hash must be exactly 64 characters long, got %d", len(hash)))
	}
	_, err := hex.DecodeString(hash)
	if err != nil {
		return errors.WrapValidationError("transaction hash must contain only valid hexadecimal characters")
	}
	return nil
}
