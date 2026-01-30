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
	"errors"
	"testing"
)

func TestMockRunnerDefault(t *testing.T) {
	mock := NewDefaultMockRunner()

	req := &SimulationRequest{
		EnvelopeXdr: "test",
	}

	resp, err := mock.Run(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected success status, got %s", resp.Status)
	}
}

func TestMockRunnerCustom(t *testing.T) {
	customErr := errors.New("custom error")
	mock := NewMockRunner(func(req *SimulationRequest) (*SimulationResponse, error) {
		return nil, customErr
	})

	req := &SimulationRequest{
		EnvelopeXdr: "test",
	}

	resp, err := mock.Run(req)
	if err != customErr {
		t.Errorf("expected custom error, got %v", err)
	}
	if resp != nil {
		t.Error("expected nil response with error")
	}
}

func TestMockRunnerCustomResponse(t *testing.T) {
	expectedResp := &SimulationResponse{
		Status: "failed",
		Error:  "test error",
		Events: []string{"event1", "event2"},
	}
	mock := NewMockRunner(func(req *SimulationRequest) (*SimulationResponse, error) {
		return expectedResp, nil
	})

	req := &SimulationRequest{
		EnvelopeXdr: "test",
	}

	resp, err := mock.Run(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp.Status != expectedResp.Status {
		t.Errorf("expected status %s, got %s", expectedResp.Status, resp.Status)
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}
}

func TestRunnerInterface(t *testing.T) {
	runner := NewDefaultMockRunner()

	if runner == nil {
		t.Error("runner should not be nil")
	}

	req := &SimulationRequest{
		EnvelopeXdr: "test",
	}

	resp, err := runner.Run(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Error("response should not be nil")
	}
}
