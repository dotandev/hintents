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
		{Operation: "call", Function: "test", Arguments: []interface{}{"arg1", 42}},
		{Operation: "return", ReturnValue: "result"},
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
