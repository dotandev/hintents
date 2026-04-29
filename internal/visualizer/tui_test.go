// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"testing"

	"github.com/dotandev/hintents/internal/decoder"
	"github.com/stretchr/testify/assert"
)

func TestTUIModel_Flatten(t *testing.T) {
	root := &decoder.CallNode{
		ContractID: "ROOT",
		Function:   "main",
		SubCalls: []*decoder.CallNode{
			{
				ContractID: "C1",
				Function:   "foo",
			},
			{
				ContractID: "C2",
				Function:   "bar",
				SubCalls: []*decoder.CallNode{
					{
						ContractID: "C3",
						Function:   "baz",
					},
				},
			},
		},
	}

	m := NewTUI(root)
	assert.Equal(t, 4, len(m.items))
	assert.Equal(t, "main", m.items[0].node.Function)
	assert.Equal(t, "foo", m.items[1].node.Function)
	assert.Equal(t, "bar", m.items[2].node.Function)
	assert.Equal(t, "baz", m.items[3].node.Function)
}

func TestTUIModel_Visibility(t *testing.T) {
	root := &decoder.CallNode{
		ContractID: "ROOT",
		Function:   "main",
		SubCalls: []*decoder.CallNode{
			{
				ContractID: "C1",
				Function:   "foo",
			},
		},
	}

	m := NewTUI(root)
	// Initially root is expanded
	assert.True(t, m.items[0].expanded)
	assert.True(t, m.items[1].visible)

	// Collapse root
	m.items[0].expanded = false
	m.updateVisibility()
	assert.False(t, m.items[1].visible)
}

func TestTUIModel_Filter(t *testing.T) {
	root := &decoder.CallNode{
		ContractID: "ROOT",
		Function:   "main",
		SubCalls: []*decoder.CallNode{
			{
				ContractID: "C1",
				Function:   "foo",
			},
		},
	}

	m := NewTUI(root)
	m.filterInput.SetValue("foo")
	m.updateVisibility()

	assert.False(t, m.items[1].filtered)
	assert.True(t, m.items[0].filtered) // "main" doesn't contain "foo"
}
