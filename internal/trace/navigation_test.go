// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"
)

func TestExecutionTrace_Navigation(t *testing.T) {
	trace := NewExecutionTrace("test-tx-hash", 3)

	// Add some test states
	states := []ExecutionState{
		{Operation: "init", ContractID: "contract1", Function: "initialize"},
		{Operation: "call", ContractID: "contract1", Function: "transfer", Arguments: []interface{}{"100"}},
		{Operation: "call", ContractID: "contract2", Function: "validate"},
		{Operation: "return", ReturnValue: "success"},
		{Operation: "error", Error: "insufficient balance"},
	}

	for _, state := range states {
		trace.AddState(state)
	}

	// Test forward navigation
	state, err := trace.StepForward()
	if err != nil {
		t.Fatalf("StepForward failed: %v", err)
	}
	if state.Step != 1 {
		t.Errorf("Expected step 1, got %d", state.Step)
	}
	if state.Operation != "call" {
		t.Errorf("Expected operation 'call', got '%s'", state.Operation)
	}

	// Test backward navigation
	state, err = trace.StepBackward()
	if err != nil {
		t.Fatalf("StepBackward failed: %v", err)
	}
	if state.Step != 0 {
		t.Errorf("Expected step 0, got %d", state.Step)
	}

	// Test jump to step
	state, err = trace.JumpToStep(3)
	if err != nil {
		t.Fatalf("JumpToStep failed: %v", err)
	}
	if state.Step != 3 {
		t.Errorf("Expected step 3, got %d", state.Step)
	}
	if state.ReturnValue != "success" {
		t.Errorf("Expected return value 'success', got '%v'", state.ReturnValue)
	}

	// Test boundary conditions
	_, err = trace.StepForward()
	if err != nil {
		t.Fatalf("StepForward failed: %v", err)
	}

	_, err = trace.StepForward() // Should fail at last step
	if err == nil {
		t.Error("Expected error when stepping forward from last step")
	}

	// Jump to first step and test backward boundary
	_, err = trace.JumpToStep(0)
	if err != nil {
		t.Fatalf("JumpToStep(0) failed: %v", err)
	}

	_, err = trace.StepBackward() // Should fail at first step
	if err == nil {
		t.Error("Expected error when stepping backward from first step")
	}
}

func TestExecutionTrace_Snapshots(t *testing.T) {
	trace := NewExecutionTrace("test-tx-hash", 2) // Snapshot every 2 steps

	// Add states with host state changes
	states := []ExecutionState{
		{Operation: "init", HostState: map[string]interface{}{"balance": 1000}},
		{Operation: "call1", HostState: map[string]interface{}{"balance": 900}},
		{Operation: "call2", HostState: map[string]interface{}{"balance": 800, "counter": 1}},
		{Operation: "call3", HostState: map[string]interface{}{"balance": 700, "counter": 2}},
		{Operation: "call4", HostState: map[string]interface{}{"balance": 600, "counter": 3}},
	}

	for _, state := range states {
		trace.AddState(state)
	}

	// Should have snapshots at steps 0, 2, 4
	expectedSnapshots := 3
	if len(trace.Snapshots) != expectedSnapshots {
		t.Errorf("Expected %d snapshots, got %d", expectedSnapshots, len(trace.Snapshots))
	}

	// Test state reconstruction
	reconstructed, err := trace.ReconstructStateAt(3)
	if err != nil {
		t.Fatalf("ReconstructStateAt failed: %v", err)
	}

	if reconstructed.Step != 3 {
		t.Errorf("Expected reconstructed step 3, got %d", reconstructed.Step)
	}

	balance, ok := reconstructed.HostState["balance"]
	if !ok {
		t.Error("Expected balance in reconstructed state")
	}
	if balance != 700 {
		t.Errorf("Expected balance 700, got %v", balance)
	}

	counter, ok := reconstructed.HostState["counter"]
	if !ok {
		t.Error("Expected counter in reconstructed state")
	}
	if counter != 2 {
		t.Errorf("Expected counter 2, got %v", counter)
	}
}

func TestExecutionTrace_NavigationInfo(t *testing.T) {
	trace := NewExecutionTrace("test-tx-hash", 5)

	// Add some states
	for i := 0; i < 10; i++ {
		trace.AddState(ExecutionState{
			Operation: "step",
			HostState: map[string]interface{}{"step": i},
		})
	}

	// Test navigation info at start
	info := trace.GetNavigationInfo()
	if info["total_steps"] != 10 {
		t.Errorf("Expected total_steps 10, got %v", info["total_steps"])
	}
	if info["current_step"] != 0 {
		t.Errorf("Expected current_step 0, got %v", info["current_step"])
	}
	if info["can_step_back"] != false {
		t.Errorf("Expected can_step_back false, got %v", info["can_step_back"])
	}
	if info["can_step_forward"] != true {
		t.Errorf("Expected can_step_forward true, got %v", info["can_step_forward"])
	}

	// Move to middle and test again
	_, err := trace.JumpToStep(5)
	if err != nil {
		t.Fatalf("JumpToStep(5) failed: %v", err)
	}
	info = trace.GetNavigationInfo()
	if info["current_step"] != 5 {
		t.Errorf("Expected current_step 5, got %v", info["current_step"])
	}
	if info["can_step_back"] != true {
		t.Errorf("Expected can_step_back true, got %v", info["can_step_back"])
	}
	if info["can_step_forward"] != true {
		t.Errorf("Expected can_step_forward true, got %v", info["can_step_forward"])
	}

	// Move to end and test
	_, err = trace.JumpToStep(9)
	if err != nil {
		t.Fatalf("JumpToStep(9) failed: %v", err)
	}
	info = trace.GetNavigationInfo()
	if info["can_step_forward"] != false {
		t.Errorf("Expected can_step_forward false, got %v", info["can_step_forward"])
	}
}

func TestExecutionTrace_JSONSerialization(t *testing.T) {
	original := NewExecutionTrace("test-tx-hash", 3)

	// Add some states
	states := []ExecutionState{
		{Operation: "init", ContractID: "contract1"},
		{Operation: "call", Function: "test", Arguments: []interface{}{"arg1", 42}, RawArguments: []string{"AAAAAQ==", "AAAAAg=="}},
		{Operation: "return", ReturnValue: "result", RawReturnValue: "AAAAAw=="},
	}

	for _, state := range states {
		original.AddState(state)
	}

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize from JSON
	restored, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	// Verify restoration
	if restored.TransactionHash != original.TransactionHash {
		t.Errorf("Transaction hash mismatch")
	}
	if len(restored.States) != len(original.States) {
		t.Errorf("States count mismatch")
	}
	if len(restored.Snapshots) != len(original.Snapshots) {
		t.Errorf("Snapshots count mismatch")
	}

	// Test navigation on restored trace
	state, err := restored.StepForward()
	if err != nil {
		t.Fatalf("Navigation failed on restored trace: %v", err)
	}
	if state.Operation != "call" {
		t.Errorf("Expected operation 'call', got '%s'", state.Operation)
	}
	if len(state.RawArguments) != 2 || state.RawArguments[0] != "AAAAAQ==" {
		t.Errorf("RawArguments not restored correctly")
	}

	state, _ = restored.StepForward()
	if state.RawReturnValue != "AAAAAw==" {
		t.Errorf("RawReturnValue not restored correctly")
	}
}

func TestExecutionTrace_StateReconstruction(t *testing.T) {
	trace := NewExecutionTrace("test-tx-hash", 2)

	// Create a sequence of states that modify memory
	states := []ExecutionState{
		{Operation: "init", Memory: map[string]interface{}{"var1": 0, "var2": "initial"}},
		{Operation: "set1", Memory: map[string]interface{}{"var1": 10}},
		{Operation: "set2", Memory: map[string]interface{}{"var1": 20, "var3": true}},
		{Operation: "set3", Memory: map[string]interface{}{"var2": "modified"}},
		{Operation: "set4", Memory: map[string]interface{}{"var1": 30}},
	}

	for _, state := range states {
		trace.AddState(state)
	}

	// Test reconstruction at step 2
	intermediate, err := trace.ReconstructStateAt(2)
	if err != nil {
		t.Fatalf("ReconstructStateAt(2) failed: %v", err)
	}

	// At step 2, we should have: var1=20, var2="initial", var3=true
	if intermediate.Memory["var1"] != 20 {
		t.Errorf("Expected var1=20 at step 2, got %v", intermediate.Memory["var1"])
	}
	if intermediate.Memory["var2"] != "initial" {
		t.Errorf("Expected var2='initial' at step 2, got %v", intermediate.Memory["var2"])
	}
	if intermediate.Memory["var3"] != true {
		t.Errorf("Expected var3=true at step 2, got %v", intermediate.Memory["var3"])
	}

	// Test final reconstruction at step 4
	reconstructed, err := trace.ReconstructStateAt(4)
	if err != nil {
		t.Fatalf("ReconstructStateAt(4) failed: %v", err)
	}

	// At step 4: var1=30, var2="modified", var3=true
	if reconstructed.Memory["var1"] != 30 {
		t.Errorf("Expected var1=30 at step 4, got %v", reconstructed.Memory["var1"])
	}
	if reconstructed.Memory["var2"] != "modified" {
		t.Errorf("Expected var2='modified' at step 4, got %v", reconstructed.Memory["var2"])
	}
	if reconstructed.Memory["var3"] != true {
		t.Errorf("Expected var3=true at step 4, got %v", reconstructed.Memory["var3"])
	}
}

func TestExecutionTrace_FilteredNavigation(t *testing.T) {
	trace := NewExecutionTrace("filter-test", 10)
	states := []ExecutionState{
		{Operation: "contract_call", ContractID: "C1", Function: "transfer"},
		{Operation: "host_fn", Function: "get_ledger_entry"},
		{Operation: "contract_call", ContractID: "C2", Function: "swap"},
		{Operation: "error", Error: "Wasm Trap: unreachable"},
		{Operation: "call", Function: "require_auth"},
		{Operation: "contract_call", ContractID: "C1", Function: "balance"},
	}
	for _, s := range states {
		trace.AddState(s)
	}

	if n := trace.FilteredStepCount(EventTypeContractCall); n != 3 {
		t.Errorf("FilteredStepCount(contract_call) = %d, want 3", n)
	}
	if n := trace.FilteredStepCount(EventTypeTrap); n != 1 {
		t.Errorf("FilteredStepCount(trap) = %d, want 1", n)
	}
	if n := trace.FilteredStepCount(EventTypeAuth); n != 1 {
		t.Errorf("FilteredStepCount(auth) = %d, want 1", n)
	}
	if n := trace.FilteredStepCount(EventTypeHostFunction); n != 1 {
		t.Errorf("FilteredStepCount(host_function) = %d, want 1", n)
	}

	// Start at 0 (contract_call). Filtered forward with contract_call should go to step 2, then 5
	state, err := trace.FilteredStepForward(EventTypeContractCall)
	if err != nil {
		t.Fatalf("FilteredStepForward(contract_call): %v", err)
	}
	if state.Step != 2 {
		t.Errorf("FilteredStepForward first: step = %d, want 2", state.Step)
	}
	state, err = trace.FilteredStepForward(EventTypeContractCall)
	if err != nil {
		t.Fatalf("FilteredStepForward(contract_call) second: %v", err)
	}
	if state.Step != 5 {
		t.Errorf("FilteredStepForward second: step = %d, want 5", state.Step)
	}
	_, err = trace.FilteredStepForward(EventTypeContractCall)
	if err == nil {
		t.Error("FilteredStepForward at end should return error")
	}

	// Backward from 5: should go to 2, then 0
	state, err = trace.FilteredStepBackward(EventTypeContractCall)
	if err != nil {
		t.Fatalf("FilteredStepBackward: %v", err)
	}
	if state.Step != 2 {
		t.Errorf("FilteredStepBackward first: step = %d, want 2", state.Step)
	}
	state, err = trace.FilteredStepBackward(EventTypeContractCall)
	if err != nil {
		t.Fatalf("FilteredStepBackward second: %v", err)
	}
	if state.Step != 0 {
		t.Errorf("FilteredStepBackward second: step = %d, want 0", state.Step)
	}
	_, err = trace.FilteredStepBackward(EventTypeContractCall)
	if err == nil {
		t.Error("FilteredStepBackward at start should return error")
	}

	// Empty filter: same as normal step
	trace.CurrentStep = 0
	state, err = trace.FilteredStepForward("")
	if err != nil {
		t.Fatalf("FilteredStepForward(\"\") failed: %v", err)
	}
	if state.Step != 1 {
		t.Errorf("FilteredStepForward(empty) = step %d, want 1", state.Step)
	}

	// FilteredCurrentIndex
	trace.CurrentStep = 2
	if idx := trace.FilteredCurrentIndex(EventTypeContractCall); idx != 2 {
		t.Errorf("FilteredCurrentIndex at step 2 = %d, want 2", idx)
	}
	trace.CurrentStep = 1
	if idx := trace.FilteredCurrentIndex(EventTypeContractCall); idx != 0 {
		t.Errorf("FilteredCurrentIndex at non-matching step = %d, want 0", idx)
	}
}

// TestExecutionTrace_AddStateDoesNotBlock verifies that AddState with a large
// number of states does not synchronously reconstruct snapshot state. The
// snapshot HostState must be nil (un-built) immediately after AddState.
func TestExecutionTrace_AddStateDoesNotBlock(t *testing.T) {
	const numStates = 10_000
	trace := NewExecutionTrace("large-trace-hash", 100)

	for i := 0; i < numStates; i++ {
		trace.AddState(ExecutionState{
			Operation: "step",
			HostState: map[string]interface{}{"counter": i},
		})
	}

	if len(trace.States) != numStates {
		t.Fatalf("expected %d states, got %d", numStates, len(trace.States))
	}

	// Snapshots should have been registered (one per 100 steps)
	expectedSnapshots := numStates / 100
	if len(trace.Snapshots) != expectedSnapshots {
		t.Fatalf("expected %d snapshots, got %d", expectedSnapshots, len(trace.Snapshots))
	}

	// None of the snapshot HostState maps should be populated yet (lazy init).
	for i, snap := range trace.Snapshots {
		if snap.built {
			t.Errorf("snapshot %d was eagerly built; expected lazy initialization", i)
		}
		if snap.HostState != nil {
			t.Errorf("snapshot %d HostState is non-nil before first read", i)
		}
	}
}

// TestExecutionTrace_LazySnapshotMaterializesOnRead verifies that snapshot
// data IS available after ReconstructStateAt has been called.
func TestExecutionTrace_LazySnapshotMaterializesOnRead(t *testing.T) {
	trace := NewExecutionTrace("lazy-test", 5)

	for i := 0; i < 15; i++ {
		trace.AddState(ExecutionState{
			Operation: "step",
			HostState: map[string]interface{}{"v": i},
		})
	}

	// Before any reconstruction, snapshot at index 0 (step 0) should be unbuilt.
	if trace.Snapshots[0].built {
		t.Error("snapshot[0] should not be built before first ReconstructStateAt call")
	}

	// Reconstruct step 7; this will need the snapshot at step 5.
	reconstructed, err := trace.ReconstructStateAt(7)
	if err != nil {
		t.Fatalf("ReconstructStateAt(7) failed: %v", err)
	}
	if reconstructed.Step != 7 {
		t.Errorf("expected step 7, got %d", reconstructed.Step)
	}

	// The snapshot at step 5 (index 1) should now be built.
	// Find it by step number.
	var snap5 *StateSnapshot
	for i := range trace.Snapshots {
		if trace.Snapshots[i].Step == 5 {
			snap5 = &trace.Snapshots[i]
			break
		}
	}
	if snap5 == nil {
		t.Fatal("snapshot at step 5 not found")
	}
	if !snap5.built {
		t.Error("snapshot at step 5 should be built after ReconstructStateAt(7)")
	}
	if snap5.HostState == nil {
		t.Error("snapshot at step 5 HostState should be populated")
	}

	// v at step 7 should be 7 (last write before step 7 inclusive is state[7].v=7).
	if v, ok := reconstructed.HostState["v"]; !ok || v != 7 {
		t.Errorf("expected HostState[v]=7, got %v", v)
	}
}
