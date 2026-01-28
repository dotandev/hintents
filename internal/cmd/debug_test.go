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
	mockRunner := new(MockRunner)
	debugCmd := NewDebugCommand(mockRunner)
	
	assert.NotNil(t, debugCmd)
	assert.Equal(t, "debug", debugCmd.Use[:5])
	
	networkFlag := debugCmd.Flags().Lookup("network")
	assert.NotNil(t, networkFlag)
	assert.Equal(t, "mainnet", networkFlag.DefValue)
	
	rpcURLFlag := debugCmd.Flags().Lookup("rpc-url")
	assert.NotNil(t, rpcURLFlag)
}

func TestMockRunner_ImplementsInterface(t *testing.T) {
	var _ simulator.RunnerInterface = (*MockRunner)(nil)
	
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
	resp, err := mockRunner.Run(req)
	
	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockRunner.AssertExpectations(t)
}

func TestDebugCommand_BackwardCompatibility(t *testing.T) {
	assert.NotNil(t, debugCmd)
	assert.Equal(t, "debug", debugCmd.Use[:5])
	
	networkFlag := debugCmd.Flags().Lookup("network")
	assert.NotNil(t, networkFlag)
	assert.Equal(t, "mainnet", networkFlag.DefValue)
	
	rpcURLFlag := debugCmd.Flags().Lookup("rpc-url")
	assert.NotNil(t, rpcURLFlag)
}
