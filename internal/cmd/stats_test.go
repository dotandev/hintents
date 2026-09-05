// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
)

func makeResponse(events []simulator.CategorizedEvent) *simulator.SimulationResponse {
	return &simulator.SimulationResponse{
		Status:            "success",
		CategorizedEvents: events,
	}
}

func TestBuildContractStats_Empty(t *testing.T) {
	resp := makeResponse(nil)
	stats := buildContractStats(resp)
	if len(stats) != 0 {
		t.Errorf("expected 0 stats, got %d", len(stats))
	}
}

func catEvent(eventType string, contractID *string) simulator.CategorizedEvent {
	return simulator.CategorizedEvent{
		Category: "Diagnostic",
		Event: simulator.DiagnosticEvent{
			EventType:  eventType,
			ContractID: contractID,
		},
	}
}

func TestBuildContractStats_SingleContract(t *testing.T) {
	cid := "CONTRACT_A"
	resp := makeResponse([]simulator.CategorizedEvent{
		catEvent("storage_write", &cid),
		catEvent("require_auth", &cid),
	})

	stats := buildContractStats(resp)

	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}

	s := stats[0]
	wantCost := uint64(costWeightStorageWrite + costWeightAuth)
	if s.estimatedCost != wantCost {
		t.Errorf("estimatedCost = %d, want %d", s.estimatedCost, wantCost)
	}
}

func TestBuildContractStats_Sorted(t *testing.T) {
	cheap := "B"
	expensive := "A"

	resp := makeResponse([]simulator.CategorizedEvent{
		catEvent("contract_call", &cheap),
		catEvent("storage_write", &expensive),
	})

	stats := buildContractStats(resp)

	if stats[0].contractID != expensive {
		t.Errorf("expected %s first, got %s", expensive, stats[0].contractID)
	}
}

func TestEventCost(t *testing.T) {
	cases := []struct {
		eventType string
		want      uint64
	}{
		{"storage_write", uint64(costWeightStorageWrite)},
		{"require_auth", uint64(costWeightAuth)},
		{"other", uint64(costWeightDefault)},
	}

	for _, tc := range cases {
		got := eventCost(tc.eventType)
		if got != tc.want {
			t.Errorf("eventCost(%q) = %d, want %d", tc.eventType, got, tc.want)
		}
	}
}
