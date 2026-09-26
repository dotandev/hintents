// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

// traceNodeFactory creates a trace node. Both NewTraceNode and
// (*NodeArena).NewNode satisfy it.
type traceNodeFactory func(id, nodeType string) *TraceNode

// DefaultArenaSlabSize is the number of nodes allocated per slab when no
// explicit slab size is provided. One slab is a single contiguous allocation,
// so a 512-node slab turns 512 individual heap allocations into one.
const DefaultArenaSlabSize = 512

// NodeArena is a bump (region) allocator for temporary TraceNode values.
//
// Parsing a large transaction produces thousands of small, short-lived trace
// nodes. Handing each one to the Go allocator individually fragments the heap
// and pays the per-allocation overhead thousands of times. A NodeArena instead
// hands them out from pre-sized slabs and frees the whole region at once with
// Reset (reuse) or Release (return to the GC).
//
// Nodes returned by NewNode stay valid until Reset/Release is called; callers
// must not retain them past that point.
type NodeArena struct {
	slabSize int
	slabs    [][]TraceNode
	slabIdx  int
	used     int

	allocated int
	recycled  int
}

// NewNodeArena creates an arena whose slabs hold slabSize nodes each.
// A non-positive slabSize falls back to DefaultArenaSlabSize.
func NewNodeArena(slabSize int) *NodeArena {
	if slabSize <= 0 {
		slabSize = DefaultArenaSlabSize
	}
	return &NodeArena{slabSize: slabSize}
}

// NewNode hands out a zeroed, arena-owned TraceNode with the given id and type.
//
// Unlike NewTraceNode it leaves Children nil: symbols are appended on demand by
// AddChild, which keeps a freshly parsed node from allocating a backing array it
// may never use. Any data left over from a previous Reset is cleared so recycled
// slots never expose stale nodes, parents or source references.
func (a *NodeArena) NewNode(id, nodeType string) *TraceNode {
	n := a.alloc()
	*n = TraceNode{
		ID:       id,
		Type:     nodeType,
		Expanded: true,
	}
	return n
}

// alloc bumps the arena and returns the next node slot.
func (a *NodeArena) alloc() *TraceNode {
	if a.used >= a.slabSize {
		a.slabIdx++
		a.used = 0
	}
	for a.slabIdx >= len(a.slabs) {
		a.slabs = append(a.slabs, make([]TraceNode, a.slabSize))
	}
	n := &a.slabs[a.slabIdx][a.used]
	a.used++
	a.allocated++
	return n
}

// Reset releases every node in one go while keeping the slabs for reuse. It is
// the "drop them together" half of the arena: no per-node free, no GC pressure
// on the next parse that reuses the same region.
func (a *NodeArena) Reset() {
	a.recycled += a.allocated
	a.allocated = 0
	a.slabIdx = 0
	a.used = 0
}

// Release drops the underlying slabs so the garbage collector can reclaim them.
// A released arena is still usable and reallocates on the next NewNode.
func (a *NodeArena) Release() {
	a.slabs = nil
	a.slabIdx = 0
	a.used = 0
	a.allocated = 0
}

// Allocated reports how many nodes have been handed out since the last Reset.
func (a *NodeArena) Allocated() int { return a.allocated }

// Recycled reports how many nodes were freed by Reset across the arena's life.
func (a *NodeArena) Recycled() int { return a.recycled }

// SlabSize reports the configured number of nodes per slab.
func (a *NodeArena) SlabSize() int { return a.slabSize }

// NodeArenaStats is a snapshot of arena usage, useful for tracing and metrics.
type NodeArenaStats struct {
	// Allocated is the number of live nodes handed out since the last Reset.
	Allocated int `json:"allocated"`
	// Capacity is how many nodes the current slabs can hold without growing.
	Capacity int `json:"capacity"`
	// Slabs is the number of allocated slabs.
	Slabs int `json:"slabs"`
	// SlabSize is the configured nodes-per-slab count.
	SlabSize int `json:"slab_size"`
	// Recycled counts nodes freed by Reset over the arena's lifetime.
	Recycled int `json:"recycled"`
}

// Stats returns a snapshot of the arena's current usage.
func (a *NodeArena) Stats() NodeArenaStats {
	return NodeArenaStats{
		Allocated: a.allocated,
		Capacity:  len(a.slabs) * a.slabSize,
		Slabs:     len(a.slabs),
		SlabSize:  a.slabSize,
		Recycled:  a.recycled,
	}
}
