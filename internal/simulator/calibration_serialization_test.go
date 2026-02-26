// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/json"
	"testing"
)

func TestCalibrationSerialization(t *testing.T) {
	req := &SimulationRequest{
		EnvelopeXdr: "AAAA...",
		ResourceCalibration: &ResourceCalibration{
			SHA256Fixed:      3738,
			SHA256PerByte:    37,
			Keccak256Fixed:   keccak256FixedCalibration,
			Keccak256PerByte: keccak256PerByteCalibration,
			Ed25519Fixed:     377524,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var req2 SimulationRequest
	if err := json.Unmarshal(data, &req2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req2.ResourceCalibration == nil {
		t.Fatal("ResourceCalibration is nil after unmarshal")
	}

	if req2.ResourceCalibration.SHA256Fixed != 3738 {
		t.Errorf("expected 3738, got %d", req2.ResourceCalibration.SHA256Fixed)
	}
	if req2.ResourceCalibration.Keccak256Fixed != keccak256FixedCalibration {
		t.Errorf("expected %d, got %d", keccak256FixedCalibration, req2.ResourceCalibration.Keccak256Fixed)
	}
	if req2.ResourceCalibration.Keccak256PerByte != keccak256PerByteCalibration {
		t.Errorf("expected %d, got %d", keccak256PerByteCalibration, req2.ResourceCalibration.Keccak256PerByte)
	}
}
