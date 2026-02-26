// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"encoding/json"
	"fmt"
	"time"
)

// ExecutionState represents the state at a specific point in execution
type ExecutionState struct {
	Step           int                    `json:"step"`
	Timestamp      time.Time              `json:"timestamp"`
	Operation      string                 `json:"operation"`
	ContractID     string                 `json:"contract_id,omitempty"`
	Function       string                 `json:"function,omitempty"`
	Arguments      []interface{}          `json:"arguments,omitempty"`
	RawArguments   []string               `json:"raw_arguments,omitempty"`
	ReturnValue    interface{}            `json:"return_value,omitempty"`
	RawReturnValue string                 `json:"raw_return_value,omitempty"`
	Error          string                 `json:"error,omitempty"`
	HostState      map[string]interface{} `json:"host_state,omitempty"`
	Memory          map[string]interface{} `json:"memory,omitempty"`
	WasmInstruction string                 `json:"wasm_instruction,omitempty"`
	Memory         map[string]interface{} `json:"memory,omitempty"`
	Step        int                    `json:"step"`
	Timestamp   time.Time              `json:"timestamp"`
	Operation   string                 `json:"operation"`
	EventType   string                 `json:"event_type,omitempty"` // trap, contract_call, host_function, auth, or empty for inferred
	ContractID  string                 `json:"contract_id,omitempty"`
	Function    string                 `json:"function,omitempty"`
	Arguments   []interface{}          `json:"arguments,omitempty"`
	ReturnValue interface{}            `json:"return_value,omitempty"`
	Error       string                 `json:"error,omitempty"`
	HostState   map[string]interface{} `json:"host_state,omitempty"`
	Memory      map[string]interface{} `json:"memory,omitempty"`
}

// DefaultSnapshotInterval is the number of steps between state snapshots.
// A larger interval reduces ingestion overhead at the cost of slightly more
// replay work during ReconstructStateAt. 100 is well-suited to large traces.
const DefaultSnapshotInterval = 100

// StateSnapshot represents a complete state snapshot for efficient reconstruction.
// HostState and Memory are populated lazily the first time the snapshot is read,
// keeping AddState O(1) regardless of trace size.
type StateSnapshot struct {
	Step      int                    `json:"step"`
	Timestamp time.Time              `json:"timestamp"`
	HostState map[string]interface{} `json:"host_state"`
	Memory    map[string]interface{} `json:"memory"`
	CallStack []string               `json:"call_stack"`
	// built tracks whether HostState/Memory have been populated.
	built bool
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

// NewExecutionTrace creates a new execution trace.
// snapshotInterval controls how frequently lazy state snapshots are registered;
// pass 0 or a negative value to use DefaultSnapshotInterval.
func NewExecutionTrace(txHash string, snapshotInterval int) *ExecutionTrace {
	if snapshotInterval <= 0 {
		snapshotInterval = DefaultSnapshotInterval
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

// ensureSnapshot materialises the HostState and Memory for snapshot at index idx
// if they have not been built yet. This is the only place reconstructStateUpTo is
// called, keeping AddState free of any blocking reconstruction work.
func (t *ExecutionTrace) ensureSnapshot(idx int) {
	s := &t.Snapshots[idx]
	if s.built {
		return
	}
	reconstructed, err := t.reconstructStateUpTo(s.Step)
	if err == nil {
		s.HostState = copyMap(reconstructed.HostState)
		s.Memory = copyMap(reconstructed.Memory)
	}
	s.built = true
}

// AddState adds a new execution state and registers a lazy snapshot as needed.
// Snapshot data (HostState/Memory) is NOT computed here; it is deferred until
// the first call to ReconstructStateAt that needs this snapshot. This keeps
// AddState O(1) and avoids blocking the caller while parsing large traces.
func (t *ExecutionTrace) AddState(state ExecutionState) {
	state.Step = len(t.States)
	state.Timestamp = time.Now()
	t.States = append(t.States, state)

	// Register a snapshot placeholder at each interval boundary.
	// HostState and Memory are intentionally left nil (built=false).
	if state.Step%t.SnapshotInterval == 0 {
		t.Snapshots = append(t.Snapshots, StateSnapshot{
			Step:      state.Step,
			Timestamp: state.Timestamp,
			CallStack: t.getCurrentCallStack(),
			// built: false — populated on first read via ensureSnapshot
		})
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
			reconstructedState.RawArguments = state.RawArguments
			reconstructedState.ReturnValue = state.ReturnValue
			reconstructedState.RawReturnValue = state.RawReturnValue
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

// ReconstructStateAt reconstructs the complete state at a given step.
// It finds the nearest snapshot (materialising it lazily if needed) to
// minimise the number of states that must be replayed.
func (t *ExecutionTrace) ReconstructStateAt(step int) (*ExecutionState, error) {
	if step < 0 || step >= len(t.States) {
		return nil, fmt.Errorf("step %d out of range", step)
	}

	// Find the nearest snapshot before or at the target step
	snapshotIdx := -1
	for i := len(t.Snapshots) - 1; i >= 0; i-- {
		if t.Snapshots[i].Step <= step {
			snapshotIdx = i
			break
		}
	}

	// Start with empty state
	reconstructedState := &ExecutionState{
		Step:      step,
		HostState: make(map[string]interface{}),
		Memory:    make(map[string]interface{}),
	}

	// Start from snapshot or beginning, materialising lazily if needed
	startStep := 0
	if snapshotIdx >= 0 {
		t.ensureSnapshot(snapshotIdx)
		baseSnapshot := &t.Snapshots[snapshotIdx]
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
			reconstructedState.RawArguments = state.RawArguments
			reconstructedState.ReturnValue = state.ReturnValue
			reconstructedState.RawReturnValue = state.RawReturnValue
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

// FilteredStepForward moves to the next step that matches the given event type filter.
// If filter is empty, behaves like StepForward. Returns the new state or error.
func (t *ExecutionTrace) FilteredStepForward(filter string) (*ExecutionState, error) {
	if filter == "" {
		return t.StepForward()
	}
	for i := t.CurrentStep + 1; i < len(t.States); i++ {
		if t.StepMatchesFilter(i, filter) {
			t.CurrentStep = i
			return &t.States[i], nil
		}
	}
	return nil, fmt.Errorf("no more steps matching filter %q", filter)
}

// FilteredStepBackward moves to the previous step that matches the given event type filter.
// If filter is empty, behaves like StepBackward. Returns the new state or error.
func (t *ExecutionTrace) FilteredStepBackward(filter string) (*ExecutionState, error) {
	if filter == "" {
		return t.StepBackward()
	}
	for i := t.CurrentStep - 1; i >= 0; i-- {
		if t.StepMatchesFilter(i, filter) {
			t.CurrentStep = i
			return &t.States[i], nil
		}
	}
	return nil, fmt.Errorf("no earlier steps matching filter %q", filter)
}

// FilteredStepCount returns the number of steps that match the given filter.
// Empty filter returns len(t.States).
func (t *ExecutionTrace) FilteredStepCount(filter string) int {
	if filter == "" {
		return len(t.States)
	}
	n := 0
	for i := 0; i < len(t.States); i++ {
		if t.StepMatchesFilter(i, filter) {
			n++
		}
	}
	return n
}

// FilteredCurrentIndex returns the 1-based index of the current step among steps matching the filter.
// Returns 0 if current step does not match the filter. Empty filter uses natural step index.
func (t *ExecutionTrace) FilteredCurrentIndex(filter string) int {
	if t.CurrentStep < 0 || t.CurrentStep >= len(t.States) {
		return 0
	}
	if filter == "" {
		return t.CurrentStep + 1
	}
	if !t.StepMatchesFilter(t.CurrentStep, filter) {
		return 0
	}
	idx := 0
	for i := 0; i <= t.CurrentStep; i++ {
		if t.StepMatchesFilter(i, filter) {
			idx++
		}
	}
	return idx
}
