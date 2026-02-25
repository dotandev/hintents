// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
)

// MockRunner implements simulator.RunnerInterface for testing
type MockRunner struct {
	RunFunc func(*simulator.SimulationRequest) (*simulator.SimulationResponse, error)
}

func (m *MockRunner) Run(req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
	if m.RunFunc != nil {
		return m.RunFunc(req)
	}
	return &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1"},
		Logs:   []string{"log1"},
	}, nil
}

func TestNewSession(t *testing.T) {
	runner := &MockRunner{}
	rpcClient := rpc.NewClient(rpc.Testnet)
	
	session := NewSession(runner, rpcClient, rpc.Testnet)
	
	if session == nil {
		t.Fatal("Expected session to be created")
	}
	
	if session.ledgerSequence != 1 {
		t.Errorf("Expected initial ledger sequence to be 1, got %d", session.ledgerSequence)
	}
	
	if len(session.ledgerEntries) != 0 {
		t.Errorf("Expected empty ledger entries, got %d", len(session.ledgerEntries))
	}
	
	if session.invocationCount != 0 {
		t.Errorf("Expected invocation count to be 0, got %d", session.invocationCount)
	}
}

func TestGetStateSummary(t *testing.T) {
	runner := &MockRunner{}
	rpcClient := rpc.NewClient(rpc.Testnet)
	session := NewSession(runner, rpcClient, rpc.Testnet)
	
	// Add some entries
	session.ledgerEntries["key1"] = "value1"
	session.ledgerEntries["key2"] = "value2"
	session.ledgerSequence = 10
	session.invocationCount = 5
	
	summary := session.GetStateSummary()
	
	if summary.EntryCount != 2 {
		t.Errorf("Expected entry count 2, got %d", summary.EntryCount)
	}
	
	if summary.LedgerSequence != 10 {
		t.Errorf("Expected ledger sequence 10, got %d", summary.LedgerSequence)
	}
	
	if summary.InvocationCount != 5 {
		t.Errorf("Expected invocation count 5, got %d", summary.InvocationCount)
	}
}

func TestSaveAndLoadState(t *testing.T) {
	runner := &MockRunner{}
	rpcClient := rpc.NewClient(rpc.Testnet)
	session := NewSession(runner, rpcClient, rpc.Testnet)
	
	// Set up some state
	session.ledgerEntries["key1"] = "value1"
	session.ledgerEntries["key2"] = "value2"
	session.ledgerSequence = 42
	session.timestamp = time.Now().Unix()
	
	// Save state
	tmpfile := "test_state.json"
	defer os.Remove(tmpfile)
	
	err := session.SaveState(tmpfile)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}
	
	// Create new session and load state
	newSession := NewSession(runner, rpcClient, rpc.Testnet)
	err = newSession.LoadState(tmpfile)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}
	
	// Verify loaded state
	if len(newSession.ledgerEntries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(newSession.ledgerEntries))
	}
	
	if newSession.ledgerEntries["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %s", newSession.ledgerEntries["key1"])
	}
	
	if newSession.ledgerSequence != 42 {
		t.Errorf("Expected ledger sequence 42, got %d", newSession.ledgerSequence)
	}
}

func TestResetState(t *testing.T) {
	runner := &MockRunner{}
	rpcClient := rpc.NewClient(rpc.Testnet)
	session := NewSession(runner, rpcClient, rpc.Testnet)
	
	// Set initial state
	session.ledgerEntries["initial"] = "value"
	session.ledgerSequence = 5
	session.initialState = &LedgerState{
		Entries:        map[string]string{"initial": "value"},
		LedgerSequence: 5,
		Timestamp:      session.timestamp,
	}
	
	// Modify state
	session.ledgerEntries["new"] = "newvalue"
	session.ledgerSequence = 10
	session.invocationCount = 3
	
	// Reset
	session.ResetState()
	
	// Verify reset
	if len(session.ledgerEntries) != 1 {
		t.Errorf("Expected 1 entry after reset, got %d", len(session.ledgerEntries))
	}
	
	if session.ledgerEntries["initial"] != "value" {
		t.Errorf("Expected initial entry to be restored")
	}
	
	if _, exists := session.ledgerEntries["new"]; exists {
		t.Errorf("Expected new entry to be removed after reset")
	}
	
	if session.ledgerSequence != 5 {
		t.Errorf("Expected ledger sequence to be reset to 5, got %d", session.ledgerSequence)
	}
	
	if session.invocationCount != 0 {
		t.Errorf("Expected invocation count to be reset to 0, got %d", session.invocationCount)
	}
}

func TestInvoke(t *testing.T) {
	runner := &MockRunner{
		RunFunc: func(req *simulator.SimulationRequest) (*simulator.SimulationResponse, error) {
			return &simulator.SimulationResponse{
				Status: "success",
				Events: []string{"transfer_event"},
				Logs:   []string{"transferred 100 tokens"},
			}, nil
		},
	}
	
	rpcClient := rpc.NewClient(rpc.Testnet)
	session := NewSession(runner, rpcClient, rpc.Testnet)
	
	ctx := context.Background()
	
	// Note: This will fail with "envelope building not yet implemented"
	// which is expected until we implement proper XDR building
	_, err := session.Invoke(ctx, "CAAAA...", "transfer", []string{"alice", "bob", "100"})
	
	// We expect an error about envelope building
	if err == nil {
		t.Error("Expected error about envelope building not implemented")
	}
}

func TestUpdateLedgerState(t *testing.T) {
	runner := &MockRunner{}
	rpcClient := rpc.NewClient(rpc.Testnet)
	session := NewSession(runner, rpcClient, rpc.Testnet)
	
	initialSequence := session.ledgerSequence
	initialTimestamp := session.timestamp
	
	resp := &simulator.SimulationResponse{
		Status: "success",
	}
	
	session.updateLedgerState(resp)
	
	// Verify sequence incremented
	if session.ledgerSequence != initialSequence+1 {
		t.Errorf("Expected ledger sequence to increment, got %d", session.ledgerSequence)
	}
	
	// Verify timestamp updated
	if session.timestamp <= initialTimestamp {
		t.Errorf("Expected timestamp to be updated")
	}
}

func TestLoadStateInvalidFile(t *testing.T) {
	runner := &MockRunner{}
	rpcClient := rpc.NewClient(rpc.Testnet)
	session := NewSession(runner, rpcClient, rpc.Testnet)
	
	err := session.LoadState("nonexistent.json")
	if err == nil {
		t.Error("Expected error when loading nonexistent file")
	}
}

func TestLoadStateInvalidJSON(t *testing.T) {
	runner := &MockRunner{}
	rpcClient := rpc.NewClient(rpc.Testnet)
	session := NewSession(runner, rpcClient, rpc.Testnet)
	
	// Create invalid JSON file
	tmpfile := "invalid.json"
	defer os.Remove(tmpfile)
	
	err := os.WriteFile(tmpfile, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	err = session.LoadState(tmpfile)
	if err == nil {
		t.Error("Expected error when loading invalid JSON")
	}
}
