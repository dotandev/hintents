// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import "context"

// RunnerInterface defines the contract for simulator execution
type RunnerInterface interface {
	Run(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error)
}

// NewRunnerInterface creates a RunnerInterface implementation
func NewRunnerInterface() (RunnerInterface, error) {
	return NewRunner()
}

// ExampleUsage demonstrates how to use the RunnerInterface
func ExampleUsage(ctx context.Context, runner RunnerInterface, req *SimulationRequest) (*SimulationResponse, error) {
	return runner.Run(ctx, req)
}
