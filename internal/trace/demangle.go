// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"github.com/dotandev/hintents/internal/demangle"
)

// DemangleNode rewrites the Function field of a single TraceNode using the
// provided symbol table. If Function contains a raw WASM reference like
// "func[42]" it is replaced with the demangled Rust name, e.g.
// "my_contract::invoke".
//
// The ContractID and EventData fields are also scanned so that any func[N]
// references embedded in those strings are resolved too.
//
// The node is modified in-place. Children are not touched; use
// DemangleTree to process the full tree recursively.
func DemangleNode(node *TraceNode, table demangle.SymbolTable) {
	if len(table) == 0 {
		return
	}

	node.Function = demangle.DemangleTrace(node.Function, table)
	node.EventData = demangle.DemangleTrace(node.EventData, table)
	node.ContractID = demangle.DemangleTrace(node.ContractID, table)
}

// DemangleTree walks the full trace tree rooted at node and rewrites every
// node's Function, EventData, and ContractID fields in-place.
//
// Calls this once after receiving trace data from the simulator, before the
// tree is rendered in the TUI.
func DemangleTree(root *TraceNode, table demangle.SymbolTable) {
	if root == nil || len(table) == 0 {
		return
	}

	DemangleNode(root, table)

	for _, child := range root.Children {
		DemangleTree(child, table)
	}
}