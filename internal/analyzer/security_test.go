// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/assert"
)

func TestSecurityAnalyzer_NoViolations(t *testing.T) {
	analyzer := NewSecurityAnalyzer()

	contractID := "contract123"
	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			{
				EventType:  "require_auth",
				ContractID: &contractID,
				Topics:     []string{"require_auth"},
				Data:       "auth_data",
			},
			{
				EventType:  "storage_write",
				ContractID: &contractID,
				Topics:     []string{"write"},
				Data:       "write_data",
			},
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
			{
				EventType:  "storage_write",
				ContractID: &contractID,
				Topics:     []string{"write"},
				Data:       "unauthorized_write",
			},
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
				{
					EventType:  "storage_write",
					ContractID: strPtr("sac_contract"),
					Topics:     []string{"Balance"},
					Data:       "balance_data",
				},
			},
		},
		{
			name: "SAC allowance update",
			events: []simulator.CategorizedEvent{
				{
					EventType:  "storage_write",
					ContractID: strPtr("sac_contract"),
					Topics:     []string{"Allowance"},
					Data:       "allowance_data",
				},
			},
		},
		{
			name: "SAC admin operation",
			events: []simulator.CategorizedEvent{
				{
					EventType:  "storage_write",
					ContractID: strPtr("sac_contract"),
					Topics:     []string{"Admin"},
					Data:       "admin_data",
				},
			},
		},
		{
			name: "Stellar asset contract",
			events: []simulator.CategorizedEvent{
				{
					EventType:  "storage_write",
					ContractID: strPtr("sac_contract"),
					Topics:     []string{"write"},
					Data:       "stellar_asset_data",
				},
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
			{
				EventType:  "require_auth",
				ContractID: &contract1,
				Topics:     []string{"require_auth"},
				Data:       "auth1",
			},
			{
				EventType:  "storage_write",
				ContractID: &contract1,
				Topics:     []string{"write"},
				Data:       "write1",
			},
			{
				EventType:  "storage_write",
				ContractID: &contract2,
				Topics:     []string{"write"},
				Data:       "write2",
			},
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
			{
				EventType:  "storage_write",
				ContractID: &contractID,
				Topics:     []string{"write"},
				Data:       "write_data",
			},
			{
				EventType:  "require_auth",
				ContractID: &contractID,
				Topics:     []string{"require_auth"},
				Data:       "auth_data",
			},
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Len(t, violations, 1)
}

func strPtr(s string) *string {
	return &s
}

func TestSecurityAnalyzer_UnboundedStorageGrowth_Triggered(t *testing.T) {
	analyzer := NewSecurityAnalyzer()

	contractID := "contract123"
	largePayload := strings.Repeat("A", 2048) // Exceeds default 1024 bytes threshold

	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			{
				EventType:  "require_auth",
				ContractID: &contractID,
				Topics:     []string{"require_auth"},
				Data:       "auth_data",
			},
			{
				EventType:  "storage_write",
				ContractID: &contractID,
				Topics:     []string{"write"},
				Data:       largePayload,
			},
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Len(t, violations, 1)
	assert.Equal(t, "UnboundedStorageGrowth", violations[0].Type)
	assert.Equal(t, "medium", violations[0].Severity)
	assert.Contains(t, violations[0].Description, "exceeds growth threshold")
}

func TestSecurityAnalyzer_UnboundedStorageGrowth_BelowThreshold_NoViolation(t *testing.T) {
	analyzer := NewSecurityAnalyzer()

	contractID := "contract123"
	smallPayload := strings.Repeat("A", 128) // Below threshold

	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			{
				EventType:  "require_auth",
				ContractID: &contractID,
				Topics:     []string{"require_auth"},
				Data:       "auth_data",
			},
			{
				EventType:  "storage_write",
				ContractID: &contractID,
				Topics:     []string{"write"},
				Data:       smallPayload,
			},
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Empty(t, violations)
}

func TestSecurityAnalyzer_UserInputKeyedStorage_Triggered(t *testing.T) {
	analyzer := NewSecurityAnalyzer()

	contractID := "contract123"
	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			{
				EventType:  "require_auth",
				ContractID: &contractID,
				Topics:     []string{"require_auth"},
				Data:       "auth_data",
			},
			{
				EventType:  "storage_write",
				ContractID: &contractID,
				Topics:     []string{"user_input_key"},
				Data:       "some_data",
			},
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Len(t, violations, 1)
	assert.Equal(t, "UserInputKeyedStorage", violations[0].Type)
	assert.Equal(t, "medium", violations[0].Severity)
	assert.Contains(t, violations[0].Description, "caller-supplied input")
}

func TestSecurityAnalyzer_StorageGrowth_CustomThreshold(t *testing.T) {
	analyzer := NewSecurityAnalyzerWithConfig(StorageGrowthConfig{
		GrowthThresholdBytes: 50,
	})

	contractID := "contract123"
	resp := &simulator.SimulationResponse{
		Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{
			{
				EventType:  "require_auth",
				ContractID: &contractID,
				Topics:     []string{"require_auth"},
				Data:       "auth_data",
			},
			{
				EventType:  "storage_write",
				ContractID: &contractID,
				Topics:     []string{"write"},
				Data:       strings.Repeat("A", 60), // Exceeds custom 50 threshold
			},
		},
	}

	violations := analyzer.Analyze(resp)
	assert.Len(t, violations, 1)
	assert.Equal(t, "UnboundedStorageGrowth", violations[0].Type)
}
