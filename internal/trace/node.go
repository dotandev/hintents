// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import "fmt"

// TraceNode represents a single node in the execution trace tree
type TraceNode struct {
	ID          string       // Unique identifier for this node
	Type        string       // Type of event: "contract_call", "host_fn", "error", "event"
	ContractID  string       // Contract ID if applicable
	Function    string       // Function name being called
	Error       string       // Error message if this is an error node
	EventData   string       // Event data/payload
	Depth       int          // Depth in the call tree (0 = root)
	Children    []*TraceNode // Child nodes in the execution tree
	Parent      *TraceNode   // Parent node (nil for root)
	Expanded    bool         // Whether this node is expanded in the UI
	SourceRef   *SourceRef   // Optional source mapping from WASM debug info; nil if unknown
	CPUDelta    *uint64      // CPU instructions consumed by this node (nil if not tracked)
	MemoryDelta *uint64      // Memory bytes consumed by this node (nil if not tracked)
}

// NewTraceNode creates a new trace node
func NewTraceNode(id, nodeType string) *TraceNode {
	return &TraceNode{
		ID:       id,
		Type:     nodeType,
		Children: make([]*TraceNode, 0),
		Expanded: true, // Expanded by default
	}
}

// AddChild adds a child node to this node
func (n *TraceNode) AddChild(child *TraceNode) {
	child.Parent = n
	child.Depth = n.Depth + 1
	n.Children = append(n.Children, child)
}

// IsLeaf returns true if this node has no children
func (n *TraceNode) IsLeaf() bool {
	return len(n.Children) == 0
}

// Flatten returns a flat list of all nodes in depth-first order
// Only includes visible nodes (respects Expanded state)
func (n *TraceNode) Flatten() []*TraceNode {
	result := []*TraceNode{n}

	if n.Expanded {
		for _, child := range n.Children {
			result = append(result, child.Flatten()...)
		}
	}

	return result
}

// FlattenAll returns a flat list of all nodes regardless of expansion state
func (n *TraceNode) FlattenAll() []*TraceNode {
	result := []*TraceNode{n}

	for _, child := range n.Children {
		result = append(result, child.FlattenAll()...)
	}

	return result
}

// ToggleExpanded toggles the expanded state of this node
func (n *TraceNode) ToggleExpanded() {
	n.Expanded = !n.Expanded
}

// ApplyHeuristics applies heuristics to detect and collapse repetitive patterns
func (n *TraceNode) ApplyHeuristics() {
	if len(n.Children) <= 10 {
		for _, child := range n.Children {
			child.ApplyHeuristics()
		}
		return
	}

	newChildren := make([]*TraceNode, 0)
	i := 0
	for i < len(n.Children) {
		similarityKey := n.Children[i].similarityKey()

		// Count consecutive similar siblings
		count := 1
		for j := i + 1; j < len(n.Children); j++ {
			if n.Children[j].similarityKey() == similarityKey {
				count++
			} else {
				break
			}
		}

		if count > 10 {
			// Keep first 5
			for k := 0; k < 5; k++ {
				child := n.Children[i+k]
				newChildren = append(newChildren, child)
				child.ApplyHeuristics()
			}

			// Create collapsed node for the rest
			collapsedCount := count - 5
			collapsed := NewTraceNode(fmt.Sprintf("%s-collapsed-%d", n.ID, i), "collapsed")
			collapsed.EventData = fmt.Sprintf("Show %d more elements", collapsedCount)
			collapsed.Expanded = false
			collapsed.Depth = n.Depth + 1
			collapsed.Parent = n

			// Move the rest of similar nodes as children of the collapsed node
			for k := 5; k < count; k++ {
				child := n.Children[i+k]
				collapsed.AddChild(child)
				child.ApplyHeuristics()
			}
			newChildren = append(newChildren, collapsed)
			i += count
		} else {
			child := n.Children[i]
			newChildren = append(newChildren, child)
			child.ApplyHeuristics()
			i++
		}
	}
	n.Children = newChildren
}

func (n *TraceNode) similarityKey() string {
	return fmt.Sprintf("%s|%s|%s", n.Type, n.ContractID, n.Function)
}

// ExpandAll expands this node and all descendants
func (n *TraceNode) ExpandAll() {
	n.Expanded = true
	for _, child := range n.Children {
		child.ExpandAll()
	}
}

// CollapseAll collapses this node and all descendants
func (n *TraceNode) CollapseAll() {
	n.Expanded = false
	for _, child := range n.Children {
		child.CollapseAll()
	}
}

// IsCrossContractCall returns true if this node represents a call to a different
// contract than its parent.
func (n *TraceNode) IsCrossContractCall() bool {
	if n.Parent == nil || n.ContractID == "" || n.Parent.ContractID == "" {
		return false
	}
	return n.ContractID != n.Parent.ContractID
}
