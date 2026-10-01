// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"testing"
)

// benchSimulationResponse builds a response large enough to exercise the
// thousands-of-nodes path the arena exists for.
func benchSimulationResponse(events int) *SimulationResponse {
	contractID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	instruction := "i64.const 1"

	evs := make([]string, 0, events)
	logs := make([]string, 0, events)
	diags := make([]DiagnosticEvent, 0, events)
	for i := 0; i < events; i++ {
		evs = append(evs, fmt.Sprintf("event %d contract: %s fn: transfer", i, contractID))
		logs = append(logs, fmt.Sprintf("log %d", i))
		diags = append(diags, DiagnosticEvent{
			EventType:       "budget",
			ContractID:      &contractID,
			Data:            fmt.Sprintf("data %d", i),
			WasmInstruction: &instruction,
		})
	}

	return &SimulationResponse{
		Status:           "success",
		Events:           evs,
		Logs:             logs,
		DiagnosticEvents: diags,
	}
}

func BenchmarkParseSimulationResponse_Heap(b *testing.B) {
	resp := benchSimulationResponse(2_000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root, err := ParseSimulationResponse(resp)
		if err != nil {
			b.Fatal(err)
		}
		if len(root.Children) == 0 {
			b.Fatal("empty tree")
		}
	}
}

func BenchmarkParseSimulationResponseWithArena_Reused(b *testing.B) {
	resp := benchSimulationResponse(2_000)
	arena := NewNodeArena(DefaultArenaSlabSize)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		root, err := ParseSimulationResponseWithArena(resp, arena)
		if err != nil {
			b.Fatal(err)
		}
		if len(root.Children) == 0 {
			b.Fatal("empty tree")
		}
		arena.Reset()
	}
}

func BenchmarkNodeArena_NewNode(b *testing.B) {
	arena := NewNodeArena(DefaultArenaSlabSize)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		arena.NewNode("id", "event")
		if arena.Allocated() == arena.SlabSize() {
			arena.Reset()
		}
	}
}
