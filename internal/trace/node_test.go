// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTraceNode(t *testing.T) {
	node := NewTraceNode("test-id", "contract_call")

	assert.Equal(t, "test-id", node.ID)
	assert.Equal(t, "contract_call", node.Type)
	assert.True(t, node.Expanded)
	assert.NotNil(t, node.Children)
	assert.Equal(t, 0, len(node.Children))
}

func TestTraceNode_AddChild(t *testing.T) {
	parent := NewTraceNode("parent", "contract_call")
	child := NewTraceNode("child", "host_fn")

	parent.AddChild(child)

	assert.Equal(t, 1, len(parent.Children))
	assert.Equal(t, child, parent.Children[0])
	assert.Equal(t, parent, child.Parent)
	assert.Equal(t, 1, child.Depth)
}

func TestTraceNode_IsLeaf(t *testing.T) {
	parent := NewTraceNode("parent", "contract_call")
	child := NewTraceNode("child", "host_fn")

	assert.True(t, parent.IsLeaf())

	parent.AddChild(child)

	assert.False(t, parent.IsLeaf())
	assert.True(t, child.IsLeaf())
}

func TestTraceNode_Flatten(t *testing.T) {
	// Create tree:
	//   root
	//   ├── child1
	//   │   └── grandchild1
	//   └── child2

	root := NewTraceNode("root", "contract_call")
	child1 := NewTraceNode("child1", "host_fn")
	child2 := NewTraceNode("child2", "host_fn")
	grandchild1 := NewTraceNode("grandchild1", "event")

	root.AddChild(child1)
	root.AddChild(child2)
	child1.AddChild(grandchild1)

	flattened := root.Flatten()

	assert.Equal(t, 4, len(flattened))
	assert.Equal(t, "root", flattened[0].ID)
	assert.Equal(t, "child1", flattened[1].ID)
	assert.Equal(t, "grandchild1", flattened[2].ID)
	assert.Equal(t, "child2", flattened[3].ID)
}

func TestTraceNode_Flatten_Collapsed(t *testing.T) {
	root := NewTraceNode("root", "contract_call")
	child1 := NewTraceNode("child1", "host_fn")
	grandchild1 := NewTraceNode("grandchild1", "event")

	root.AddChild(child1)
	child1.AddChild(grandchild1)

	// Collapse child1
	child1.Expanded = false

	flattened := root.Flatten()

	// Should only include root and child1, not grandchild1
	assert.Equal(t, 2, len(flattened))
	assert.Equal(t, "root", flattened[0].ID)
	assert.Equal(t, "child1", flattened[1].ID)
}

func TestTraceNode_FlattenAll(t *testing.T) {
	root := NewTraceNode("root", "contract_call")
	child1 := NewTraceNode("child1", "host_fn")
	grandchild1 := NewTraceNode("grandchild1", "event")

	root.AddChild(child1)
	child1.AddChild(grandchild1)

	// Collapse child1
	child1.Expanded = false

	flattened := root.FlattenAll()

	// Should include all nodes regardless of expansion
	assert.Equal(t, 3, len(flattened))
	assert.Equal(t, "root", flattened[0].ID)
	assert.Equal(t, "child1", flattened[1].ID)
	assert.Equal(t, "grandchild1", flattened[2].ID)
}

func TestTraceNode_ToggleExpanded(t *testing.T) {
	node := NewTraceNode("test", "contract_call")

	assert.True(t, node.Expanded)

	node.ToggleExpanded()
	assert.False(t, node.Expanded)

	node.ToggleExpanded()
	assert.True(t, node.Expanded)
}

func TestTraceNode_ExpandAll(t *testing.T) {
	root := NewTraceNode("root", "contract_call")
	child1 := NewTraceNode("child1", "host_fn")
	grandchild1 := NewTraceNode("grandchild1", "event")

	root.AddChild(child1)
	child1.AddChild(grandchild1)

	// Collapse all
	root.Expanded = false
	child1.Expanded = false
	grandchild1.Expanded = false

	root.ExpandAll()

	assert.True(t, root.Expanded)
	assert.True(t, child1.Expanded)
	assert.True(t, grandchild1.Expanded)
}

func TestTraceNode_CollapseAll(t *testing.T) {
	root := NewTraceNode("root", "contract_call")
	child1 := NewTraceNode("child1", "host_fn")
	grandchild1 := NewTraceNode("grandchild1", "event")

	root.AddChild(child1)
	child1.AddChild(grandchild1)

	root.CollapseAll()

	assert.False(t, root.Expanded)
	assert.False(t, child1.Expanded)
	assert.False(t, grandchild1.Expanded)
}

func TestTraceNode_Depth(t *testing.T) {
	root := NewTraceNode("root", "contract_call")
	child1 := NewTraceNode("child1", "host_fn")
	grandchild1 := NewTraceNode("grandchild1", "event")

	root.AddChild(child1)
	child1.AddChild(grandchild1)

	assert.Equal(t, 0, root.Depth)
	assert.Equal(t, 1, child1.Depth)
	assert.Equal(t, 2, grandchild1.Depth)
}
