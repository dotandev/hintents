// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSimulationResponse_Success(t *testing.T) {
	resp := &SimulationResponse{
		Status: "success",
		Events: []string{
			"contract: CDLZFC3 fn: transfer",
			"event: Transfer completed",
		},
		Logs: []string{
			"Debug: Starting transfer",
			"Debug: Transfer successful",
		},
	}

	root, err := ParseSimulationResponse(resp)

	require.NoError(t, err)
	assert.NotNil(t, root)
	assert.Equal(t, "root", root.ID)
	assert.Equal(t, "simulation", root.Type)
	assert.Contains(t, root.EventData, "success")

	// Should have 2 event nodes + 2 log nodes = 4 children
	assert.Equal(t, 4, len(root.Children))
}

func TestParseSimulationResponse_WithError(t *testing.T) {
	resp := &SimulationResponse{
		Status: "error",
		Error:  "Contract execution failed",
		Events: []string{},
		Logs:   []string{},
	}

	root, err := ParseSimulationResponse(resp)

	require.NoError(t, err)
	assert.NotNil(t, root)

	// Should have 1 error node
	assert.Equal(t, 1, len(root.Children))
	assert.Equal(t, "error", root.Children[0].Type)
	assert.Equal(t, "Contract execution failed", root.Children[0].Error)
}

func TestParseSimulationResponse_Nil(t *testing.T) {
	root, err := ParseSimulationResponse(nil)

	assert.Error(t, err)
	assert.Nil(t, root)
	assert.Contains(t, err.Error(), "nil")
}

func TestParseEvent_ContractID(t *testing.T) {
	event := "contract: CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC fn: transfer"

	node := parseEvent("test-1", event)

	assert.Equal(t, "test-1", node.ID)
	assert.Equal(t, "event", node.Type)
	assert.Equal(t, "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC", node.ContractID)
	assert.Equal(t, "transfer", node.Function)
}

func TestParseEvent_Error(t *testing.T) {
	event := "error: Insufficient balance"

	node := parseEvent("test-1", event)

	assert.Equal(t, "error", node.Type)
	assert.Equal(t, "Insufficient balance", node.Error)
}

func TestParseEvent_Simple(t *testing.T) {
	event := "Simple event message"

	node := parseEvent("test-1", event)

	assert.Equal(t, "test-1", node.ID)
	assert.Equal(t, "event", node.Type)
	assert.Equal(t, event, node.EventData)
}

func TestCreateMockTrace(t *testing.T) {
	root := CreateMockTrace()

	assert.NotNil(t, root)
	assert.Equal(t, "root", root.ID)
	assert.Equal(t, "transaction", root.Type)

	// Should have multiple children
	assert.Greater(t, len(root.Children), 0)

	// Flatten to check total nodes
	allNodes := root.FlattenAll()
	assert.Greater(t, len(allNodes), 5, "Mock trace should have multiple nodes")

	// Check for contract calls
	hasContractCall := false
	hasError := false
	for _, node := range allNodes {
		if node.Type == "contract_call" {
			hasContractCall = true
		}
		if node.Type == "error" {
			hasError = true
		}
	}

	assert.True(t, hasContractCall, "Mock trace should have contract calls")
	assert.True(t, hasError, "Mock trace should have errors")
}

func TestCreateMockTrace_SearchableContent(t *testing.T) {
	root := CreateMockTrace()
	engine := NewSearchEngine()

	allNodes := root.FlattenAll()

	// Test searching for contract ID
	engine.SetQuery("CDLZFC3")
	matches := engine.Search(allNodes)
	assert.Greater(t, len(matches), 0, "Should find contract IDs")

	// Test searching for function
	engine.SetQuery("transfer")
	matches = engine.Search(allNodes)
	assert.Greater(t, len(matches), 0, "Should find function names")

	// Test searching for error
	engine.SetQuery("balance")
	matches = engine.Search(allNodes)
	assert.Greater(t, len(matches), 0, "Should find error messages")
}
