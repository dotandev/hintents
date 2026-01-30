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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunnerInterface_CompileTimeCheck(t *testing.T) {
	// Verify Runner implements RunnerInterface at compile time
	var _ RunnerInterface = (*Runner)(nil)

	// This test ensures the interface contract is maintained
	assert.True(t, true, "Runner implements RunnerInterface")
}

func TestNewRunnerInterface(t *testing.T) {
	// Test the factory function
	runner, err := NewRunnerInterface()

	// Note: This will fail in the current environment due to missing binary
	// but the interface structure is correct
	if err != nil {
		// Expected in test environment without erst-sim binary
		assert.Contains(t, err.Error(), "erst-sim binary not found")
	} else {
		// If binary exists, verify interface is returned
		assert.NotNil(t, runner)
		assert.Implements(t, (*RunnerInterface)(nil), runner)
	}
}

func TestExampleUsage(t *testing.T) {
	// Create a mock implementation for testing
	mockRunner := &mockRunnerForTest{}

	req := &SimulationRequest{
		EnvelopeXdr:   "test-envelope",
		ResultMetaXdr: "test-meta",
	}

	resp, err := ExampleUsage(mockRunner, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "success", resp.Status)
}

// Simple mock for testing the interface
type mockRunnerForTest struct{}

func (m *mockRunnerForTest) Run(req *SimulationRequest) (*SimulationResponse, error) {
	return &SimulationResponse{
		Status: "success",
		Events: []string{"mock-event"},
	}, nil
}
