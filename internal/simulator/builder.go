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

package simulator

import (
	"fmt"
)

// SimulationRequestBuilder provides a fluent interface for building SimulationRequest objects.
// It uses the builder pattern to make request construction more readable and less error-prone.
//
// Example usage:
//
//	req, err := NewSimulationRequestBuilder().
//		WithEnvelopeXDR("AAAAAgAAAA...").
//		WithResultMetaXDR("AAAAAQAAA...").
//		WithLedgerEntry("key1", "value1").
//		Build()
type SimulationRequestBuilder struct {
	envelopeXdr   string
	resultMetaXdr string
	ledgerEntries map[string]string
	errors        []string
}

// NewSimulationRequestBuilder creates a new builder instance.
func NewSimulationRequestBuilder() *SimulationRequestBuilder {
	return &SimulationRequestBuilder{
		ledgerEntries: make(map[string]string),
		errors:        make([]string, 0),
	}
}

// WithEnvelopeXDR sets the XDR encoded TransactionEnvelope.
// This is a required field for simulation.
func (b *SimulationRequestBuilder) WithEnvelopeXDR(xdr string) *SimulationRequestBuilder {
	b.envelopeXdr = xdr
	return b
}

// WithResultMetaXDR sets the XDR encoded TransactionResultMeta.
// This contains historical data needed for replay.
func (b *SimulationRequestBuilder) WithResultMetaXDR(xdr string) *SimulationRequestBuilder {
	b.resultMetaXdr = xdr
	return b
}

// WithLedgerEntry adds a single ledger entry to the snapshot.
// The key and value should both be XDR encoded.
func (b *SimulationRequestBuilder) WithLedgerEntry(key, value string) *SimulationRequestBuilder {
	if key == "" {
		b.errors = append(b.errors, "ledger entry key cannot be empty")
		return b
	}
	if value == "" {
		b.errors = append(b.errors, fmt.Sprintf("ledger entry value for key '%s' cannot be empty", key))
		return b
	}
	b.ledgerEntries[key] = value
	return b
}

// WithLedgerEntries sets multiple ledger entries at once.
// This replaces any previously set ledger entries.
func (b *SimulationRequestBuilder) WithLedgerEntries(entries map[string]string) *SimulationRequestBuilder {
	if entries == nil {
		b.ledgerEntries = make(map[string]string)
		return b
	}

	// Validate entries
	for key, value := range entries {
		if key == "" {
			b.errors = append(b.errors, "ledger entry key cannot be empty")
			continue
		}
		if value == "" {
			b.errors = append(b.errors, fmt.Sprintf("ledger entry value for key '%s' cannot be empty", key))
			continue
		}
	}

	b.ledgerEntries = entries
	return b
}

// Build constructs and validates the final SimulationRequest.
// Returns an error if required fields are missing or validation fails.
func (b *SimulationRequestBuilder) Build() (*SimulationRequest, error) {
	// Check for any errors collected during building
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("validation errors: %v", b.errors)
	}

	// Validate required fields
	if b.envelopeXdr == "" {
		return nil, fmt.Errorf("envelope XDR is required")
	}

	if b.resultMetaXdr == "" {
		return nil, fmt.Errorf("result meta XDR is required")
	}

	// Build the request
	req := &SimulationRequest{
		EnvelopeXdr:   b.envelopeXdr,
		ResultMetaXdr: b.resultMetaXdr,
	}

	// Only set ledger entries if there are any
	if len(b.ledgerEntries) > 0 {
		req.LedgerEntries = b.ledgerEntries
	}

	return req, nil
}

// MustBuild is like Build but panics if there's an error.
// Use this only when you're certain the request is valid (e.g., in tests with known good data).
func (b *SimulationRequestBuilder) MustBuild() *SimulationRequest {
	req, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build simulation request: %v", err))
	}
	return req
}

// Reset clears all fields and errors, allowing the builder to be reused.
func (b *SimulationRequestBuilder) Reset() *SimulationRequestBuilder {
	b.envelopeXdr = ""
	b.resultMetaXdr = ""
	b.ledgerEntries = make(map[string]string)
	b.errors = make([]string, 0)
	return b
}
