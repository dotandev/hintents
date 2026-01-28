package simulator

// NewRunnerInterface creates a RunnerInterface implementation
func NewRunnerInterface() (RunnerInterface, error) {
	return NewRunner()
}

// ExampleUsage demonstrates how commands can accept the interface
func ExampleUsage(runner RunnerInterface, req *SimulationRequest) (*SimulationResponse, error) {
	return runner.Run(req)
}
