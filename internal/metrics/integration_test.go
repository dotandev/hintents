// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package metrics

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsEndpoint verifies that metrics are properly exposed via HTTP
func TestMetricsEndpoint(t *testing.T) {
	// Reset metrics
	RemoteNodeLastResponseTimestamp.Reset()
	RemoteNodeResponseTotal.Reset()
	RemoteNodeResponseDurationSeconds.Reset()
	SimulationExecutionTotal.Reset()

	// Record some test data
	nodeAddress := "https://soroban-testnet.stellar.org"
	network := "testnet"

	RecordRemoteNodeResponse(nodeAddress, network, true, 150*time.Millisecond)
	RecordRemoteNodeResponse(nodeAddress, network, true, 200*time.Millisecond)
	RecordRemoteNodeResponse(nodeAddress, network, false, 50*time.Millisecond)
	RecordSimulationExecution(true)
	RecordSimulationExecution(false)

	// Create test HTTP server with metrics handler
	server := httptest.NewServer(promhttp.Handler())
	defer server.Close()

	// Fetch metrics
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	metricsOutput := string(body)

	// Verify all expected metrics are present
	assert.Contains(t, metricsOutput, "remote_node_last_response_timestamp_seconds")
	assert.Contains(t, metricsOutput, "remote_node_response_total")
	assert.Contains(t, metricsOutput, "remote_node_response_duration_seconds")
	assert.Contains(t, metricsOutput, "simulation_execution_total")

	// Verify labels are present
	assert.Contains(t, metricsOutput, `node_address="https://soroban-testnet.stellar.org"`)
	assert.Contains(t, metricsOutput, `network="testnet"`)
	assert.Contains(t, metricsOutput, `status="success"`)
	assert.Contains(t, metricsOutput, `status="error"`)

	// Verify metric values
	assert.Contains(t, metricsOutput, `remote_node_response_total{network="testnet",node_address="https://soroban-testnet.stellar.org",status="success"} 2`)
	assert.Contains(t, metricsOutput, `remote_node_response_total{network="testnet",node_address="https://soroban-testnet.stellar.org",status="error"} 1`)
	assert.Contains(t, metricsOutput, `simulation_execution_total{status="success"} 1`)
	assert.Contains(t, metricsOutput, `simulation_execution_total{status="error"} 1`)
}

// TestMetricsStalenessDetection verifies that timestamp metrics enable staleness detection
func TestMetricsStalenessDetection(t *testing.T) {
	// Reset metrics
	RemoteNodeLastResponseTimestamp.Reset()

	nodeAddress := "https://soroban-testnet.stellar.org"
	network := "testnet"

	// Record initial successful response
	beforeTime := time.Now().Unix()
	RecordRemoteNodeResponse(nodeAddress, network, true, 100*time.Millisecond)

	// Create test HTTP server
	server := httptest.NewServer(promhttp.Handler())
	defer server.Close()

	// Fetch metrics
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	metricsOutput := string(body)

	// Extract timestamp from metrics output
	lines := strings.Split(metricsOutput, "\n")
	var timestamp float64
	for _, line := range lines {
		if strings.Contains(line, "remote_node_last_response_timestamp_seconds") &&
			strings.Contains(line, nodeAddress) &&
			!strings.HasPrefix(line, "#") {
			// Parse the timestamp value
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				_, err := fmt.Sscanf(parts[len(parts)-1], "%f", &timestamp)
				require.NoError(t, err)
				break
			}
		}
	}

	// Verify timestamp is recent (within last 5 seconds)
	assert.Greater(t, timestamp, float64(beforeTime-5))
	assert.LessOrEqual(t, timestamp, float64(time.Now().Unix()+1))

	// Wait a bit
	time.Sleep(2 * time.Second)

	// Record error response (should NOT update timestamp)
	RecordRemoteNodeResponse(nodeAddress, network, false, 50*time.Millisecond)

	// Fetch metrics again
	resp2, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)

	metricsOutput2 := string(body2)

	// Extract timestamp again
	lines2 := strings.Split(metricsOutput2, "\n")
	var timestamp2 float64
	for _, line := range lines2 {
		if strings.Contains(line, "remote_node_last_response_timestamp_seconds") &&
			strings.Contains(line, nodeAddress) &&
			!strings.HasPrefix(line, "#") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				_, err := fmt.Sscanf(parts[len(parts)-1], "%f", &timestamp2)
				require.NoError(t, err)
				break
			}
		}
	}

	// Verify timestamp hasn't changed (error responses don't update it)
	assert.Equal(t, timestamp, timestamp2, "Timestamp should not update on error responses")

	// Verify that time() - timestamp would show staleness
	staleness := float64(time.Now().Unix()) - timestamp2
	assert.GreaterOrEqual(t, staleness, 2.0, "Staleness should be at least 2 seconds")
}

// TestMetricsMultipleNodes verifies that metrics correctly track multiple nodes
func TestMetricsMultipleNodes(t *testing.T) {
	// Reset metrics
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

	// Create test HTTP server
	server := httptest.NewServer(promhttp.Handler())
	defer server.Close()

	// Fetch metrics
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	metricsOutput := string(body)

	// Verify both nodes are tracked separately
	assert.Contains(t, metricsOutput, `node_address="https://soroban-testnet.stellar.org"`)
	assert.Contains(t, metricsOutput, `node_address="https://soroban-mainnet.stellar.org"`)
	assert.Contains(t, metricsOutput, `network="testnet"`)
	assert.Contains(t, metricsOutput, `network="mainnet"`)

	// Verify separate counters
	assert.Contains(t, metricsOutput, `remote_node_response_total{network="testnet",node_address="https://soroban-testnet.stellar.org",status="success"} 1`)
	assert.Contains(t, metricsOutput, `remote_node_response_total{network="testnet",node_address="https://soroban-testnet.stellar.org",status="error"} 1`)
	assert.Contains(t, metricsOutput, `remote_node_response_total{network="mainnet",node_address="https://soroban-mainnet.stellar.org",status="success"} 1`)
}
