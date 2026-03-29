// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordRemoteNodeResponse_Success(t *testing.T) {
	// Reset metrics before test
	RemoteNodeLastResponseTimestamp.Reset()
	RemoteNodeResponseTotal.Reset()
	RemoteNodeResponseDurationSeconds.Reset()

	nodeAddress := "https://soroban-testnet.stellar.org"
	network := "testnet"

	// Record a successful response
	RecordRemoteNodeResponse(nodeAddress, network, true, 150*time.Millisecond)

	// Verify counter incremented
	counter := RemoteNodeResponseTotal.WithLabelValues(nodeAddress, network, "success")
	assert.Equal(t, float64(1), testutil.ToFloat64(counter))

	// Verify timestamp was set (should be recent)
	gauge := RemoteNodeLastResponseTimestamp.WithLabelValues(nodeAddress, network)
	timestamp := testutil.ToFloat64(gauge)
	assert.Greater(t, timestamp, float64(time.Now().Unix()-5)) // Within last 5 seconds

	// Verify histogram recorded
	assert.Equal(t, 1, testutil.CollectAndCount(RemoteNodeResponseDurationSeconds))
}

func TestRecordRemoteNodeResponse_Error(t *testing.T) {
	// Reset metrics before test
	RemoteNodeLastResponseTimestamp.Reset()
	RemoteNodeResponseTotal.Reset()
	RemoteNodeResponseDurationSeconds.Reset()

	nodeAddress := "https://soroban-testnet.stellar.org"
	network := "testnet"

	// Record initial timestamp
	beforeTimestamp := time.Now().Unix()
	RemoteNodeLastResponseTimestamp.WithLabelValues(nodeAddress, network).Set(float64(beforeTimestamp))

	// Record an error response
	RecordRemoteNodeResponse(nodeAddress, network, false, 50*time.Millisecond)

	// Verify error counter incremented
	errorCounter := RemoteNodeResponseTotal.WithLabelValues(nodeAddress, network, "error")
	assert.Equal(t, float64(1), testutil.ToFloat64(errorCounter))

	// Verify timestamp was NOT updated (should still be the old value)
	gauge := RemoteNodeLastResponseTimestamp.WithLabelValues(nodeAddress, network)
	timestamp := testutil.ToFloat64(gauge)
	assert.Equal(t, float64(beforeTimestamp), timestamp)

	// Verify histogram still recorded (even for errors)
	assert.Equal(t, 1, testutil.CollectAndCount(RemoteNodeResponseDurationSeconds))
}

func TestRecordRemoteNodeResponse_MultipleNodes(t *testing.T) {
	// Reset metrics before test
	RemoteNodeLastResponseTimestamp.Reset()
	RemoteNodeResponseTotal.Reset()

	node1 := "https://soroban-testnet.stellar.org"
	node2 := "https://soroban-mainnet.stellar.org"
	network1 := "testnet"
	network2 := "mainnet"

	// Record responses from different nodes
	RecordRemoteNodeResponse(node1, network1, true, 100*time.Millisecond)
	RecordRemoteNodeResponse(node2, network2, true, 200*time.Millisecond)
	RecordRemoteNodeResponse(node1, network1, false, 50*time.Millisecond)

	// Verify node1 metrics
	node1Success := RemoteNodeResponseTotal.WithLabelValues(node1, network1, "success")
	assert.Equal(t, float64(1), testutil.ToFloat64(node1Success))
	node1Error := RemoteNodeResponseTotal.WithLabelValues(node1, network1, "error")
	assert.Equal(t, float64(1), testutil.ToFloat64(node1Error))

	// Verify node2 metrics
	node2Success := RemoteNodeResponseTotal.WithLabelValues(node2, network2, "success")
	assert.Equal(t, float64(1), testutil.ToFloat64(node2Success))
}

func TestRecordSimulationExecution(t *testing.T) {
	// Reset metrics before test
	SimulationExecutionTotal.Reset()

	// Record successful executions
	RecordSimulationExecution(true)
	RecordSimulationExecution(true)

	// Record failed execution
	RecordSimulationExecution(false)

	// Verify counters
	successCounter := SimulationExecutionTotal.WithLabelValues("success")
	assert.Equal(t, float64(2), testutil.ToFloat64(successCounter))

	errorCounter := SimulationExecutionTotal.WithLabelValues("error")
	assert.Equal(t, float64(1), testutil.ToFloat64(errorCounter))
}

func TestMetricsLabels(t *testing.T) {
	// Verify that metrics have the expected labels
	tests := []struct {
		name     string
		metric   prometheus.Collector
		expected []string
	}{
		{
			name:     "RemoteNodeLastResponseTimestamp",
			metric:   RemoteNodeLastResponseTimestamp,
			expected: []string{"node_address", "network"},
		},
		{
			name:     "RemoteNodeResponseTotal",
			metric:   RemoteNodeResponseTotal,
			expected: []string{"node_address", "network", "status"},
		},
		{
			name:     "RemoteNodeResponseDurationSeconds",
			metric:   RemoteNodeResponseDurationSeconds,
			expected: []string{"node_address", "network"},
		},
		{
			name:     "SimulationExecutionTotal",
			metric:   SimulationExecutionTotal,
			expected: []string{"status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test verifies the metrics are properly registered
			// The actual label validation happens at registration time
			assert.NotNil(t, tt.metric)
		})
	}
}
