// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace_test

import (
	"testing"

	"github.com/dotandev/hintents/internal/demangle"
	"github.com/dotandev/hintents/internal/trace"
)

// testTable returns a small symbol table used across all tests.
func testTable() demangle.SymbolTable {
	return demangle.SymbolTable{
		42: "_ZN11my_contract6invoke17h1a2b3c4d5e6f7890E",
		7:  "_ZN11soroban_sdk3log17habcdef1234567890E",
	}
}

// ---------------------------------------------------------------------------
// DemangleNode
// ---------------------------------------------------------------------------

func TestDemangleNode_RewritesFunction(t *testing.T) {
	node := trace.NewTraceNode("n1", "contract_call")
	node.Function = "func[42]"

	trace.DemangleNode(node, testTable())

	want := "my_contract::invoke"
	if node.Function != want {
		t.Errorf("Function = %q, want %q", node.Function, want)
	}
}

func TestDemangleNode_RewritesEventData(t *testing.T) {
	node := trace.NewTraceNode("n1", "event")
	node.EventData = "called func[7] with args"

	trace.DemangleNode(node, testTable())

	want := "called soroban_sdk::log with args"
	if node.EventData != want {
		t.Errorf("EventData = %q, want %q", node.EventData, want)
	}
}

func TestDemangleNode_PreservesReadableFunction(t *testing.T) {
	node := trace.NewTraceNode("n1", "contract_call")
	node.Function = "transfer"

	trace.DemangleNode(node, testTable())

	if node.Function != "transfer" {
		t.Errorf("Function changed unexpectedly: got %q", node.Function)
	}
}

func TestDemangleNode_NilTable(t *testing.T) {
	node := trace.NewTraceNode("n1", "contract_call")
	node.Function = "func[42]"

	trace.DemangleNode(node, nil)

	// Must be unchanged with empty table
	if node.Function != "func[42]" {
		t.Errorf("Function changed with nil table: got %q", node.Function)
	}
}

func TestDemangleNode_PreservesUnknownFuncRef(t *testing.T) {
	node := trace.NewTraceNode("n1", "contract_call")
	node.Function = "func[999]"

	trace.DemangleNode(node, testTable())

	// func[999] is not in the table - must be kept as-is
	if node.Function != "func[999]" {
		t.Errorf("unknown func ref was modified: got %q", node.Function)
	}
}

// ---------------------------------------------------------------------------
// DemangleTree
// ---------------------------------------------------------------------------

func TestDemangleTree_RewritesRootAndChildren(t *testing.T) {
	root := trace.NewTraceNode("root", "transaction")
	root.Function = "func[42]"

	child := trace.NewTraceNode("child", "host_fn")
	child.Function = "func[7]"
	root.AddChild(child)

	trace.DemangleTree(root, testTable())

	if root.Function != "my_contract::invoke" {
		t.Errorf("root.Function = %q, want %q", root.Function, "my_contract::invoke")
	}
	if child.Function != "soroban_sdk::log" {
		t.Errorf("child.Function = %q, want %q", child.Function, "soroban_sdk::log")
	}
}

func TestDemangleTree_DeepNesting(t *testing.T) {
	root := trace.NewTraceNode("root", "transaction")

	level1 := trace.NewTraceNode("l1", "contract_call")
	level1.Function = "func[42]"
	root.AddChild(level1)

	level2 := trace.NewTraceNode("l2", "host_fn")
	level2.Function = "func[7]"
	level1.AddChild(level2)

	trace.DemangleTree(root, testTable())

	if level1.Function != "my_contract::invoke" {
		t.Errorf("level1.Function = %q, want %q", level1.Function, "my_contract::invoke")
	}
	if level2.Function != "soroban_sdk::log" {
		t.Errorf("level2.Function = %q, want %q", level2.Function, "soroban_sdk::log")
	}
}

func TestDemangleTree_NilRoot(t *testing.T) {
	trace.DemangleTree(nil, testTable())
}

func TestDemangleTree_EmptyTable(t *testing.T) {
	root := trace.NewTraceNode("root", "transaction")
	root.Function = "func[42]"

	trace.DemangleTree(root, nil)

	if root.Function != "func[42]" {
		t.Errorf("Function changed with nil table: got %q", root.Function)
	}
}

func TestDemangleTree_UsesMockTrace(t *testing.T) {
	// Use CreateMockTrace to ensure demangling does not
	// break or alter already human-readable function names.
	root := trace.CreateMockTrace()

	// the mock trace contains readable names like "transfer" or "swap",
	// not func[N] references. After demangling, they must remain unchanged.
	table := testTable()
	trace.DemangleTree(root, table)

	all := root.FlattenAll()
	for _, node := range all {
		if node.Function == "" {
			continue
		}
		// None of the mock functions are func[N] references, so all must
		// remain exactly as set by CreateMockTrace.
		if len(node.Function) > 5 && node.Function[:5] == "func[" {
			t.Errorf("readable function was mangled: %q", node.Function)
		}
	}
}
