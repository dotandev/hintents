// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSocketListener(t *testing.T) {
	handler := NewDefaultStreamHandler()
	listener, err := NewSocketListener(handler)
	require.NoError(t, err, "Failed to create socket listener")
	defer listener.Close()

	socketPath := listener.GetSocketPath()
	assert.NotEmpty(t, socketPath, "Socket path should not be empty")

	listener.Start()

	// Give the listener time to start
	time.Sleep(100 * time.Millisecond)

	// Test that socket file exists
	assert.FileExists(t, socketPath, "Socket file should exist")
}

func TestDefaultStreamHandler(t *testing.T) {
	handler := NewDefaultStreamHandler()

	// Test event handling
	event := DiagnosticEvent{
		EventType:                "contract",
		ContractID:               stringPtr("CABC123"),
		Topics:                   []string{"transfer"},
		Data:                     "100",
		InSuccessfulContractCall: true,
	}
	handler.OnEvent(event)

	// Test log handling
	handler.OnLog("Test log message")

	// Test budget update
	handler.OnBudgetUpdate(1000, 2048)

	// Test completion
	handler.OnComplete()

	// Verify collected data
	events, logs, cpu, mem, completed, errMsg := handler.GetResults()

	assert.Len(t, events, 1, "Should have 1 event")
	assert.Equal(t, "contract", events[0].EventType)

	assert.Len(t, logs, 1, "Should have 1 log")
	assert.Equal(t, "Test log message", logs[0])

	assert.Equal(t, uint64(1000), cpu, "CPU should be 1000")
	assert.Equal(t, uint64(2048), mem, "Memory should be 2048")

	assert.True(t, completed, "Should be marked as completed")
	assert.Empty(t, errMsg, "Should have no error")
}

func TestStreamHandlerError(t *testing.T) {
	handler := NewDefaultStreamHandler()

	handler.OnError("Test error message")

	_, _, _, _, completed, errMsg := handler.GetResults()

	assert.False(t, completed, "Should not be marked as completed")
	assert.Equal(t, "Test error message", errMsg)
}

func stringPtr(s string) *string {
	return &s
}
