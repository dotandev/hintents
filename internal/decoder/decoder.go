// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"encoding/base64"
	"fmt"

	"github.com/stellar/go/xdr"
)

// DecodeEnvelope decodes a base64-encoded XDR transaction envelope
func DecodeEnvelope(envelopeXdr string) (*xdr.TransactionEnvelope, error) {
	if envelopeXdr == "" {
		return nil, fmt.Errorf("envelope XDR is empty")
	}

	// Decode base64
	xdrBytes, err := base64.StdEncoding.DecodeString(envelopeXdr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Decode XDR
	var envelope xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshal(xdrBytes, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XDR: %w", err)
	}

	return &envelope, nil
}
