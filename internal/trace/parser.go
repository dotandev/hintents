// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"
)

// SimulationResponse represents a simulation response (to avoid import cycle)
type SimulationResponse struct {
	Status string
	Error  string
	Events []string
	Logs   []string
}

// ParseSimulationResponse converts a simulation response into a trace tree
func ParseSimulationResponse(resp *SimulationResponse) (*TraceNode, error) {
	if resp == nil {
		return nil, fmt.Errorf("simulation response is nil")
	}

	root := NewTraceNode("root", "simulation")
	root.EventData = fmt.Sprintf("Status: %s", resp.Status)

	// Add error if present
	if resp.Error != "" {
		errorNode := NewTraceNode("error", "error")
		errorNode.Error = resp.Error
		root.AddChild(errorNode)
	}

	// Parse events
	for i, event := range resp.Events {
		eventNode := parseEvent(fmt.Sprintf("event-%d", i), event)
		root.AddChild(eventNode)
	}

	// Parse logs
	for i, log := range resp.Logs {
		logNode := NewTraceNode(fmt.Sprintf("log-%d", i), "log")
		logNode.EventData = log
		root.AddChild(logNode)
	}

	return root, nil
}

// parseEvent parses a single event string into a trace node
func parseEvent(id, event string) *TraceNode {
	node := NewTraceNode(id, "event")
	node.EventData = event

	// Try to extract contract ID and function from event
	// This is a simple parser - adjust based on actual event format
	if strings.Contains(event, "contract:") {
		parts := strings.Split(event, "contract:")
		if len(parts) > 1 {
			contractPart := strings.TrimSpace(parts[1])
			contractID := strings.Fields(contractPart)[0]
			node.ContractID = contractID
		}
	}

	if strings.Contains(event, "fn:") {
		parts := strings.Split(event, "fn:")
		if len(parts) > 1 {
			fnPart := strings.TrimSpace(parts[1])
			function := strings.Fields(fnPart)[0]
			node.Function = function
		}
	}

	if strings.Contains(event, "error") || strings.Contains(event, "Error") {
		node.Type = "error"
		// Extract error message
		if strings.Contains(event, ":") {
			parts := strings.SplitN(event, ":", 2)
			if len(parts) > 1 {
				node.Error = strings.TrimSpace(parts[1])
			}
		}
	}

	return node
}

// CreateMockTrace creates a mock trace tree for testing
func CreateMockTrace() *TraceNode {
	root := NewTraceNode("root", "transaction")
	root.EventData = "Transaction: 5c0a1234567890abcdef"

	// Contract call 1
	call1 := NewTraceNode("call-1", "contract_call")
	call1.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	call1.Function = "transfer"
	root.AddChild(call1)

	// Host function call
	hostFn1 := NewTraceNode("host-1", "host_fn")
	hostFn1.Function = "require_auth"
	call1.AddChild(hostFn1)

	// Event
	event1 := NewTraceNode("event-1", "event")
	event1.EventData = "Transfer: 100 XLM"
	call1.AddChild(event1)

	// Contract call 2 with error
	call2 := NewTraceNode("call-2", "contract_call")
	call2.ContractID = "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"
	call2.Function = "swap"
	root.AddChild(call2)

	// Error node
	errorNode := NewTraceNode("error-1", "error")
	errorNode.Error = "Insufficient balance"
	call2.AddChild(errorNode)

	// Contract call 3
	call3 := NewTraceNode("call-3", "contract_call")
	call3.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	call3.Function = "get_balance"
	root.AddChild(call3)

	// Nested calls
	nestedCall := NewTraceNode("call-4", "contract_call")
	nestedCall.ContractID = "CBGTG4XUWRWXDJ5QQVXJVFXPQNQPQNQPQNQPQNQPQNQPQNQPQNQPQNQP"
	nestedCall.Function = "validate"
	call3.AddChild(nestedCall)

	event2 := NewTraceNode("event-2", "event")
	event2.EventData = "Balance: 500 XLM"
	nestedCall.AddChild(event2)

	return root
}
