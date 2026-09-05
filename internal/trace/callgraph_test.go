// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"strings"
	"testing"
)

// sampleANSITrace mirrors the output of PrintExecutionTrace, including the
// ANSI colour codes the interactive viewer and simulator emit.
const sampleANSITrace = "\x1b[1m Transaction Execution Trace\x1b[0m\n" +
	" Hash  : 5c0a1234567890abcdef\n" +
	" Start : 2026-08-30T07:00:00Z\n" +
	" Steps : 3\n" +
	" ──────────────────────────────────\n" +
	"▸ TX  5c0a1234…\n" +
	"├─ \x1b[90m[0]\x1b[0m \x1b[96m◆ CONTRACT_CALL\x1b[0m  \x1b[96mtransfer\x1b[0m  \x1b[90mCDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC\x1b[0m\n" +
	"│  CPU: 150,000  MEM: 2.0 KB\n" +
	"├─ \x1b[90m[1]\x1b[0m \x1b[32m⚙ HOST_FUNCTION\x1b[0m  \x1b[32mrequire_auth\x1b[0m\n" +
	"└─ \x1b[90m[2]\x1b[0m \x1b[96m◆ CONTRACT_CALL\x1b[0m  \x1b[96mswap\x1b[0m  \x1b[90mCA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE\x1b[0m\n" +
	"   \x1b[91m[FAIL]\x1b[0m Insufficient balance\n"

func TestParseANSITrace_StripsANSIAndBuildsGraph(t *testing.T) {
	graph, err := ParseANSITrace(sampleANSITrace)
	if err != nil {
		t.Fatalf("ParseANSITrace returned error: %v", err)
	}

	if got := len(graph.Nodes()); got != 4 {
		t.Fatalf("expected 4 nodes (root + 3 steps), got %d", got)
	}

	root := graph.Root
	if got := len(root.Calls); got != 1 {
		t.Fatalf("expected 1 top-level call, got %d", got)
	}

	transfer := root.Calls[0]
	if transfer.Function != "transfer" {
		t.Errorf("expected function transfer, got %q", transfer.Function)
	}
	if transfer.ContractID != "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC" {
		t.Errorf("unexpected contract id %q", transfer.ContractID)
	}
	if transfer.CPUDelta != 150000 {
		t.Errorf("expected CPU 150000, got %d", transfer.CPUDelta)
	}
	if transfer.MemoryDelta != 2048 {
		t.Errorf("expected MEM 2048, got %d", transfer.MemoryDelta)
	}

	if got := len(transfer.Calls); got != 2 {
		t.Fatalf("expected 2 nested calls, got %d", got)
	}

	hostFn := transfer.Calls[0]
	if hostFn.Function != "require_auth" || hostFn.EventType != EventTypeHostFunction {
		t.Errorf("unexpected host function node: %+v", hostFn)
	}

	swap := transfer.Calls[1]
	if swap.Function != "swap" {
		t.Errorf("expected function swap, got %q", swap.Function)
	}
	if swap.Error != "Insufficient balance" {
		t.Errorf("expected error text, got %q", swap.Error)
	}

	if got := len(graph.Edges); got != 3 {
		t.Fatalf("expected 3 call edges, got %d", got)
	}
}

func TestParseANSITrace_EmptyInput(t *testing.T) {
	if _, err := ParseANSITrace("   \n\n"); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseANSITrace_PlainText(t *testing.T) {
	input := " Transaction Execution Trace\n" +
		" Steps : 2\n" +
		"──────\n" +
		"├─ [0] ◆ CONTRACT_CALL  transfer  CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC\n" +
		"└─ [1] ◆ CONTRACT_CALL  swap  CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE\n"

	graph, err := ParseANSITrace(input)
	if err != nil {
		t.Fatalf("ParseANSITrace returned error: %v", err)
	}
	if got := len(graph.Nodes()); got != 3 {
		t.Fatalf("expected 3 nodes, got %d", got)
	}
	if got := graph.Root.Calls[0].Calls[0].Function; got != "swap" {
		t.Errorf("expected nested swap, got %q", got)
	}
}

func TestCallGraph_Find(t *testing.T) {
	graph, err := ParseANSITrace(sampleANSITrace)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(graph.Find("transfer")); got != 1 {
		t.Fatalf("expected 1 match for transfer, got %d", got)
	}
	if got := len(graph.Find("SWAP")); got != 1 {
		t.Fatalf("expected 1 case-insensitive match for swap, got %d", got)
	}
	if got := len(graph.Find("missing")); got != 0 {
		t.Fatalf("expected 0 matches for missing, got %d", got)
	}
}

func TestCallGraph_RenderInteractiveHTML(t *testing.T) {
	graph, err := ParseANSITrace(sampleANSITrace)
	if err != nil {
		t.Fatal(err)
	}

	html := graph.RenderInteractiveHTML()
	if !strings.Contains(html, "Erst Interactive Call Graph") {
		t.Error("missing page title")
	}
	if !strings.Contains(html, "transfer") {
		t.Error("missing transfer node")
	}
	if !strings.Contains(html, "Insufficient balance") {
		t.Error("missing error text")
	}
	if !strings.Contains(html, "&rarr;") {
		t.Error("missing call edge arrow")
	}
}
