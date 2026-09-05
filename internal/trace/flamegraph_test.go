// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateFlamegraph(t *testing.T) {
	svg := GenerateFlamegraph(5)
	assert.Equal(t, "<svg height=\"150\"></svg>", svg)

	svgZero := GenerateFlamegraph(0)
	assert.Equal(t, "<svg height=\"50\"></svg>", svgZero)
}

func TestGenerateMemoryFlamegraph(t *testing.T) {
	svg := GenerateMemoryFlamegraph(10)
	assert.Contains(t, svg, "height=\"250\"")
	assert.Contains(t, svg, "data-flamegraph-type=\"memory\"")

	svgNegative := GenerateMemoryFlamegraph(-3)
	assert.Contains(t, svgNegative, "height=\"50\"")
}

func TestGenerateMemoryAllocationFlamegraph(t *testing.T) {
	svg := GenerateMemoryAllocationFlamegraph(8)
	assert.Equal(t, GenerateMemoryFlamegraph(8), svg)
}

func TestGenerateMemoryFlamegraphWithConfig(t *testing.T) {
	config := MemoryFlamegraphConfig{
		Title:         "Custom Allocations",
		Metric:        "allocs",
		MaxStackDepth: 4,
		MinBytes:      1024,
	}
	svg := GenerateMemoryFlamegraphWithConfig(config)
	assert.Contains(t, svg, "height=\"130\"")
	assert.Contains(t, svg, "data-title=\"Custom Allocations\"")
	assert.Contains(t, svg, "data-metric=\"allocs\"")

	// Default fallbacks
	defaultSvg := GenerateMemoryFlamegraphWithConfig(MemoryFlamegraphConfig{
		MaxStackDepth: 2,
	})
	assert.Contains(t, defaultSvg, "height=\"90\"")
	assert.Contains(t, defaultSvg, "data-title=\"Memory Allocation Flamegraph\"")
	assert.Contains(t, defaultSvg, "data-metric=\"bytes\"")
}

func TestBuildFoldedMemoryTrace(t *testing.T) {
	assert.Equal(t, "", BuildFoldedMemoryTrace(nil))

	root := NewTraceNode("root", "contract_call")
	root.ContractID = "CA123"
	root.Function = "init"
	rootMem := uint64(512)
	root.MemoryDelta = &rootMem

	child1 := NewTraceNode("child1", "host_fn")
	child1.Function = "obj_to_u64"
	child1Mem := uint64(1024)
	child1.MemoryDelta = &child1Mem
	root.AddChild(child1)

	child2 := NewTraceNode("child2", "contract_call")
	child2.ContractID = "CB456"
	child2.Function = "mint"
	child2Mem := uint64(2048)
	child2.MemoryDelta = &child2Mem
	root.AddChild(child2)

	folded := BuildFoldedMemoryTrace(root)
	expected := "CA123::init 512\nCA123::init;obj_to_u64 1024\nCA123::init;CB456::mint 2048\n"
	assert.Equal(t, expected, folded)
}

func TestExtractMemoryAllocations(t *testing.T) {
	allocsNil := ExtractMemoryAllocations(nil)
	assert.Empty(t, allocsNil)

	root := NewTraceNode("root", "contract_call")
	root.ContractID = "CA123"
	root.Function = "execute"
	rootMem := uint64(100)
	root.MemoryDelta = &rootMem

	child := NewTraceNode("child", "host_fn")
	child.Function = "vec_new"
	childMem := uint64(400)
	child.MemoryDelta = &childMem
	root.AddChild(child)

	allocs := ExtractMemoryAllocations(root)
	assert.Equal(t, uint64(100), allocs["CA123::execute"])
	assert.Equal(t, uint64(400), allocs["CA123::execute;vec_new"])
}

func TestGenerateMemoryFlamegraphFromTree(t *testing.T) {
	svgNil := GenerateMemoryFlamegraphFromTree(nil)
	assert.Contains(t, svgNil, "height=\"50\"")

	root := NewTraceNode("root", "contract_call")
	root.Depth = 0

	child := NewTraceNode("child", "contract_call")
	root.AddChild(child) // Depth = 1

	grandchild := NewTraceNode("grandchild", "host_fn")
	child.AddChild(grandchild) // Depth = 2

	svg := GenerateMemoryFlamegraphFromTree(root)
	assert.Contains(t, svg, "height=\"90\"") // 2 * 20 + 50 = 90
	assert.Contains(t, svg, "data-flamegraph-type=\"memory\"")
}
