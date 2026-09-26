// Copyright 2026 Erst Users
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

// ParseSimulationResponse converts a simulation response into a trace tree.
//
// Nodes are allocated individually with NewTraceNode. When parsing large
// transactions prefer ParseSimulationResponseWithArena so the temporary nodes
// come from a single region that can be reset or released in one go.
func ParseSimulationResponse(resp *SimulationResponse) (*TraceNode, error) {
	return parseSimulationResponseWith(resp, NewTraceNode)
}

// ParseSimulationResponseWithArena converts a simulation response into a trace
// tree whose nodes are bump-allocated from arena.
//
// Every node in the returned tree (including nodes synthesised by the collapse
// heuristics) lives in arena, so a single arena.Reset or arena.Release frees the
// entire parse result. A nil arena allocates a fresh one with
// DefaultArenaSlabSize. The returned tree must not outlive a Reset/Release on
// the supplied arena.
func ParseSimulationResponseWithArena(resp *SimulationResponse, arena *NodeArena) (*TraceNode, error) {
	if resp == nil {
		return nil, fmt.Errorf("simulation response is nil")
	}
	if arena == nil {
		arena = NewNodeArena(DefaultArenaSlabSize)
	}
	return parseSimulationResponseWith(resp, arena.NewNode)
}

// parseSimulationResponseWith builds the trace tree using newNode to allocate
// every node.
func parseSimulationResponseWith(resp *SimulationResponse, newNode traceNodeFactory) (*TraceNode, error) {
	if resp == nil {
		return nil, fmt.Errorf("simulation response is nil")
	}

	root := newNode("root", "simulation")
	root.EventData = fmt.Sprintf("Status: %s", resp.Status)

	// Add error if present
	if resp.Error != "" {
		errorNode := newNode("error", "error")
		errorNode.Error = resp.Error
		root.AddChild(errorNode)
	}

	// Parse events
	for i, event := range resp.Events {
		eventNode := parseEventWith(newNode, fmt.Sprintf("event-%d", i), event)
		root.AddChild(eventNode)
	}

	// Parse logs
	for i, log := range resp.Logs {
		logNode := newNode(fmt.Sprintf("log-%d", i), "log")
		logNode.EventData = log
		root.AddChild(logNode)
	}

	// Parse diagnostic events
	for i, de := range resp.DiagnosticEvents {
		deNode := newNode(fmt.Sprintf("diag-%d", i), "diagnostic")
		deNode.EventData = de.Data
		if de.ContractID != nil {
			deNode.ContractID = *de.ContractID
		}

		// If it's a budget tick with a WASM instruction, add a sub-node
		if de.WasmInstruction != nil {
			instrNode := newNode(fmt.Sprintf("diag-%d-instr", i), "wasm_instruction")
			instrNode.EventData = fmt.Sprintf("WASM Instruction: %s", *de.WasmInstruction)
			deNode.AddChild(instrNode)
		}

		root.AddChild(deNode)
	}

	root.ApplyHeuristicsWith(newNode)

	return root, nil
}

// parseEvent parses a single event string into a trace node
func parseEvent(id, event string) *TraceNode {
	return parseEventWith(NewTraceNode, id, event)
}

// parseEventWith parses a single event string, allocating its node with newNode.
func parseEventWith(newNode traceNodeFactory, id, event string) *TraceNode {
	node := newNode(id, "event")
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

// CreateMockTrace creates a mock trace tree for testing.
//
// Nodes are allocated individually with NewTraceNode; see
// CreateMockTraceWithArena to build the tree inside a caller-owned arena.
func CreateMockTrace() *TraceNode {
	return createMockTraceWith(NewTraceNode)
}

// CreateMockTraceWithArena creates the mock trace tree with every node
// bump-allocated from arena. A nil arena allocates a fresh one with
// DefaultArenaSlabSize.
func CreateMockTraceWithArena(arena *NodeArena) *TraceNode {
	if arena == nil {
		arena = NewNodeArena(DefaultArenaSlabSize)
	}
	return createMockTraceWith(arena.NewNode)
}

// createMockTraceWith builds the mock tree using newNode for every node.
func createMockTraceWith(newNode traceNodeFactory) *TraceNode {
	root := newNode("root", "transaction")
	root.EventData = "Transaction: 5c0a1234567890abcdef"

	// Contract call 1 with budget metrics
	call1 := newNode("call-1", "contract_call")
	call1.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	call1.Function = "transfer"
	cpu1 := uint64(150000)
	mem1 := uint64(2048)
	call1.CPUDelta = &cpu1
	call1.MemoryDelta = &mem1
	root.AddChild(call1)

	// Host function call
	hostFn1 := newNode("host-1", "host_fn")
	hostFn1.Function = "require_auth"
	cpu2 := uint64(50000)
	mem2 := uint64(512)
	hostFn1.CPUDelta = &cpu2
	hostFn1.MemoryDelta = &mem2
	call1.AddChild(hostFn1)

	// Event
	event1 := newNode("event-1", "event")
	event1.EventData = "Transfer: 100 XLM"
	call1.AddChild(event1)

	// Contract call 2 with error
	call2 := newNode("call-2", "contract_call")
	call2.ContractID = "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"
	call2.Function = "swap"
	cpu3 := uint64(250000)
	mem3 := uint64(4096)
	call2.CPUDelta = &cpu3
	call2.MemoryDelta = &mem3
	root.AddChild(call2)

	// Error node
	errorNode := newNode("error-1", "error")
	errorNode.Error = "Insufficient balance"
	call2.AddChild(errorNode)

	// Contract call 3
	call3 := newNode("call-3", "contract_call")
	call3.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	call3.Function = "get_balance"
	cpu4 := uint64(80000)
	mem4 := uint64(1024)
	call3.CPUDelta = &cpu4
	call3.MemoryDelta = &mem4
	root.AddChild(call3)

	// Nested calls
	nestedCall := newNode("call-4", "contract_call")
	nestedCall.ContractID = "CBGTG4XUWRWXDJ5QQVXJVFXPQNQPQNQPQNQPQNQPQNQPQNQPQNQPQNQP"
	nestedCall.Function = "validate"
	cpu5 := uint64(120000)
	mem5 := uint64(1536)
	nestedCall.CPUDelta = &cpu5
	nestedCall.MemoryDelta = &mem5
	call3.AddChild(nestedCall)

	event2 := newNode("event-2", "event")
	event2.EventData = "Balance: 500 XLM"
	nestedCall.AddChild(event2)

	return root
}
