// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
)

// RunnerInterface defines the contract for simulator execution
type RunnerInterface interface {
	Run(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error)
	Close() error
}

// NewRunnerInterface creates a RunnerInterface implementation
// This allows for easy swapping between real and mock implementations
func NewRunnerInterface() (RunnerInterface, error) {
	return NewRunner("", false)
}

// ExampleUsage of how commands can accept the interface
func ExampleUsage(ctx context.Context, runner RunnerInterface, req *SimulationRequest) (*SimulationResponse, error) {
	// Commands can now work with any implementation of RunnerInterface
	// This enables easy testing with mocks and flexible production usage
	return runner.Run(ctx, req)
}
