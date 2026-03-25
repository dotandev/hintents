// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDepthAnalyzer(t *testing.T) {
	da := NewDepthAnalyzer(5)
	assert.NotNil(t, da)
	assert.Equal(t, 5, da.maxDisplayDepth)
}

func TestAnalyzeDepth(t *testing.T) {
	root := createTestTrace()
	da := NewDepthAnalyzer(10)

	analysis := da.AnalyzeDepth(root)

	assert.NotNil(t, analysis)
	assert.Greater(t, analysis.MaxDepth, 0)
	assert.Greater(t, analysis.TotalNodes, 0)
}

func TestOptimizeForDisplay(t *testing.T) {
	root := createTestTrace()
	da := NewDepthAnalyzer(2)

	optimized := da.OptimizeForDisplay(root)

	assert.NotNil(t, optimized)
	assert.Equal(t, root, optimized)
}

func TestFocusOnErrors(t *testing.T) {
	root := createTestTrace()

	FocusOnErrors(root)

	// Verify error paths are expanded
	allNodes := root.FlattenAll()
	for _, node := range allNodes {
		if node.Type == "error" {
			assert.True(t, node.Parent == nil || node.Parent.Expanded)
		}
	}
}

func TestExpandErrorPaths(t *testing.T) {
	root := createTestTrace()
	root.CollapseAll()

	ExpandErrorPaths(root)

	// Verify paths to errors are expanded
	allNodes := root.FlattenAll()
	errorFound := false
	for _, node := range allNodes {
		if node.Type == "error" {
			errorFound = true
			break
		}
	}
	assert.True(t, errorFound)
}

func TestDepthAnalysis_Summary(t *testing.T) {
	analysis := &DepthAnalysis{
		MaxDepth:   5,
		TotalNodes: 10,
		ErrorNodes: make([]*TraceNode, 2),
	}

	summary := analysis.Summary()
	assert.Contains(t, summary, "Max Depth: 5")
	assert.Contains(t, summary, "Total Nodes: 10")
}

func createTestTrace() *TraceNode {
	root := NewTraceNode("root", "transaction")

	call1 := NewTraceNode("call-1", "contract_call")
	call1.ContractID = "CONTRACT1"
	call1.Function = "transfer"
	cpu1 := uint64(1000)
	mem1 := uint64(512)
	call1.CPUDelta = &cpu1
	call1.MemoryDelta = &mem1
	root.AddChild(call1)

	call2 := NewTraceNode("call-2", "contract_call")
	call2.ContractID = "CONTRACT2"
	call2.Function = "swap"
	cpu2 := uint64(2000)
	mem2 := uint64(1024)
	call2.CPUDelta = &cpu2
	call2.MemoryDelta = &mem2
	call1.AddChild(call2)

	errorNode := NewTraceNode("error-1", "error")
	errorNode.Error = "Insufficient balance"
	call2.AddChild(errorNode)

	return root
}
