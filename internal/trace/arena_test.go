// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// arenaNodes returns every slot the arena currently owns, allocated or spare.
func arenaNodes(a *NodeArena) []*TraceNode {
	var out []*TraceNode
	for i := range a.slabs {
		for j := range a.slabs[i] {
			out = append(out, &a.slabs[i][j])
		}
	}
	return out
}

// nodeIsArenaOwned reports whether n points into one of the arena's slabs.
func nodeIsArenaOwned(a *NodeArena, n *TraceNode) bool {
	if n == nil {
		return false
	}
	for _, candidate := range arenaNodes(a) {
		if candidate == n {
			return true
		}
	}
	return false
}

func TestNewNodeArena_DefaultsSlabSize(t *testing.T) {
	assert.Equal(t, DefaultArenaSlabSize, NewNodeArena(0).SlabSize())
	assert.Equal(t, DefaultArenaSlabSize, NewNodeArena(-4).SlabSize())
	assert.Equal(t, 8, NewNodeArena(8).SlabSize())
}

func TestNodeArena_NewNodeSetsFieldsAndLeavesChildrenNil(t *testing.T) {
	arena := NewNodeArena(4)

	node := arena.NewNode("call-1", "contract_call")

	assert.Equal(t, "call-1", node.ID)
	assert.Equal(t, "contract_call", node.Type)
	assert.True(t, node.Expanded)
	assert.Nil(t, node.Children, "arena nodes grow Children on demand to avoid per-node arrays")
	assert.Nil(t, node.Parent)
	assert.Equal(t, 0, node.Depth)
	assert.Nil(t, node.SourceRef)
	assert.Nil(t, node.CPUDelta)
	assert.Nil(t, node.MemoryDelta)
	assert.Empty(t, node.EventData)
	assert.Empty(t, node.ContractID)
	assert.Empty(t, node.Function)
	assert.Empty(t, node.Error)
}

func TestNodeArena_HandsOutDistinctNodes(t *testing.T) {
	arena := NewNodeArena(16)

	seen := make(map[*TraceNode]bool)
	for i := 0; i < 200; i++ {
		node := arena.NewNode(fmt.Sprintf("n-%d", i), "event")
		require.False(t, seen[node], "arena handed out the same node twice")
		seen[node] = true
	}

	assert.Len(t, seen, 200)
	assert.Equal(t, 200, arena.Allocated())
}

func TestNodeArena_AddChildStillWorksOnNilChildren(t *testing.T) {
	arena := NewNodeArena(4)
	parent := arena.NewNode("parent", "contract_call")
	child := arena.NewNode("child", "host_fn")

	parent.AddChild(child)

	assert.Len(t, parent.Children, 1)
	assert.Same(t, parent, child.Parent)
	assert.Equal(t, 1, child.Depth)
	assert.False(t, parent.IsLeaf())
}

func TestNodeArena_GrowsAcrossSlabBoundaries(t *testing.T) {
	arena := NewNodeArena(4)

	// 10 nodes with a slab size of 4 needs 3 slabs.
	for i := 0; i < 10; i++ {
		arena.NewNode(fmt.Sprintf("n-%d", i), "event")
	}

	stats := arena.Stats()
	assert.Equal(t, 10, stats.Allocated)
	assert.Equal(t, 3, stats.Slabs)
	assert.Equal(t, 12, stats.Capacity)
	assert.Equal(t, 4, stats.SlabSize)
}

func TestNodeArena_ResetReusesSlabs(t *testing.T) {
	arena := NewNodeArena(8)

	first := make([]*TraceNode, 0, 8)
	for i := 0; i < 8; i++ {
		first = append(first, arena.NewNode(fmt.Sprintf("n-%d", i), "event"))
	}
	capacityBefore := arena.Stats().Capacity

	arena.Reset()

	assert.Equal(t, 0, arena.Allocated())
	assert.Equal(t, 8, arena.Recycled())
	assert.Equal(t, capacityBefore, arena.Stats().Capacity, "slabs are kept for reuse")

	// The recycled slots are handed back out in the same order.
	for i := 0; i < 8; i++ {
		reused := arena.NewNode(fmt.Sprintf("m-%d", i), "event")
		assert.Same(t, first[i], reused, "expected slot reuse at index %d", i)
	}
	assert.Equal(t, 8, arena.Allocated())
	assert.Equal(t, 8, arena.Recycled())
}

func TestNodeArena_ResetClearsStaleNodeState(t *testing.T) {
	arena := NewNodeArena(2)

	parent := arena.NewNode("parent", "contract_call")
	child := arena.NewNode("child", "event")
	child.Parent = parent
	child.ContractID = "CDLZ"
	child.Function = "transfer"
	child.Error = "boom"
	child.EventData = "stale payload"
	child.Depth = 7
	child.Expanded = false
	sourceRef := &SourceRef{}
	child.SourceRef = sourceRef
	cpu := uint64(10)
	child.CPUDelta = &cpu
	child.Children = append(child.Children, parent)

	arena.Reset()

	fresh := arena.NewNode("fresh", "event")

	assert.Equal(t, "fresh", fresh.ID)
	assert.Equal(t, "event", fresh.Type)
	assert.True(t, fresh.Expanded, "Expanded must be reset to the default")
	assert.Nil(t, fresh.Parent)
	assert.Nil(t, fresh.Children)
	assert.Nil(t, fresh.SourceRef)
	assert.Nil(t, fresh.CPUDelta)
	assert.Nil(t, fresh.MemoryDelta)
	assert.Empty(t, fresh.ContractID)
	assert.Empty(t, fresh.Function)
	assert.Empty(t, fresh.Error)
	assert.Empty(t, fresh.EventData)
	assert.Equal(t, 0, fresh.Depth)
}

func TestNodeArena_ReleaseDropsSlabsButStaysUsable(t *testing.T) {
	arena := NewNodeArena(4)
	for i := 0; i < 12; i++ {
		arena.NewNode("n", "event")
	}
	assert.Equal(t, 12, arena.Stats().Capacity)

	arena.Release()

	assert.Equal(t, 0, arena.Stats().Capacity)
	assert.Equal(t, 0, arena.Stats().Slabs)
	assert.Equal(t, 0, arena.Allocated())

	node := arena.NewNode("after-release", "event")
	assert.Equal(t, "after-release", node.ID)
	assert.Equal(t, 1, arena.Allocated())
	assert.Equal(t, 4, arena.Stats().Capacity)
}

func TestParseSimulationResponseWithArena_NilResponse(t *testing.T) {
	arena := NewNodeArena(4)

	root, err := ParseSimulationResponseWithArena(nil, arena)

	require.Error(t, err)
	assert.Nil(t, root)
	assert.Equal(t, 0, arena.Allocated(), "no nodes allocated for an invalid response")
}

func TestParseSimulationResponseWithArena_AllNodesComeFromArena(t *testing.T) {
	arena := NewNodeArena(4)
	contractID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	instruction := "i64.const 1"

	resp := &SimulationResponse{
		Status: "success",
		Events: []string{fmt.Sprintf("contract: %s fn: transfer", contractID)},
		Logs:   []string{"log line"},
		DiagnosticEvents: []DiagnosticEvent{
			{EventType: "budget", ContractID: &contractID, Data: "cpu", WasmInstruction: &instruction},
		},
	}

	root, err := ParseSimulationResponseWithArena(resp, arena)
	require.NoError(t, err)

	flat := root.FlattenAll()
	assert.Equal(t, arena.Allocated(), len(flat), "every node in the tree is arena-allocated")

	for _, node := range flat {
		assert.True(t, nodeIsArenaOwned(arena, node), "node %q escaped the arena", node.ID)
	}
}

func TestParseSimulationResponseWithArena_MatchesDefaultParser(t *testing.T) {
	contractID := "CABC"
	instruction := "call 0"
	resp := &SimulationResponse{
		Status: "success",
		Error:  "reverted",
		Events: []string{"contract: CABC fn: mint", "error: not enough balance"},
		Logs:   []string{"first", "second"},
		DiagnosticEvents: []DiagnosticEvent{
			{EventType: "budget", ContractID: &contractID, Data: "d1", WasmInstruction: &instruction},
		},
	}

	defaultRoot, err := ParseSimulationResponse(resp)
	require.NoError(t, err)

	arenaRoot, err := ParseSimulationResponseWithArena(resp, NewNodeArena(3))
	require.NoError(t, err)

	defaultFlat := defaultRoot.FlattenAll()
	arenaFlat := arenaRoot.FlattenAll()

	require.Len(t, arenaFlat, len(defaultFlat))
	for i := range defaultFlat {
		assert.Equal(t, defaultFlat[i].ID, arenaFlat[i].ID)
		assert.Equal(t, defaultFlat[i].Type, arenaFlat[i].Type)
		assert.Equal(t, defaultFlat[i].EventData, arenaFlat[i].EventData)
		assert.Equal(t, defaultFlat[i].Function, arenaFlat[i].Function)
		assert.Equal(t, defaultFlat[i].ContractID, arenaFlat[i].ContractID)
		assert.Equal(t, defaultFlat[i].Error, arenaFlat[i].Error)
		assert.Equal(t, defaultFlat[i].Depth, arenaFlat[i].Depth)
	}
}

func TestParseSimulationResponseWithArena_NilArenaAllocatesItsOwn(t *testing.T) {
	resp := &SimulationResponse{Status: "success", Events: []string{"one"}}

	root, err := ParseSimulationResponseWithArena(resp, nil)

	require.NoError(t, err)
	assert.Len(t, root.FlattenAll(), 2)
}

func TestParseSimulationResponseWithArena_CollapsedNodesAreArenaOwned(t *testing.T) {
	arena := NewNodeArena(8)

	events := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		events = append(events, fmt.Sprintf("event %d", i))
	}

	root, err := ParseSimulationResponseWithArena(&SimulationResponse{Status: "ok", Events: events}, arena)
	require.NoError(t, err)

	// 1 root + 30 event nodes + 1 synthetic collapsed node
	assert.Equal(t, 32, arena.Allocated())

	collapsed := root.Children[len(root.Children)-1]
	require.Equal(t, "collapsed", collapsed.Type)
	assert.True(t, nodeIsArenaOwned(arena, collapsed), "collapsed node must come from the arena")
	assert.False(t, collapsed.Expanded)

	for _, node := range root.FlattenAll() {
		assert.True(t, nodeIsArenaOwned(arena, node), "node %q escaped the arena", node.ID)
	}
}

func TestParseSimulationResponseWithArena_ResetThenReparse(t *testing.T) {
	arena := NewNodeArena(8)
	resp := &SimulationResponse{
		Status: "success",
		Events: []string{"a", "b", "c"},
	}

	first, err := ParseSimulationResponseWithArena(resp, arena)
	require.NoError(t, err)
	firstAllocated := arena.Allocated()

	arena.Reset()

	second, err := ParseSimulationResponseWithArena(resp, arena)
	require.NoError(t, err)

	assert.Equal(t, firstAllocated, arena.Allocated(), "the second parse reuses the same region")
	assert.Equal(t, firstAllocated, arena.Recycled())
	assert.Same(t, first, second, "the arena hands the same slab slots back out")

	// The reused tree is fully rebuilt rather than inheriting stale data.
	assert.Equal(t, "root", second.ID)
	assert.Equal(t, "Status: success", second.EventData)
	assert.Len(t, second.Children, 3)
	for i, child := range second.Children {
		assert.Equal(t, fmt.Sprintf("event-%d", i), child.ID)
		assert.Same(t, second, child.Parent)
	}
}

func TestCreateMockTraceWithArena_AllNodesArenaOwned(t *testing.T) {
	arena := NewNodeArena(4)

	root := CreateMockTraceWithArena(arena)

	flat := root.FlattenAll()
	assert.Equal(t, arena.Allocated(), len(flat))
	for _, node := range flat {
		assert.True(t, nodeIsArenaOwned(arena, node), "node %q escaped the arena", node.ID)
	}
}

func TestCreateMockTraceWithArena_MatchesDefaultMockTrace(t *testing.T) {
	defaultFlat := CreateMockTrace().FlattenAll()
	arenaFlat := CreateMockTraceWithArena(nil).FlattenAll()

	require.Len(t, arenaFlat, len(defaultFlat))
	for i := range defaultFlat {
		assert.Equal(t, defaultFlat[i].ID, arenaFlat[i].ID)
		assert.Equal(t, defaultFlat[i].Type, arenaFlat[i].Type)
		assert.Equal(t, defaultFlat[i].Function, arenaFlat[i].Function)
	}
}

func TestApplyHeuristicsWith_NilFactoryFallsBack(t *testing.T) {
	root := NewTraceNode("root", "simulation")
	for i := 0; i < 15; i++ {
		root.AddChild(NewTraceNode(fmt.Sprintf("event-%d", i), "event"))
	}

	root.ApplyHeuristicsWith(nil)

	require.Len(t, root.Children, 6)
	assert.Equal(t, "collapsed", root.Children[5].Type)
}
