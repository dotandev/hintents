// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMouseTracker_Enable(t *testing.T) {
	mt := NewMouseTracker()
	assert.False(t, mt.enabled)

	err := mt.Enable()
	assert.NoError(t, err)
	assert.True(t, mt.enabled)

	// Should be idempotent
	err = mt.Enable()
	assert.NoError(t, err)
	assert.True(t, mt.enabled)
}

func TestMouseTracker_Disable(t *testing.T) {
	mt := NewMouseTracker()
	mt.Enable()
	assert.True(t, mt.enabled)

	err := mt.Disable()
	assert.NoError(t, err)
	assert.False(t, mt.enabled)

	// Should be idempotent
	err = mt.Disable()
	assert.NoError(t, err)
	assert.False(t, mt.enabled)
}

func TestParseMouseEvent_SGRFormat(t *testing.T) {
	// SGR1006 format: <button;col;row;M|m
	sequence := "<0;10;5;M" // Left button click at column 10, row 5

	evt, err := ParseMouseEvent(sequence)
	assert.NoError(t, err)
	assert.NotNil(t, evt)
	assert.Equal(t, LeftButton, evt.Button)
	assert.Equal(t, 9, evt.Col)   // 0-based
	assert.Equal(t, 4, evt.Row)   // 0-based
	assert.True(t, evt.IsClickEvent())
	assert.False(t, evt.IsScrollEvent())
}

func TestParseMouseEvent_ScrollUp(t *testing.T) {
	sequence := "<64;10;5;M" // Scroll up at column 10, row 5

	evt, err := ParseMouseEvent(sequence)
	assert.NoError(t, err)
	assert.NotNil(t, evt)
	assert.Equal(t, ScrollUp, evt.Button)
	assert.False(t, evt.IsClickEvent())
	assert.True(t, evt.IsScrollEvent())
}

func TestParseMouseEvent_ScrollDown(t *testing.T) {
	sequence := "<65;10;5;M" // Scroll down

	evt, err := ParseMouseEvent(sequence)
	assert.NoError(t, err)
	assert.NotNil(t, evt)
	assert.Equal(t, ScrollDown, evt.Button)
	assert.True(t, evt.IsScrollEvent())
}

func TestTreeRenderer_RenderTree(t *testing.T) {
	root := NewTraceNode("root", "transaction")
	child1 := NewTraceNode("child1", "contract_call")
	child1.Function = "transfer"
	child2 := NewTraceNode("child2", "contract_call")
	child2.Function = "validate"

	root.AddChild(child1)
	root.AddChild(child2)

	renderer := NewTreeRenderer(80, 24)
	renderer.RenderTree(root)

	nodes := renderer.GetAllNodes()
	assert.Equal(t, 3, len(nodes))

	// Check root node
	assert.Equal(t, root, nodes[0].Node)
	assert.Equal(t, 0, nodes[0].IndentLevel)

	// Check children
	assert.Equal(t, child1, nodes[1].Node)
	assert.Equal(t, 1, nodes[1].IndentLevel)

	assert.Equal(t, child2, nodes[2].Node)
	assert.Equal(t, 1, nodes[2].IndentLevel)
}

func TestTreeRenderer_HandleMouseClick_ExpandBox(t *testing.T) {
	root := NewTraceNode("root", "transaction")
	child := NewTraceNode("child", "contract_call")
	child.Function = "transfer"
	root.AddChild(child)

	renderer := NewTreeRenderer(80, 24)
	renderer.RenderTree(root)

	// Initially root is expanded
	assert.True(t, root.Expanded)

	// Click on the expand box for root (should be at column 0-1)
	toggled := renderer.HandleMouseClick(0, 0)
	assert.True(t, toggled)
	assert.False(t, root.Expanded) // Should be toggled to collapsed

	// Click again to expand
	toggled = renderer.HandleMouseClick(0, 0)
	assert.True(t, toggled)
	assert.True(t, root.Expanded)
}

func TestTreeRenderer_HandleMouseClick_SelectRow(t *testing.T) {
	root := NewTraceNode("root", "transaction")
	child := NewTraceNode("child", "contract_call")
	root.AddChild(child)

	renderer := NewTreeRenderer(80, 24)
	renderer.RenderTree(root)

	// Click on child row at column where it's not the expand box
	toggled := renderer.HandleMouseClick(20, 1) // Click on text, not expand box
	assert.False(t, toggled)                     // Should not toggle
	assert.Equal(t, child, renderer.GetSelectedNode())
}

func TestTreeRenderer_NavigateSelection(t *testing.T) {
	root := NewTraceNode("root", "transaction")
	child1 := NewTraceNode("child1", "contract_call")
	child1.Function = "transfer"
	child2 := NewTraceNode("child2", "contract_call")
	child2.Function = "validate"

	root.AddChild(child1)
	root.AddChild(child2)

	renderer := NewTreeRenderer(80, 24)
	renderer.RenderTree(root)

	// Initially selecting root
	assert.Equal(t, root, renderer.GetSelectedNode())

	// Navigate down
	renderer.SelectDown()
	assert.Equal(t, child1, renderer.GetSelectedNode())

	renderer.SelectDown()
	assert.Equal(t, child2, renderer.GetSelectedNode())

	// Navigate up
	renderer.SelectUp()
	assert.Equal(t, child1, renderer.GetSelectedNode())
}

func TestTreeRenderer_ExpandCollapse(t *testing.T) {
	root := NewTraceNode("root", "transaction")
	child := NewTraceNode("child", "contract_call")
	grandchild := NewTraceNode("grandchild", "event")

	root.AddChild(child)
	child.AddChild(grandchild)

	renderer := NewTreeRenderer(80, 24)
	renderer.RenderTree(root)

	// Initially all expanded, so 3 nodes visible
	assert.Equal(t, 3, len(renderer.GetAllNodes()))

	// Collapse child
	child.ToggleExpanded()
	renderer.RenderTree(root)

	// Now only root and child visible
	assert.Equal(t, 2, len(renderer.GetAllNodes()))

	// Expand again
	child.ToggleExpanded()
	renderer.RenderTree(root)

	assert.Equal(t, 3, len(renderer.GetAllNodes()))
}
