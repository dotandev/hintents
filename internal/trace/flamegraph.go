// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import "fmt"

// FlamegraphKind selects which resource profile should be rendered.
type FlamegraphKind string

const (
	FlamegraphKindCPU              FlamegraphKind = "cpu"
	FlamegraphKindMemoryAllocation FlamegraphKind = "memory-allocation"
)

// BuildFlamegraphInput produces the folded stack format used by flamegraph
// renderers. CPU profiles emit a CPU label while memory-allocation profiles
// emit a dedicated allocation label to make memory bloat visible.
func BuildFlamegraphInput(cpuInsns, memBytes uint64, kind FlamegraphKind) string {
	label := "CPU"
	if kind == FlamegraphKindMemoryAllocation {
		label = "Memory Allocation"
	}

	return fmt.Sprintf("Total;%s %d\nTotal;Memory %d\n", label, cpuInsns, memBytes)
}
