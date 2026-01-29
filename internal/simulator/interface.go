package simulator

// Example usage of RunnerInterface for dependency injection

// NewRunnerInterface creates a RunnerInterface implementation
// This allows for easy swapping between real and mock implementations
func NewRunnerInterface() (RunnerInterface, error) {
	return NewRunner()
}

// Example of how commands can accept the interface
func ExampleUsage(runner RunnerInterface, req *SimulationRequest) (*SimulationResponse, error) {
	// Commands can now work with any implementation of RunnerInterface
	// This enables easy testing with mocks and flexible production usage
	return runner.Run(req)
// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

type Runner interface {
	Run(req *SimulationRequest) (*SimulationResponse, error)
}
