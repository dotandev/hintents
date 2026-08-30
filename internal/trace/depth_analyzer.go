// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"
)

// DepthAnalyzer analyzes and optimizes trace visibility for deeply nested calls
type DepthAnalyzer struct {
	maxDisplayDepth int
	errorPaths      map[string]*PathMatch
}

// PathMatch represents a path from root to a specific node
type PathMatch struct {
	Nodes       []*TraceNode
	TotalDepth  int
	HasError    bool
	ErrorDepth  int
	ContractIDs []string
}

// NewDepthAnalyzer creates a new depth analyzer
func NewDepthAnalyzer(maxDisplayDepth int) *DepthAnalyzer {
	if maxDisplayDepth <= 0 {
		maxDisplayDepth = 10 // Default
	}
	return &DepthAnalyzer{
		maxDisplayDepth: maxDisplayDepth,
		errorPaths:      make(map[string]*PathMatch),
	}
}

// AnalyzeDepth analyzes the trace tree and identifies deeply nested calls
func (da *DepthAnalyzer) AnalyzeDepth(root *TraceNode) *DepthAnalysis {
	analysis := &DepthAnalysis{
		MaxDepth:           0,
		TotalNodes:         0,
		ErrorNodes:         make([]*TraceNode, 0),
		DeepNodes:          make([]*TraceNode, 0),
		CrossContractCalls: make([]*TraceNode, 0),
	}

	da.analyzeNode(root, analysis, []*TraceNode{})
	return analysis
}

// analyzeNode recursively analyzes a node and its children
func (da *DepthAnalyzer) analyzeNode(node *TraceNode, analysis *DepthAnalysis, path []*TraceNode) {
	analysis.TotalNodes++
	currentPath := append(path, node)

	if node.Depth > analysis.MaxDepth {
		analysis.MaxDepth = node.Depth
	}

	// Track error nodes
	if node.Type == "error" || node.Error != "" {
		analysis.ErrorNodes = append(analysis.ErrorNodes, node)
		da.errorPaths[node.ID] = &PathMatch{
			Nodes:      currentPath,
			TotalDepth: node.Depth,
			HasError:   true,
			ErrorDepth: node.Depth,
		}
	}

	// Track deeply nested nodes
	if node.Depth >= da.maxDisplayDepth {
		analysis.DeepNodes = append(analysis.DeepNodes, node)
	}

	// Track cross-contract calls
	if node.IsCrossContractCall() {
		analysis.CrossContractCalls = append(analysis.CrossContractCalls, node)
	}

	// Recurse to children
	for _, child := range node.Children {
		da.analyzeNode(child, analysis, currentPath)
	}
}

// OptimizeForDisplay optimizes the trace tree for better visibility
func (da *DepthAnalyzer) OptimizeForDisplay(root *TraceNode) *TraceNode {
	optimized := da.cloneNode(root)
	da.optimizeNode(optimized, 0)
	return optimized
}

// optimizeNode recursively optimizes a node
func (da *DepthAnalyzer) optimizeNode(node *TraceNode, currentDepth int) {
	// If we're at max depth, collapse non-error branches
	if currentDepth >= da.maxDisplayDepth {
		if !da.hasErrorInSubtree(node) {
			node.Expanded = false
			return
		}
	}

	// Optimize children
	for _, child := range node.Children {
		da.optimizeNode(child, currentDepth+1)
	}
}

// hasErrorInSubtree checks if a node or its descendants have errors
func (da *DepthAnalyzer) hasErrorInSubtree(node *TraceNode) bool {
	if node.Type == "error" || node.Error != "" {
		return true
	}
	for _, child := range node.Children {
		if da.hasErrorInSubtree(child) {
			return true
		}
	}
	return false
}

// cloneNode creates a deep copy of a node
func (da *DepthAnalyzer) cloneNode(node *TraceNode) *TraceNode {
	clone := &TraceNode{
		ID:          node.ID,
		Type:        node.Type,
		ContractID:  node.ContractID,
		Function:    node.Function,
		Error:       node.Error,
		EventData:   node.EventData,
		Depth:       node.Depth,
		Expanded:    node.Expanded,
		CPUDelta:    node.CPUDelta,
		MemoryDelta: node.MemoryDelta,
		Children:    make([]*TraceNode, len(node.Children)),
	}

	for i, child := range node.Children {
		clone.Children[i] = da.cloneNode(child)
		clone.Children[i].Parent = clone
	}

	return clone
}

// GetErrorPath returns the path to an error node
func (da *DepthAnalyzer) GetErrorPath(errorNodeID string) *PathMatch {
	return da.errorPaths[errorNodeID]
}

// FormatErrorPath formats an error path for display
func (da *DepthAnalyzer) FormatErrorPath(path *PathMatch) string {
	if path == nil {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Error at depth %d:\n", path.ErrorDepth)

	for i, node := range path.Nodes {
		indent := strings.Repeat("  ", i)
		fmt.Fprintf(&sb, "%s[%d] %s", indent, i, da.formatNode(node))
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatNode formats a single node for display
func (da *DepthAnalyzer) formatNode(node *TraceNode) string {
	switch node.Type {
	case "contract_call":
		return fmt.Sprintf("Contract: %s -> %s()", truncateID(node.ContractID), node.Function)
	case "host_fn":
		return fmt.Sprintf("Host: %s()", node.Function)
	case "error":
		return fmt.Sprintf("ERROR: %s", node.Error)
	case "event":
		return fmt.Sprintf("Event: %s", truncateData(node.EventData))
	default:
		return fmt.Sprintf("%s: %s", node.Type, truncateData(node.EventData))
	}
}

// truncateID truncates a contract ID for display
func truncateID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:6] + "..." + id[len(id)-6:]
}

// truncateData truncates event data for display
func truncateData(data string) string {
	if len(data) <= 50 {
		return data
	}
	return data[:47] + "..."
}

// DepthAnalysis contains the results of depth analysis
type DepthAnalysis struct {
	MaxDepth           int
	TotalNodes         int
	ErrorNodes         []*TraceNode
	DeepNodes          []*TraceNode
	CrossContractCalls []*TraceNode
}

// Summary returns a summary of the analysis
func (da *DepthAnalysis) Summary() string {
	return fmt.Sprintf(
		"Depth Analysis:\n"+
			"  Max Depth: %d\n"+
			"  Total Nodes: %d\n"+
			"  Error Nodes: %d\n"+
			"  Deep Nodes (>=%d): %d\n"+
			"  Cross-Contract Calls: %d",
		da.MaxDepth,
		da.TotalNodes,
		len(da.ErrorNodes),
		10,
		len(da.DeepNodes),
		len(da.CrossContractCalls),
	)
}

// FocusOnErrors collapses all non-error branches in the trace
func FocusOnErrors(root *TraceNode) {
	focusOnErrorsRecursive(root)
}

func focusOnErrorsRecursive(node *TraceNode) bool {
	hasError := node.Type == "error" || node.Error != ""

	for _, child := range node.Children {
		if focusOnErrorsRecursive(child) {
			hasError = true
		}
	}

	if !hasError {
		node.Expanded = false
	}

	return hasError
}

// ExpandErrorPaths expands all paths leading to errors
func ExpandErrorPaths(root *TraceNode) {
	expandErrorPathsRecursive(root)
}

func expandErrorPathsRecursive(node *TraceNode) bool {
	hasError := node.Type == "error" || node.Error != ""

	for _, child := range node.Children {
		if expandErrorPathsRecursive(child) {
			hasError = true
		}
	}

	if hasError {
		node.Expanded = true
	}

	return hasError
}
