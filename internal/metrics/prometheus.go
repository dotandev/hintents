// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RemoteNodeLastResponseTimestamp tracks the last time a simulated response was successfully
	// received or updated per remote node. This metric enables staleness alerting by exposing
	// the Unix timestamp (in seconds) of the most recent successful simulation response.
	//
	// Labels:
	//   - node_address: The RPC URL or identifier of the remote node
	//   - network: The Stellar network (testnet, mainnet, futurenet)
	//
	// Alert threshold example:
	//   Alert when no response received in 60 seconds:
	//   time() - remote_node_last_response_timestamp_seconds{node_address="<rpc-url>"} > 60
	RemoteNodeLastResponseTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "remote_node_last_response_timestamp_seconds",
			Help: "Unix timestamp of the last successful simulation response from a remote node",
		},
		[]string{"node_address", "network"},
	)

	// RemoteNodeResponseTotal counts the total number of simulation responses received from
	// remote nodes, labeled by status (success/error). This helps track overall node health
	// and error rates over time.
	//
	// Labels:
	//   - node_address: The RPC URL or identifier of the remote node
	//   - network: The Stellar network (testnet, mainnet, futurenet)
	//   - status: Response status (success, error)
	//
	// Alert threshold example:
	//   Alert when error rate exceeds 10%:
	//   rate(remote_node_response_total{status="error"}[5m]) / rate(remote_node_response_total[5m]) > 0.1
	RemoteNodeResponseTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "remote_node_response_total",
			Help: "Total number of simulation responses from remote nodes by status",
		},
		[]string{"node_address", "network", "status"},
	)

	// RemoteNodeResponseDurationSeconds measures the duration of simulation requests to remote
	// nodes in seconds. This helps identify performance degradation or latency issues.
	//
	// Labels:
	//   - node_address: The RPC URL or identifier of the remote node
	//   - network: The Stellar network (testnet, mainnet, futurenet)
	//
	// Alert threshold example:
	//   Alert when p95 latency exceeds 5 seconds:
	//   histogram_quantile(0.95, rate(remote_node_response_duration_seconds_bucket[5m])) > 5
	RemoteNodeResponseDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "remote_node_response_duration_seconds",
			Help:    "Duration of simulation requests to remote nodes in seconds",
			Buckets: prometheus.DefBuckets, // [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
		},
		[]string{"node_address", "network"},
	)

	// SimulationExecutionTotal counts the total number of simulation executions,
	// regardless of whether they involve remote nodes. This provides overall system throughput.
	//
	// Labels:
	//   - status: Execution status (success, error)
	//
	// Alert threshold example:
	//   Alert when simulation error rate exceeds 5%:
	//   rate(simulation_execution_total{status="error"}[5m]) / rate(simulation_execution_total[5m]) > 0.05
	SimulationExecutionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "simulation_execution_total",
			Help: "Total number of simulation executions by status",
		},
		[]string{"status"},
	)
)

// RecordRemoteNodeResponse records metrics for a remote node simulation response.
// This should be called after each simulation that involves fetching data from a remote node.
//
// Parameters:
//   - nodeAddress: The RPC URL or identifier of the remote node
//   - network: The Stellar network (testnet, mainnet, futurenet)
//   - success: Whether the response was successful
//   - duration: How long the request took
func RecordRemoteNodeResponse(nodeAddress, network string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}

	// Update timestamp only on success
	if success {
		RemoteNodeLastResponseTimestamp.WithLabelValues(nodeAddress, network).SetToCurrentTime()
	}

	// Increment response counter
	RemoteNodeResponseTotal.WithLabelValues(nodeAddress, network, status).Inc()

	// Record duration
	RemoteNodeResponseDurationSeconds.WithLabelValues(nodeAddress, network).Observe(duration.Seconds())
}

// RecordSimulationExecution records metrics for a simulation execution.
// This should be called for every simulation run, regardless of remote node involvement.
//
// Parameters:
//   - success: Whether the simulation was successful
func RecordSimulationExecution(success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	SimulationExecutionTotal.WithLabelValues(status).Inc()
}
