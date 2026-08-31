// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import "fmt"

// BuildTraceTree converts a flat execution trace into a collapsible tree where
// cross-contract transitions become nested children.
func BuildTraceTree(trace *ExecutionTrace) *TraceNode {
	if trace == nil {
		return NewTraceNode("root", "trace")
	}
	return trace.BuildTraceTree()
}

// BuildTraceTree converts the execution trace into a collapsible tree view.
func (t *ExecutionTrace) BuildTraceTree() *TraceNode {
	root := NewTraceNode("root", "trace")
	root.Expanded = true

	if t == nil || len(t.States) == 0 {
		return root
	}

	var previous *TraceNode
	for i, state := range t.States {
		node := NewTraceNode(fmt.Sprintf("step-%d", i), "state")
		node.ContractID = state.ContractID
		node.Function = state.Function
		node.Error = state.Error
		node.EventData = state.Operation
		switch {
		case state.EventType != "":
			node.Type = state.EventType
		case state.ContractID != "":
			node.Type = "contract_call"
		}

		if previous == nil {
			root.AddChild(node)
		} else if previous.ContractID != "" && state.ContractID != "" && state.ContractID != previous.ContractID {
			previous.AddChild(node)
		} else if previous.Parent != nil {
			previous.Parent.AddChild(node)
		} else {
			root.AddChild(node)
		}

		previous = node
	}

	return root
}
