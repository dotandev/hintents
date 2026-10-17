// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityBoundaryChecker_NoViolations(t *testing.T) {
	events := []string{
		`{"type":"contract_call","contract":"CABC123"}`,
		`{"type":"auth","address":"GABC123","contract":"CABC123"}`,
		`{"type":"storage_write","contract":"CABC123"}`,
	}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	assert.Empty(t, violations)
}

func TestSecurityBoundaryChecker_UnauthorizedStateModification(t *testing.T) {
	events := []string{
		`{"type":"contract_call","contract":"CABC123"}`,
		`{"type":"storage_write","contract":"CABC123"}`,
	}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	assert.Len(t, violations, 1)
	assert.Equal(t, "unauthorized_state_modification", violations[0].Type)
	assert.Equal(t, "high", violations[0].Severity)
	assert.Equal(t, "CABC123", violations[0].Contract)
}

func TestSecurityBoundaryChecker_SACPattern_NoFalsePositive(t *testing.T) {
	testCases := []struct {
		name     string
		contract string
	}{
		{"SAC balance update", "SAC_token_contract"},
		{"SAC allowance update", "stellar_asset_contract"},
		{"SAC admin operation", "token_SAC"},
		{"Stellar asset contract", "stellar_asset"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			events := []string{
				fmt.Sprintf(`{"type":"storage_write","contract":"%s"}`, tc.contract),
			}

			checker := NewSecurityBoundaryChecker()
			violations, err := checker.Analyze(events)

			assert.NoError(t, err)
			assert.Empty(t, violations, "SAC pattern should not trigger violations")
		})
	}
}

func TestSecurityBoundaryChecker_MultipleContracts(t *testing.T) {
	events := []string{
		`{"type":"contract_call","contract":"C1"}`,
		`{"type":"auth","address":"A1","contract":"C1"}`,
		`{"type":"storage_write","contract":"C1"}`,
		`{"type":"contract_call","contract":"C2"}`,
		`{"type":"storage_write","contract":"C2"}`,
	}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	assert.Len(t, violations, 1)
	assert.Equal(t, "C2", violations[0].Contract)
}

func TestSecurityBoundaryChecker_AuthAfterWrite_StillViolation(t *testing.T) {
	events := []string{
		`{"type":"contract_call","contract":"CABC"}`,
		`{"type":"storage_write","contract":"CABC"}`,
		`{"type":"auth","address":"GABC","contract":"CABC"}`,
	}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	assert.Len(t, violations, 1)
}

func TestSecurityBoundaryChecker_InvalidJSON_Skipped(t *testing.T) {
	events := []string{
		`invalid json`,
		`{"type":"contract_call","contract":"CABC"}`,
		`{"type":"storage_write","contract":"CABC"}`,
	}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	assert.Len(t, violations, 1)
}

func TestSecurityBoundaryChecker_EmptyEvents(t *testing.T) {
	events := []string{}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	assert.Empty(t, violations)
}

func TestSecurityBoundaryChecker_UnknownContract_Skipped(t *testing.T) {
	events := []string{
		`{"type":"storage_write","contract":"unknown"}`,
		`{"type":"storage_write","contract":""}`,
	}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	assert.Empty(t, violations)
}

func TestSecurityBoundaryChecker_CrossContractReentrancy_Detected(t *testing.T) {
	// Sequence: C_A calls C_B, which calls back into C_A, which then writes storage
	events := []string{
		`{"type":"contract_call","contract":"CA_VAULT"}`,
		`{"type":"auth","address":"GUSER","contract":"CA_VAULT"}`,
		`{"type":"contract_call","contract":"CB_ROUTER"}`,
		`{"type":"auth","address":"GUSER","contract":"CB_ROUTER"}`,
		`{"type":"contract_call","contract":"CA_VAULT"}`, // Reentrant call into CA_VAULT!
		`{"type":"storage_write","contract":"CA_VAULT"}`,  // Storage modification on reentrant path
	}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	assert.NotEmpty(t, violations)

	foundReentrancy := false
	for _, v := range violations {
		if v.Type == "cross_contract_reentrancy" {
			foundReentrancy = true
			assert.Equal(t, "critical", v.Severity)
			assert.Equal(t, "CA_VAULT", v.Contract)
			assert.Equal(t, 3, v.Details["call_depth"])
			stack, ok := v.Details["call_stack"].([]string)
			assert.True(t, ok)
			assert.Equal(t, []string{"CA_VAULT", "CB_ROUTER", "CA_VAULT"}, stack)
		}
	}
	assert.True(t, foundReentrancy, "expected cross_contract_reentrancy violation")
}

func TestSecurityBoundaryChecker_SequentialCalls_NoReentrancy(t *testing.T) {
	// Sequence: CA calls and finishes, then CB calls and finishes, then CA calls. Not reentrant!
	events := []string{
		`{"type":"contract_call","contract":"CA_VAULT"}`,
		`{"type":"auth","address":"GUSER","contract":"CA_VAULT"}`,
		`{"type":"call_end","contract":"CA_VAULT"}`,
		`{"type":"contract_call","contract":"CB_ROUTER"}`,
		`{"type":"auth","address":"GUSER","contract":"CB_ROUTER"}`,
		`{"type":"call_end","contract":"CB_ROUTER"}`,
		`{"type":"contract_call","contract":"CA_VAULT"}`,
		`{"type":"storage_write","contract":"CA_VAULT"}`,
	}

	checker := NewSecurityBoundaryChecker()
	violations, err := checker.Analyze(events)

	assert.NoError(t, err)
	for _, v := range violations {
		assert.NotEqual(t, "cross_contract_reentrancy", v.Type)
	}
}

func TestCheckInvocationTreeReentrancy_Detected(t *testing.T) {
	// Invocation tree: Root CA calls CB, which calls back into CA with a storage write
	tree := InvocationNode{
		ContractID: "CA_VAULT",
		Function:   "withdraw",
		Calls: []InvocationNode{
			{
				ContractID: "CB_CALLBACK",
				Function:   "on_notify",
				Calls: []InvocationNode{
					{
						ContractID: "CA_VAULT",
						Function:   "withdraw",
						HasWrite:   true,
					},
				},
			},
		},
	}

	violations := CheckInvocationTreeReentrancy(tree)
	assert.Len(t, violations, 1)
	assert.Equal(t, "cross_contract_reentrancy", violations[0].Type)
	assert.Equal(t, "critical", violations[0].Severity)
	assert.Equal(t, "CA_VAULT", violations[0].Contract)
}

func TestCheckInvocationTreeReentrancy_NoWrite_NoViolation(t *testing.T) {
	// Reentrant view-only call without storage write
	tree := InvocationNode{
		ContractID: "CA_VAULT",
		Function:   "get_balance",
		Calls: []InvocationNode{
			{
				ContractID: "CB_ROUTER",
				Function:   "route",
				Calls: []InvocationNode{
					{
						ContractID: "CA_VAULT",
						Function:   "get_balance",
						HasWrite:   false, // Read-only
					},
				},
			},
		},
	}

	violations := CheckInvocationTreeReentrancy(tree)
	assert.Empty(t, violations)
}
