// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import "context"

type MockRunner struct {
	RunFunc func(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error)
}

func (m *MockRunner) Run(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, req)
	}
	return &SimulationResponse{Status: "success"}, nil
}

func NewMockRunner(fn func(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error)) *MockRunner {
	return &MockRunner{RunFunc: fn}
}

func NewDefaultMockRunner() *MockRunner {
	return &MockRunner{
		RunFunc: func(ctx context.Context, req *SimulationRequest) (*SimulationResponse, error) {
			return &SimulationResponse{
				Status: "success",
				Events: []string{},
				Logs:   []string{},
			}, nil
		},
	}
}
