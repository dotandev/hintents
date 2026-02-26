// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import "testing"

func TestMockTimeInjection_OverridesRequestTimestamp(t *testing.T) {
	const fixedTime int64 = 1700000000

	runner := &Runner{
		BinaryPath: "unused-in-this-test",
		Debug:      false,
		MockTime:   fixedTime,
	}

	req := &SimulationRequest{
		EnvelopeXdr:   "test-envelope",
		ResultMetaXdr: "test-meta",
		Timestamp:     9999999,
	}

	if runner.MockTime != 0 {
		req.Timestamp = runner.MockTime
	}

	if req.Timestamp != fixedTime {
		t.Errorf("expected Timestamp %d after mock-time injection, got %d", fixedTime, req.Timestamp)
	}
}

func TestMockTimeInjection_ZeroDoesNotOverride(t *testing.T) {
	const originalTime int64 = 1234567890

	runner := &Runner{
		BinaryPath: "unused-in-this-test",
		Debug:      false,
		MockTime:   0,
	}

	req := &SimulationRequest{
		EnvelopeXdr:   "test-envelope",
		ResultMetaXdr: "test-meta",
		Timestamp:     originalTime,
	}

	if runner.MockTime != 0 {
		req.Timestamp = runner.MockTime
	}

	if req.Timestamp != originalTime {
		t.Errorf("expected Timestamp %d unchanged when MockTime is 0, got %d", originalTime, req.Timestamp)
	}
}

func TestNewRunnerWithMockTime_SetsField(t *testing.T) {
	const wantMockTime int64 = 1700000000

	r := &Runner{
		BinaryPath: "/fake/path",
		Debug:      false,
		MockTime:   wantMockTime,
	}

	if r.MockTime != wantMockTime {
		t.Errorf("expected MockTime %d, got %d", wantMockTime, r.MockTime)
	}
}
