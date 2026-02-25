// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator_test

import (
	"fmt"

	"github.com/dotandev/hintents/internal/simulator"
)

// ExampleSimulationRequestBuilder demonstrates basic usage of the builder pattern.
func ExampleSimulationRequestBuilder() {
	req, err := simulator.NewSimulationRequestBuilder().
		WithEnvelopeXDR("AAAAAgAAAACE...").
		WithResultMetaXDR("AAAAAQAAAAA...").
		Build()

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request created with envelope XDR length: %d\n", len(req.EnvelopeXdr))
	// Output: Request created with envelope XDR length: 15
}

// ExampleSimulationRequestBuilder_withLedgerEntries demonstrates adding ledger entries.
func ExampleSimulationRequestBuilder_withLedgerEntries() {
	req, err := simulator.NewSimulationRequestBuilder().
		WithEnvelopeXDR("AAAAAgAAAACE...").
		WithResultMetaXDR("AAAAAQAAAAA...").
		WithLedgerEntry("key1", "value1").
		WithLedgerEntry("key2", "value2").
		Build()

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request created with %d ledger entries\n", len(req.LedgerEntries))
	// Output: Request created with 2 ledger entries
}

// ExampleSimulationRequestBuilder_bulkLedgerEntries demonstrates setting multiple entries at once.
func ExampleSimulationRequestBuilder_bulkLedgerEntries() {
	entries := map[string]string{
		"contract_key_1": "contract_value_1",
		"contract_key_2": "contract_value_2",
		"contract_key_3": "contract_value_3",
	}

	req, err := simulator.NewSimulationRequestBuilder().
		WithEnvelopeXDR("AAAAAgAAAACE...").
		WithResultMetaXDR("AAAAAQAAAAA...").
		WithLedgerEntries(entries).
		Build()

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Request created with %d ledger entries\n", len(req.LedgerEntries))
	// Output: Request created with 3 ledger entries
}

// ExampleSimulationRequestBuilder_validation demonstrates validation errors.
func ExampleSimulationRequestBuilder_validation() {
	_, err := simulator.NewSimulationRequestBuilder().
		WithEnvelopeXDR("AAAAAgAAAACE...").
		Build()

	if err != nil {
		fmt.Println("Validation error:", err)
	}
	// Output: Validation error: validation failed: result meta XDR is required
}

// ExampleSimulationRequestBuilder_reuse demonstrates builder reuse with Reset().
func ExampleSimulationRequestBuilder_reuse() {
	builder := simulator.NewSimulationRequestBuilder()

	req1, _ := builder.
		WithEnvelopeXDR("envelope1").
		WithResultMetaXDR("result1").
		Build()

	fmt.Printf("First request envelope: %s\n", req1.EnvelopeXdr)

	req2, _ := builder.
		Reset().
		WithEnvelopeXDR("envelope2").
		WithResultMetaXDR("result2").
		Build()

	fmt.Printf("Second request envelope: %s\n", req2.EnvelopeXdr)
	// Output:
	// First request envelope: envelope1
	// Second request envelope: envelope2
}
