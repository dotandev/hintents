// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"
)

// GenerateFlamegraph dynamically calculates canvas height based on max stack depth.
func GenerateFlamegraph(maxStackDepth int) string {
	// 20 pixels per frame plus 50 for labels/padding
	canvasHeight := maxStackDepth*20 + 50
	return fmt.Sprintf("<svg height=\"%d\"></svg>", canvasHeight)
}

// GenerateMemoryFlamegraph dynamically calculates canvas height based on max stack depth
// and produces an SVG container tailored for memory allocation profiling.
func GenerateMemoryFlamegraph(maxStackDepth int) string {
	if maxStackDepth < 0 {
		maxStackDepth = 0
	}
	canvasHeight := maxStackDepth*20 + 50
	return fmt.Sprintf("<svg height=\"%d\" data-flamegraph-type=\"memory\"></svg>", canvasHeight)
}

// GenerateMemoryAllocationFlamegraph is an alias for GenerateMemoryFlamegraph.
func GenerateMemoryAllocationFlamegraph(maxStackDepth int) string {
	return GenerateMemoryFlamegraph(maxStackDepth)
}

// MemoryFlamegraphConfig contains configuration parameters for memory flamegraph generation.
type MemoryFlamegraphConfig struct {
	Title         string // Title for the flamegraph visualization
	Metric        string // Metric label (e.g. "bytes", "allocations")
	MaxStackDepth int    // Maximum call stack depth
	MinBytes      uint64 // Minimum byte threshold to include in folded stacks
}

// GenerateMemoryFlamegraphWithConfig generates an SVG memory flamegraph container with custom configuration.
func GenerateMemoryFlamegraphWithConfig(config MemoryFlamegraphConfig) string {
	depth := config.MaxStackDepth
	if depth < 0 {
		depth = 0
	}
	canvasHeight := depth*20 + 50

	title := config.Title
	if title == "" {
		title = "Memory Allocation Flamegraph"
	}

	metric := config.Metric
	if metric == "" {
		metric = "bytes"
	}

	return fmt.Sprintf(
		"<svg height=\"%d\" data-flamegraph-type=\"memory\" data-title=\"%s\" data-metric=\"%s\"></svg>",
		canvasHeight,
		title,
		metric,
	)
}

// BuildFoldedMemoryTrace extracts folded stack lines from a TraceNode execution tree
// for all nodes where MemoryDelta is tracked and greater than zero.
func BuildFoldedMemoryTrace(root *TraceNode) string {
	if root == nil {
		return ""
	}

	var sb strings.Builder
	buildFoldedMemoryRecursive(root, "", 0, &sb)
	return sb.String()
}

func buildFoldedMemoryRecursive(node *TraceNode, parentPath string, minBytes uint64, sb *strings.Builder) {
	if node == nil {
		return
	}

	frameName := formatNodeFrameName(node)
	var currentPath string
	if parentPath == "" {
		currentPath = frameName
	} else {
		currentPath = parentPath + ";" + frameName
	}

	if node.MemoryDelta != nil && *node.MemoryDelta >= minBytes && *node.MemoryDelta > 0 {
		fmt.Fprintf(sb, "%s %d\n", currentPath, *node.MemoryDelta)
	}

	for _, child := range node.Children {
		buildFoldedMemoryRecursive(child, currentPath, minBytes, sb)
	}
}

// ExtractMemoryAllocations aggregates total memory allocations per call stack path across the execution trace.
func ExtractMemoryAllocations(root *TraceNode) map[string]uint64 {
	allocations := make(map[string]uint64)
	if root == nil {
		return allocations
	}

	extractMemoryRecursive(root, "", allocations)
	return allocations
}

func extractMemoryRecursive(node *TraceNode, parentPath string, allocations map[string]uint64) {
	if node == nil {
		return
	}

	frameName := formatNodeFrameName(node)
	var currentPath string
	if parentPath == "" {
		currentPath = frameName
	} else {
		currentPath = parentPath + ";" + frameName
	}

	if node.MemoryDelta != nil && *node.MemoryDelta > 0 {
		allocations[currentPath] += *node.MemoryDelta
	}

	for _, child := range node.Children {
		extractMemoryRecursive(child, currentPath, allocations)
	}
}

// GenerateMemoryFlamegraphFromTree computes max depth from a TraceNode tree and returns the memory flamegraph SVG container.
func GenerateMemoryFlamegraphFromTree(root *TraceNode) string {
	if root == nil {
		return GenerateMemoryFlamegraph(0)
	}
	maxDepth := calculateMaxDepth(root)
	return GenerateMemoryFlamegraph(maxDepth)
}

func calculateMaxDepth(node *TraceNode) int {
	if node == nil {
		return 0
	}
	maxDepth := node.Depth
	for _, child := range node.Children {
		childMax := calculateMaxDepth(child)
		if childMax > maxDepth {
			maxDepth = childMax
		}
	}
	return maxDepth
}

func formatNodeFrameName(node *TraceNode) string {
	if node == nil {
		return "unknown"
	}
	if node.ContractID != "" && node.Function != "" {
		return fmt.Sprintf("%s::%s", node.ContractID, node.Function)
	}
	if node.Function != "" {
		return node.Function
	}
	if node.ContractID != "" {
		return node.ContractID
	}
	if node.Type != "" {
		return node.Type
	}
	return "node"
}
