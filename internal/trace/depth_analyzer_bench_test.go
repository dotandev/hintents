// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"
)

func BenchmarkAnalyzeDepth(b *testing.B) {
	root := createDeepTrace(100)
	da := NewDepthAnalyzer(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		da.AnalyzeDepth(root)
	}
}

func BenchmarkOptimizeForDisplay(b *testing.B) {
	root := createDeepTrace(100)
	da := NewDepthAnalyzer(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		da.OptimizeForDisplay(root)
	}
}

func BenchmarkFocusOnErrors(b *testing.B) {
	root := createDeepTrace(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FocusOnErrors(root)
	}
}

func createDeepTrace(depth int) *TraceNode {
	root := NewTraceNode("root", "transaction")
	current := root

	for i := 0; i < depth; i++ {
		child := NewTraceNode("node-"+string(rune(i)), "contract_call")
		child.ContractID = "CONTRACT"
		child.Function = "call"
		cpu := uint64(1000)
		mem := uint64(512)
		child.CPUDelta = &cpu
		child.MemoryDelta = &mem
		current.AddChild(child)
		current = child
	}

	errorNode := NewTraceNode("error", "error")
	errorNode.Error = "Deep error"
	current.AddChild(errorNode)

	return root
}
