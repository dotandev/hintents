package cmd

import (
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRunner implements simulator.RunnerInterface for testing
type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Run(req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
	args := m.Called(req)
	return args.Get(0).(*simulator.SimulationResponse), args.Error(1)
}

func TestDebugCommand_WithMockRunner(t *testing.T) {
	// Create mock runner
	mockRunner := new(MockRunner)
	
	// Create debug command with mock runner
	debugCmd := NewDebugCommand(mockRunner)
	
	// Verify the command was created successfully
	assert.NotNil(t, debugCmd)
	assert.Equal(t, "debug", debugCmd.Use[:5])
	
	// Verify flags are properly set up
	networkFlag := debugCmd.Flags().Lookup("network")
	assert.NotNil(t, networkFlag)
	assert.Equal(t, "mainnet", networkFlag.DefValue)
	
	rpcURLFlag := debugCmd.Flags().Lookup("rpc-url")
	assert.NotNil(t, rpcURLFlag)
	
	// This test demonstrates that the command can now be tested with a mock
	// without requiring the actual erst-sim binary
}

func TestMockRunner_ImplementsInterface(t *testing.T) {
	// Verify MockRunner implements the interface
	var _ simulator.RunnerInterface = (*MockRunner)(nil)
	
	// Test mock functionality
	mockRunner := new(MockRunner)
	
	req := &simulator.SimulationRequest{
		EnvelopeXdr:   "test-envelope",
		ResultMetaXdr: "test-meta",
	}
	expectedResp := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"test-event"},
	}
	
	mockRunner.On("Run", req).Return(expectedResp, nil)
	
	// Call the mock
	resp, err := mockRunner.Run(req)
	
	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockRunner.AssertExpectations(t)
}

func TestDebugCommand_BackwardCompatibility(t *testing.T) {
	// Test that the original debugCmd still works (backward compatibility)
	assert.NotNil(t, debugCmd)
	assert.Equal(t, "debug", debugCmd.Use[:5])
	
	// Verify flags are still present
	networkFlag := debugCmd.Flags().Lookup("network")
	assert.NotNil(t, networkFlag)
	assert.Equal(t, "mainnet", networkFlag.DefValue)
	
	rpcURLFlag := debugCmd.Flags().Lookup("rpc-url")
	assert.NotNil(t, rpcURLFlag)
}
