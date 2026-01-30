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

package trace

import (
	"encoding/json"
	"fmt"
	"time"
)

// ExecutionState represents the state at a specific point in execution
type ExecutionState struct {
	Step        int                    `json:"step"`
	Timestamp   time.Time              `json:"timestamp"`
	Operation   string                 `json:"operation"`
	ContractID  string                 `json:"contract_id,omitempty"`
	Function    string                 `json:"function,omitempty"`
	Arguments   []interface{}          `json:"arguments,omitempty"`
	ReturnValue interface{}            `json:"return_value,omitempty"`
	Error       string                 `json:"error,omitempty"`
	HostState   map[string]interface{} `json:"host_state,omitempty"`
	Memory      map[string]interface{} `json:"memory,omitempty"`
}

// StateSnapshot represents a complete state snapshot for efficient reconstruction
type StateSnapshot struct {
	Step      int                    `json:"step"`
	Timestamp time.Time              `json:"timestamp"`
	HostState map[string]interface{} `json:"host_state"`
	Memory    map[string]interface{} `json:"memory"`
	CallStack []string               `json:"call_stack"`
}

// ExecutionTrace manages the complete execution trace with bi-directional navigation
type ExecutionTrace struct {
	TransactionHash  string           `json:"transaction_hash"`
	StartTime        time.Time        `json:"start_time"`
	EndTime          time.Time        `json:"end_time"`
	States           []ExecutionState `json:"states"`
	Snapshots        []StateSnapshot  `json:"snapshots"`
	CurrentStep      int              `json:"current_step"`
	SnapshotInterval int              `json:"snapshot_interval"`
}

// NewExecutionTrace creates a new execution trace
func NewExecutionTrace(txHash string, snapshotInterval int) *ExecutionTrace {
	if snapshotInterval <= 0 {
		snapshotInterval = 10 // Default: snapshot every 10 steps
	}

	return &ExecutionTrace{
		TransactionHash:  txHash,
		StartTime:        time.Now(),
		States:           make([]ExecutionState, 0),
		Snapshots:        make([]StateSnapshot, 0),
		CurrentStep:      0,
		SnapshotInterval: snapshotInterval,
	}
}

// AddState adds a new execution state and creates snapshots as needed
func (t *ExecutionTrace) AddState(state ExecutionState) {
	state.Step = len(t.States)
	state.Timestamp = time.Now()
	t.States = append(t.States, state)

	// Create snapshot at intervals
	if state.Step%t.SnapshotInterval == 0 {
		// Reconstruct the complete state up to this point for the snapshot
		reconstructed, err := t.reconstructStateUpTo(state.Step)
		if err == nil {
			snapshot := StateSnapshot{
				Step:      state.Step,
				Timestamp: state.Timestamp,
				HostState: copyMap(reconstructed.HostState),
				Memory:    copyMap(reconstructed.Memory),
				CallStack: t.getCurrentCallStack(),
			}
			t.Snapshots = append(t.Snapshots, snapshot)
		}
	}
}

// reconstructStateUpTo is a helper that reconstructs state without using snapshots
func (t *ExecutionTrace) reconstructStateUpTo(step int) (*ExecutionState, error) {
	if step < 0 || step >= len(t.States) {
		return nil, fmt.Errorf("step %d out of range", step)
	}

	reconstructedState := &ExecutionState{
		Step:      step,
		HostState: make(map[string]interface{}),
		Memory:    make(map[string]interface{}),
	}

	// Apply all state changes from 0 to step (inclusive)
	for i := 0; i <= step; i++ {
		state := &t.States[i]

		// Update metadata from target step
		if i == step {
			reconstructedState.Timestamp = state.Timestamp
			reconstructedState.Operation = state.Operation
			reconstructedState.ContractID = state.ContractID
			reconstructedState.Function = state.Function
			reconstructedState.Arguments = state.Arguments
			reconstructedState.ReturnValue = state.ReturnValue
			reconstructedState.Error = state.Error
		}

		// Accumulate state changes
		if state.HostState != nil {
			for k, v := range state.HostState {
				reconstructedState.HostState[k] = v
			}
		}
		if state.Memory != nil {
			for k, v := range state.Memory {
				reconstructedState.Memory[k] = v
			}
		}
	}

	return reconstructedState, nil
}

// StepForward moves to the next execution step
func (t *ExecutionTrace) StepForward() (*ExecutionState, error) {
	if t.CurrentStep >= len(t.States)-1 {
		return nil, fmt.Errorf("already at the last step")
	}

	t.CurrentStep++
	return &t.States[t.CurrentStep], nil
}

// StepBackward moves to the previous execution step
func (t *ExecutionTrace) StepBackward() (*ExecutionState, error) {
	if t.CurrentStep <= 0 {
		return nil, fmt.Errorf("already at the first step")
	}

	t.CurrentStep--
	return &t.States[t.CurrentStep], nil
}

// JumpToStep jumps directly to a specific step
func (t *ExecutionTrace) JumpToStep(step int) (*ExecutionState, error) {
	if step < 0 || step >= len(t.States) {
		return nil, fmt.Errorf("step %d out of range [0, %d]", step, len(t.States)-1)
	}

	t.CurrentStep = step
	return &t.States[t.CurrentStep], nil
}

// GetCurrentState returns the current execution state
func (t *ExecutionTrace) GetCurrentState() (*ExecutionState, error) {
	if t.CurrentStep < 0 || t.CurrentStep >= len(t.States) {
		return nil, fmt.Errorf("invalid current step: %d", t.CurrentStep)
	}

	return &t.States[t.CurrentStep], nil
}

// ReconstructStateAt reconstructs the complete state at a given step
func (t *ExecutionTrace) ReconstructStateAt(step int) (*ExecutionState, error) {
	if step < 0 || step >= len(t.States) {
		return nil, fmt.Errorf("step %d out of range", step)
	}

	// Find the nearest snapshot before or at the target step
	var baseSnapshot *StateSnapshot
	for i := len(t.Snapshots) - 1; i >= 0; i-- {
		if t.Snapshots[i].Step <= step {
			baseSnapshot = &t.Snapshots[i]
			break
		}
	}

	// Start with empty state
	reconstructedState := &ExecutionState{
		Step:      step,
		HostState: make(map[string]interface{}),
		Memory:    make(map[string]interface{}),
	}

	// Start from snapshot or beginning
	startStep := 0
	if baseSnapshot != nil {
		startStep = baseSnapshot.Step
		reconstructedState.HostState = copyMap(baseSnapshot.HostState)
		reconstructedState.Memory = copyMap(baseSnapshot.Memory)
	}

	// Apply state changes from start to target step (inclusive)
	for i := startStep; i <= step; i++ {
		state := &t.States[i]

		// Update metadata from target step
		if i == step {
			reconstructedState.Timestamp = state.Timestamp
			reconstructedState.Operation = state.Operation
			reconstructedState.ContractID = state.ContractID
			reconstructedState.Function = state.Function
			reconstructedState.Arguments = state.Arguments
			reconstructedState.ReturnValue = state.ReturnValue
			reconstructedState.Error = state.Error
		}

		// Accumulate state changes from all steps up to target
		if state.HostState != nil {
			for k, v := range state.HostState {
				reconstructedState.HostState[k] = v
			}
		}
		if state.Memory != nil {
			for k, v := range state.Memory {
				reconstructedState.Memory[k] = v
			}
		}
	}

	return reconstructedState, nil
}

// GetNavigationInfo returns information about navigation possibilities
func (t *ExecutionTrace) GetNavigationInfo() map[string]interface{} {
	return map[string]interface{}{
		"total_steps":      len(t.States),
		"current_step":     t.CurrentStep,
		"can_step_back":    t.CurrentStep > 0,
		"can_step_forward": t.CurrentStep < len(t.States)-1,
		"snapshots_count":  len(t.Snapshots),
	}
}

// ToJSON serializes the trace to JSON
func (t *ExecutionTrace) ToJSON() ([]byte, error) {
	return json.MarshalIndent(t, "", "  ")
}

// FromJSON deserializes the trace from JSON
func FromJSON(data []byte) (*ExecutionTrace, error) {
	var trace ExecutionTrace
	err := json.Unmarshal(data, &trace)
	return &trace, err
}

// Helper functions

func copyMap(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return make(map[string]interface{})
	}

	copy := make(map[string]interface{})
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

func (t *ExecutionTrace) getCurrentCallStack() []string {
	// Extract call stack from current states
	var stack []string
	for i := 0; i <= t.CurrentStep && i < len(t.States); i++ {
		state := &t.States[i]
		if state.Function != "" {
			entry := fmt.Sprintf("%s::%s", state.ContractID, state.Function)
			if len(stack) == 0 || stack[len(stack)-1] != entry {
				stack = append(stack, entry)
			}
		}
	}
	return stack
}
