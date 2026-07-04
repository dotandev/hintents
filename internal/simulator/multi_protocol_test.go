// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"testing"
)

func TestRunMultiProtocol(t *testing.T) {
	// This is a basic unit test for the multi-protocol functionality
	// Integration tests would require a real simulator binary

	t.Run("validate protocol versions", func(t *testing.T) {
		versions := Supported()
		if len(versions) == 0 {
			t.Error("Expected at least one supported protocol version")
		}

		for _, v := range versions {
			if err := Validate(v); err != nil {
				t.Errorf("Protocol version %d should be valid: %v", v, err)
			}
		}
	})

	t.Run("detect feature changes", func(t *testing.T) {
		versions := []uint32{20, 21, 22}
		changes := detectFeatureChanges(versions)

		// We expect some feature changes between protocol versions
		if len(changes) == 0 {
			t.Log("No feature changes detected between protocols 20, 21, 22")
		} else {
			t.Logf("Detected %d feature changes", len(changes))
			for _, change := range changes {
				t.Logf("  - %s: %v -> %v", change.FeatureName, change.OldValue, change.NewValue)
			}
		}
	})

	t.Run("analyze gas impact", func(t *testing.T) {
		gasCosts := map[uint32]uint64{
			20: 100000,
			21: 95000,
			22: 90000,
		}

		impact := analyzeGasImpact(gasCosts)

		if impact.MinCost != 90000 {
			t.Errorf("Expected min cost 90000, got %d", impact.MinCost)
		}
		if impact.MaxCost != 100000 {
			t.Errorf("Expected max cost 100000, got %d", impact.MaxCost)
		}
		if impact.MinProtocol != 22 {
			t.Errorf("Expected min protocol 22, got %d", impact.MinProtocol)
		}
		if impact.MaxProtocol != 20 {
			t.Errorf("Expected max protocol 20, got %d", impact.MaxProtocol)
		}
		expectedVariance := float64(100000-90000) / float64(90000) * 100
		if impact.Variance != expectedVariance {
			t.Errorf("Expected variance %.2f, got %.2f", expectedVariance, impact.Variance)
		}
	})

	t.Run("clone request", func(t *testing.T) {
		original := &SimulationRequest{
			EnvelopeXdr:     "test_xdr",
			LedgerEntries:   map[string]string{"key1": "value1"},
			ProtocolVersion: func() *uint32 { v := uint32(21); return &v }(),
		}

		cloned := cloneRequest(original)

		if cloned.EnvelopeXdr != original.EnvelopeXdr {
			t.Error("Cloned request should have same envelope XDR")
		}

		// Modify original to ensure deep copy
		original.LedgerEntries["key2"] = "value2"
		if _, exists := cloned.LedgerEntries["key2"]; exists {
			t.Error("Cloned request should be a deep copy")
		}

		// Modify protocol version in clone
		newProto := uint32(22)
		cloned.ProtocolVersion = &newProto
		if *original.ProtocolVersion != 21 {
			t.Error("Modifying clone should not affect original")
		}
	})
}

func TestMultiProtocolComparison(t *testing.T) {
	t.Run("empty comparison", func(t *testing.T) {
		comparison := &MultiProtocolComparison{
			Results:        []*MultiProtocolResult{},
			GasCosts:       make(map[uint32]uint64),
			SuccessByProto: make(map[uint32]bool),
			ErrorByProto:   make(map[uint32]string),
		}

		impact := analyzeGasImpact(comparison.GasCosts)
		if impact.MinCost != 0 || impact.MaxCost != 0 {
			t.Error("Empty gas costs should result in zero impact")
		}
	})
}
