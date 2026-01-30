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
