// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/trace"
)

func TestDiffEngine_Compare(t *testing.T) {
	engine := NewDiffEngine()

	t.Run("Identical traces", func(t *testing.T) {
		base := trace.NewTraceNode("root", "call")
		base.ContractID = "C1"
		base.Function = "F1"
		cpu := uint64(100)
		base.CPUDelta = &cpu

		head := trace.NewTraceNode("root", "call")
		head.ContractID = "C1"
		head.Function = "F1"
		head.CPUDelta = &cpu

		diff := engine.Compare(base, head)
		if diff.Type != DiffUnchanged {
			t.Errorf("Expected DiffUnchanged, got %s", diff.Type)
		}
		if diff.CPUDiff != 0 {
			t.Errorf("Expected CPUDiff 0, got %d", diff.CPUDiff)
		}
	})

	t.Run("Budget difference", func(t *testing.T) {
		base := trace.NewTraceNode("root", "call")
		cpuBase := uint64(100)
		base.CPUDelta = &cpuBase

		head := trace.NewTraceNode("root", "call")
		cpuHead := uint64(150)
		head.CPUDelta = &cpuHead

		diff := engine.Compare(base, head)
		if diff.Type != DiffModified {
			t.Errorf("Expected DiffModified, got %s", diff.Type)
		}
		if diff.CPUDiff != 50 {
			t.Errorf("Expected CPUDiff 50, got %d", diff.CPUDiff)
		}
	})

	t.Run("State change difference", func(t *testing.T) {
		base := trace.NewTraceNode("root", "call")
		base.StateChanges = []trace.StateChange{
			{Type: "put", Key: "K1", Value: "V1"},
		}

		head := trace.NewTraceNode("root", "call")
		head.StateChanges = []trace.StateChange{
			{Type: "put", Key: "K1", Value: "V2"},
		}

		diff := engine.Compare(base, head)
		if diff.Type != DiffModified {
			t.Errorf("Expected DiffModified, got %s", diff.Type)
		}
		if len(diff.StateDiffs) != 1 {
			t.Errorf("Expected 1 state diff, got %d", len(diff.StateDiffs))
		}
		if diff.StateDiffs[0].Type != DiffModified {
			t.Errorf("Expected StateDiff Modified, got %s", diff.StateDiffs[0].Type)
		}
	})

	t.Run("Call tree difference", func(t *testing.T) {
		base := trace.NewTraceNode("root", "call")
		child1 := trace.NewTraceNode("c1", "call")
		base.AddChild(child1)

		head := trace.NewTraceNode("root", "call")
		child1_head := trace.NewTraceNode("c1", "call")
		child2_head := trace.NewTraceNode("c2", "call")
		head.AddChild(child1_head)
		head.AddChild(child2_head)

		diff := engine.Compare(base, head)
		if diff.Type != DiffUnchanged { // Root node itself hasn't changed
			t.Errorf("Expected root DiffUnchanged, got %s", diff.Type)
		}
		if !diff.Divergent {
			t.Error("Expected root to be marked as Divergent due to children")
		}
		if len(diff.Children) != 2 {
			t.Errorf("Expected 2 child diffs, got %d", len(diff.Children))
		}
		if diff.Children[1].Type != DiffAdded {
			t.Errorf("Expected second child to be DiffAdded, got %s", diff.Children[1].Type)
		}
	})
}

func TestRenderDiff(t *testing.T) {
	base := trace.NewTraceNode("root", "contract_call")
	base.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	base.Function = "transfer"
	cpu1 := uint64(1000)
	base.CPUDelta = &cpu1

	head := trace.NewTraceNode("root", "contract_call")
	head.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	head.Function = "transfer"
	cpu2 := uint64(1200)
	head.CPUDelta = &cpu2
	head.StateChanges = []trace.StateChange{
		{Type: "put", Key: "BALANCE_KEY", Value: "100"},
	}

	engine := NewDiffEngine()
	diff := engine.Compare(base, head)
	output := RenderDiff(diff, 0)

	// Verify output contains key information (without checking exact ANSI sequences)
	if !strings.Contains(output, "transfer") {
		t.Error("Output should contain function name 'transfer'")
	}
	if !strings.Contains(output, "CPU:+200") {
		t.Error("Output should contain CPU delta '+200'")
	}
	if !strings.Contains(output, "STATE: put BALANCE_") {
		t.Error("Output should contain state change info")
	}
}
