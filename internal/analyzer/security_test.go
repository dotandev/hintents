// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/assert"
)

func catEvent(category, eventType string, contractID *string, topics []string, data string) simulator.CategorizedEvent {
	return simulator.CategorizedEvent{
		Category: category,
		Event: simulator.DiagnosticEvent{
			EventType:  eventType,
			ContractID: contractID,
			Topics:     topics,
			Data:       data,
		},
	}
}

func TestSecurityAnalyzer_NoViolations(t *testing.T) {
	analyzer := NewSecurityAnalyzer()

	contractID := "contract123"
	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			catEvent("Diagnostic", "require_auth", &contractID, []string{"require_auth"}, "auth_data"),
			catEvent("Diagnostic", "storage_write", &contractID, []string{"write"}, "write_data"),
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Empty(t, violations)
}

func TestSecurityAnalyzer_UnauthorizedStateModification(t *testing.T) {
	analyzer := NewSecurityAnalyzer()

	contractID := "contract123"
	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			catEvent("Diagnostic", "storage_write", &contractID, []string{"write"}, "unauthorized_write"),
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Len(t, violations, 1)
	assert.Equal(t, "UnauthorizedStateModification", violations[0].Type)
	assert.Equal(t, "high", violations[0].Severity)
}

func TestSecurityAnalyzer_SACPattern_NoFalsePositive(t *testing.T) {
	tests := []struct {
		name   string
		events []simulator.CategorizedEvent
	}{
		{
			name: "SAC balance update",
			events: []simulator.CategorizedEvent{
				catEvent("Diagnostic", "storage_write", strPtr("sac_contract"), []string{"Balance"}, "balance_data"),
			},
		},
		{
			name: "SAC allowance update",
			events: []simulator.CategorizedEvent{
				catEvent("Diagnostic", "storage_write", strPtr("sac_contract"), []string{"Allowance"}, "allowance_data"),
			},
		},
		{
			name: "SAC admin operation",
			events: []simulator.CategorizedEvent{
				catEvent("Diagnostic", "storage_write", strPtr("sac_contract"), []string{"Admin"}, "admin_data"),
			},
		},
		{
			name: "Stellar asset contract",
			events: []simulator.CategorizedEvent{
				catEvent("Diagnostic", "storage_write", strPtr("sac_contract"), []string{"write"}, "stellar_asset_data"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewSecurityAnalyzer()
			resp := &simulator.SimulationResponse{
				Status:            "success",
				CategorizedEvents: tt.events,
			}

			violations := analyzer.Analyze(resp)
			assert.Empty(t, violations, "SAC pattern should not trigger false positives")
		})
	}
}

func TestSecurityAnalyzer_MultipleContracts(t *testing.T) {
	analyzer := NewSecurityAnalyzer()

	contract1 := "contract1"
	contract2 := "contract2"

	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			catEvent("Diagnostic", "require_auth", &contract1, []string{"require_auth"}, "auth1"),
			catEvent("Diagnostic", "storage_write", &contract1, []string{"write"}, "write1"),
			catEvent("Diagnostic", "storage_write", &contract2, []string{"write"}, "write2"),
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Len(t, violations, 1)
	assert.Contains(t, violations[0].Description, contract2)
}

func TestSecurityAnalyzer_AuthAfterWrite_StillViolation(t *testing.T) {
	analyzer := NewSecurityAnalyzer()

	contractID := "contract123"
	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			catEvent("Diagnostic", "storage_write", &contractID, []string{"write"}, "write_data"),
			catEvent("Diagnostic", "require_auth", &contractID, []string{"require_auth"}, "auth_data"),
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Len(t, violations, 1)
}

func strPtr(s string) *string {
	return &s
}
