// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"
)

// SimulationResponse represents a simulation response (to avoid import cycle)
type SimulationResponse struct {
	Status           string
	Error            string
	Events           []string
	Logs             []string
	DiagnosticEvents []DiagnosticEvent
}

type DiagnosticEvent struct {
	EventType       string   `json:"event_type"`
	ContractID      *string  `json:"contract_id,omitempty"`
	Topics          []string `json:"topics"`
	Data            string   `json:"data"`
	WasmInstruction *string  `json:"wasm_instruction,omitempty"`
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

	// Parse diagnostic events
	for i, de := range resp.DiagnosticEvents {
		deNode := NewTraceNode(fmt.Sprintf("diag-%d", i), "diagnostic")
		deNode.EventData = de.Data
		if de.ContractID != nil {
			deNode.ContractID = *de.ContractID
		}

		// If it's a budget tick with a WASM instruction, add a sub-node
		if de.WasmInstruction != nil {
			instrNode := NewTraceNode(fmt.Sprintf("diag-%d-instr", i), "wasm_instruction")
			instrNode.EventData = fmt.Sprintf("WASM Instruction: %s", *de.WasmInstruction)
			deNode.AddChild(instrNode)
		}

		root.AddChild(deNode)
	}

	root.ApplyHeuristics()

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

	// Contract call 1 with budget metrics
	call1 := NewTraceNode("call-1", "contract_call")
	call1.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	call1.Function = "transfer"
	cpu1 := uint64(150000)
	mem1 := uint64(2048)
	call1.CPUDelta = &cpu1
	call1.MemoryDelta = &mem1
	root.AddChild(call1)

	// Host function call
	hostFn1 := NewTraceNode("host-1", "host_fn")
	hostFn1.Function = "require_auth"
	cpu2 := uint64(50000)
	mem2 := uint64(512)
	hostFn1.CPUDelta = &cpu2
	hostFn1.MemoryDelta = &mem2
	call1.AddChild(hostFn1)

	// Event
	event1 := NewTraceNode("event-1", "event")
	event1.EventData = "Transfer: 100 XLM"
	call1.AddChild(event1)

	// Contract call 2 with error
	call2 := NewTraceNode("call-2", "contract_call")
	call2.ContractID = "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"
	call2.Function = "swap"
	cpu3 := uint64(250000)
	mem3 := uint64(4096)
	call2.CPUDelta = &cpu3
	call2.MemoryDelta = &mem3
	root.AddChild(call2)

	// Error node
	errorNode := NewTraceNode("error-1", "error")
	errorNode.Error = "Insufficient balance"
	call2.AddChild(errorNode)

	// Contract call 3
	call3 := NewTraceNode("call-3", "contract_call")
	call3.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	call3.Function = "get_balance"
	cpu4 := uint64(80000)
	mem4 := uint64(1024)
	call3.CPUDelta = &cpu4
	call3.MemoryDelta = &mem4
	root.AddChild(call3)

	// Nested calls
	nestedCall := NewTraceNode("call-4", "contract_call")
	nestedCall.ContractID = "CBGTG4XUWRWXDJ5QQVXJVFXPQNQPQNQPQNQPQNQPQNQPQNQPQNQPQNQP"
	nestedCall.Function = "validate"
	cpu5 := uint64(120000)
	mem5 := uint64(1536)
	nestedCall.CPUDelta = &cpu5
	nestedCall.MemoryDelta = &mem5
	call3.AddChild(nestedCall)

	event2 := NewTraceNode("event-2", "event")
	event2.EventData = "Balance: 500 XLM"
	nestedCall.AddChild(event2)

	return root
}
