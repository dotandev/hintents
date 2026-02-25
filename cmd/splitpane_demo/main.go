// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// splitpane_demo renders a sample split-pane view to stdout so you can
// screenshot it for the Issue #309 PR.
//
// Usage:
//
//	go run ./cmd/splitpane_demo/
package main

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/trace"
)

func main() {
	// ── Demo 1: node with no source mapping ─────────────────────────────────
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  Demo 1 — contract_call node (no WASM debug symbols)")
	fmt.Println("══════════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	node1 := &trace.TraceNode{
		ID:         "call-1",
		Type:       "contract_call",
		ContractID: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
		Function:   "transfer",
		EventData:  "amount=500 from=GDQP… to=GBKQ…",
		Depth:      1,
	}

	pane := trace.DefaultSplitPane()
	pane.Render(os.Stdout, node1, nil)

	// ── Demo 2: error node with no source mapping ────────────────────────────
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  Demo 2 — error node")
	fmt.Println("══════════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	node2 := &trace.TraceNode{
		ID:         "call-2",
		Type:       "error",
		ContractID: "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE",
		Function:   "swap",
		Error:      "Insufficient balance: required 1000, available 450",
		Depth:      2,
	}

	pane.Render(os.Stdout, node2, nil)

	// ── Demo 3: node with a live source mapping ──────────────────────────────
	// Points at splitpane.go itself so the source pane always has real content.
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════════════")
	fmt.Println("  Demo 3 — contract_call node with Rust source context")
	fmt.Println("══════════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Use splitpane.go line 40 as a stand-in for "Rust source" so the demo
	// always works without needing real WASM debug symbols.
	ref := trace.SourceRef{
		File:     "internal/trace/splitpane.go",
		Line:     40,
		Column:   1,
		Function: "LoadSourceContext",
	}

	// Resolve to absolute path relative to repo root.
	repoRoot, err := os.Getwd()
	if err != nil {
		repoRoot = "."
	}
	ref.File = repoRoot + "/" + ref.File

	src, err := trace.LoadSourceContext(ref, 6)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load source context: %v\n", err)
	}

	node3 := &trace.TraceNode{
		ID:         "call-3",
		Type:       "contract_call",
		ContractID: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
		Function:   "mint",
		EventData:  "recipient=GDQP… amount=250000",
		Depth:      0,
		SourceRef:  &ref,
	}

	pane.Render(os.Stdout, node3, src)

	fmt.Println()
}
