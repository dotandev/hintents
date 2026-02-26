// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"
)

// TreeUINode represents a renderable node in the tree UI with mouse tracking
type TreeUINode struct {
	Node         *TraceNode
	DisplayText  string
	IndentLevel  int
	ScreenRow    int // Row number on screen for mouse tracking
	ExpandBoxCol int // Column where the expand/collapse box is
	IsVisible    bool
}

// TreeRenderer handles rendering of the trace tree with mouse support
type TreeRenderer struct {
	nodes        []*TreeUINode
	selectedRow  int
	screenWidth  int
	screenHeight int
	scrollOffset int
}

// NewTreeRenderer creates a new tree renderer
func NewTreeRenderer(screenWidth, screenHeight int) *TreeRenderer {
	return &TreeRenderer{
		nodes:        make([]*TreeUINode, 0),
		selectedRow:  0,
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
		scrollOffset: 0,
	}
}

// RenderTree builds the tree UI nodes from a trace node
func (tr *TreeRenderer) RenderTree(root *TraceNode) {
	tr.nodes = make([]*TreeUINode, 0)
	tr.renderNode(root, 0)
}

// renderNode recursively renders a node and its children
func (tr *TreeRenderer) renderNode(node *TraceNode, indentLevel int) {
	if node == nil {
		return
	}

	// Create UI node
	uiNode := &TreeUINode{
		Node:        node,
		IndentLevel: indentLevel,
		IsVisible:   true,
	}

	// Build display text
	expandSymbol := "  "
	if !node.IsLeaf() {
		if node.Expanded {
			expandSymbol = "▼ "
		} else {
			expandSymbol = "▶ "
		}
	}

	// Format node display
	nodeDesc := fmt.Sprintf("%s (%s)", node.Function, node.Type)
	if node.Function == "" {
		nodeDesc = fmt.Sprintf("[%s]", node.Type)
	}
	if node.Error != "" {
		nodeDesc = fmt.Sprintf("%s [ERROR: %s]", nodeDesc, node.Error)
	}

	// Build indent
	indent := strings.Repeat("  ", indentLevel)
	uiNode.DisplayText = fmt.Sprintf("%s%s%s", indent, expandSymbol, nodeDesc)
	uiNode.ExpandBoxCol = len(indent)

	tr.nodes = append(tr.nodes, uiNode)

	// Render children if expanded
	if node.Expanded {
		for _, child := range node.Children {
			tr.renderNode(child, indentLevel+1)
		}
	}
}

// HandleMouseClick processes mouse clicks at the given column and row
// Returns true if a node was toggled
func (tr *TreeRenderer) HandleMouseClick(col, row int) bool {
	// Adjust for scroll offset
	displayRow := row + tr.scrollOffset

	if displayRow < 0 || displayRow >= len(tr.nodes) {
		return false
	}

	node := tr.nodes[displayRow]

	// Check if click is on the expand/collapse symbol
	expandBoxRight := node.ExpandBoxCol + 2
	if col >= node.ExpandBoxCol && col < expandBoxRight && !node.Node.IsLeaf() {
		node.Node.ToggleExpanded()
		return true
	}

	// Select the row
	tr.selectedRow = displayRow
	return false
}

// GetSelectedNode returns the currently selected node
func (tr *TreeRenderer) GetSelectedNode() *TraceNode {
	if tr.selectedRow < 0 || tr.selectedRow >= len(tr.nodes) {
		return nil
	}
	return tr.nodes[tr.selectedRow].Node
}

// SelectUp moves selection up
func (tr *TreeRenderer) SelectUp() {
	if tr.selectedRow > 0 {
		tr.selectedRow--
		tr.ensureSelectedVisible()
	}
}

// SelectDown moves selection down
func (tr *TreeRenderer) SelectDown() {
	if tr.selectedRow < len(tr.nodes)-1 {
		tr.selectedRow++
		tr.ensureSelectedVisible()
	}
}

// ensureSelectedVisible scrolls to keep the selected row visible
func (tr *TreeRenderer) ensureSelectedVisible() {
	visibleRows := tr.screenHeight - 3 // Account for header and footer
	if tr.selectedRow < tr.scrollOffset {
		tr.scrollOffset = tr.selectedRow
	} else if tr.selectedRow >= tr.scrollOffset+visibleRows {
		tr.scrollOffset = tr.selectedRow - visibleRows + 1
	}
}

// Render returns the rendered tree as a string
func (tr *TreeRenderer) Render() string {
	visibleRows := tr.screenHeight - 3
	output := strings.Builder{}

	// Render visible nodes
	startRow := tr.scrollOffset
	endRow := startRow + visibleRows
	if endRow > len(tr.nodes) {
		endRow = len(tr.nodes)
	}

	for i := startRow; i < endRow; i++ {
		node := tr.nodes[i]
		line := node.DisplayText

		// Add selection marker
		if i == tr.selectedRow {
			line = "▸ " + line
		} else {
			line = "  " + line
		}

		// Truncate to screen width
		if len(line) > tr.screenWidth {
			line = line[:tr.screenWidth]
		} else {
			line = line + strings.Repeat(" ", tr.screenWidth-len(line))
		}

		output.WriteString(line)
		output.WriteString("\n")
	}

	// Render scrollbar indicator if needed
	if len(tr.nodes) > visibleRows {
		_ = int(float64(tr.scrollOffset) / float64(len(tr.nodes)-visibleRows) * float64(visibleRows))
		scrollLine := fmt.Sprintf("─ Showing %d-%d of %d lines (↑↓ navigate, click [+/-] to expand) ─",
			startRow+1, endRow, len(tr.nodes))
		output.WriteString(scrollLine)
	}

	return output.String()
}

// GetNodeAtPosition returns the node UI at the given display position
func (tr *TreeRenderer) GetNodeAtPosition(row int) *TreeUINode {
	adjustedRow := row + tr.scrollOffset
	if adjustedRow >= 0 && adjustedRow < len(tr.nodes) {
		return tr.nodes[adjustedRow]
	}
	return nil
}

// GetAllNodes returns all rendered tree nodes
func (tr *TreeRenderer) GetAllNodes() []*TreeUINode {
	return tr.nodes
}

// SelectRow directly selects a specific row
func (tr *TreeRenderer) SelectRow(row int) {
	if row >= 0 && row < len(tr.nodes) {
		tr.selectedRow = row
		tr.ensureSelectedVisible()
	}
}
