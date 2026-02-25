// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package compare

import (
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func ptr(s string) *string { return &s }

func makeResp(status string, events []string, diag []simulator.DiagnosticEvent, budget *simulator.BudgetUsage) *simulator.SimulationResponse {
	return &simulator.SimulationResponse{
		Status:           status,
		Events:           events,
		DiagnosticEvents: diag,
		BudgetUsage:      budget,
	}
}

// ─── StatusDiff ──────────────────────────────────────────────────────────────

func TestDiff_StatusMatch(t *testing.T) {
	local := makeResp("success", nil, nil, nil)
	onChain := makeResp("success", nil, nil, nil)
	result := Diff(local, onChain)
	assert.True(t, result.StatusDiff.Match, "status should match")
	assert.False(t, result.HasDivergence, "no divergence expected")
}

func TestDiff_StatusMismatch(t *testing.T) {
	local := makeResp("error", nil, nil, nil)
	onChain := makeResp("success", nil, nil, nil)
	result := Diff(local, onChain)
	assert.False(t, result.StatusDiff.Match, "status mismatch expected")
	assert.True(t, result.HasDivergence, "divergence expected")
	assert.Equal(t, "error", result.StatusDiff.LocalStatus)
	assert.Equal(t, "success", result.StatusDiff.OnChainStatus)
}

// ─── RawEvent diff ────────────────────────────────────────────────────────────

func TestDiff_IdenticalEvents(t *testing.T) {
	evts := []string{"evt:mint", "evt:transfer"}
	local := makeResp("success", evts, nil, nil)
	onChain := makeResp("success", evts, nil, nil)
	result := Diff(local, onChain)
	require.Len(t, result.EventDiffs, 2)
	assert.False(t, result.EventDiffs[0].Divergent)
	assert.False(t, result.EventDiffs[1].Divergent)
	assert.Equal(t, 0, result.DivergentEvents)
}

func TestDiff_DivergentEvents(t *testing.T) {
	local := makeResp("success", []string{"evt:mint", "evt:burn"}, nil, nil)
	onChain := makeResp("success", []string{"evt:mint", "evt:transfer"}, nil, nil)
	result := Diff(local, onChain)
	require.Len(t, result.EventDiffs, 2)
	assert.False(t, result.EventDiffs[0].Divergent, "first event should match")
	assert.True(t, result.EventDiffs[1].Divergent, "second event should differ")
	assert.Equal(t, 1, result.DivergentEvents)
	assert.True(t, result.HasDivergence)
}

func TestDiff_MissingEventOnOnChain(t *testing.T) {
	local := makeResp("success", []string{"evt:a", "evt:b"}, nil, nil)
	onChain := makeResp("success", []string{"evt:a"}, nil, nil)
	result := Diff(local, onChain)
	require.Len(t, result.EventDiffs, 2)
	assert.False(t, result.EventDiffs[0].Divergent)
	assert.True(t, result.EventDiffs[1].Divergent, "extra local event should be divergent")
	assert.Equal(t, "<absent>", result.EventDiffs[1].OnChainEvent, "absent on-chain event shows <absent>")
}

func TestDiff_MissingEventOnLocal(t *testing.T) {
	local := makeResp("success", []string{"evt:a"}, nil, nil)
	onChain := makeResp("success", []string{"evt:a", "evt:b"}, nil, nil)
	result := Diff(local, onChain)
	require.Len(t, result.EventDiffs, 2)
	assert.True(t, result.EventDiffs[1].Divergent, "extra on-chain event should be divergent")
	assert.Equal(t, "<absent>", result.EventDiffs[1].LocalEvent, "absent local event shows <absent>")
}

// ─── DiagnosticEvent diff ─────────────────────────────────────────────────────

func TestDiff_IdenticalDiagnosticEvents(t *testing.T) {
	cid := "CONTRACT_A"
	diag := []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr(cid), Topics: []string{"fn_call"}, Data: "ok"},
	}
	local := makeResp("success", nil, diag, nil)
	onChain := makeResp("success", nil, diag, nil)
	result := Diff(local, onChain)
	require.Len(t, result.DiagnosticDiffs, 1)
	assert.False(t, result.DiagnosticDiffs[0].Divergent)
	assert.False(t, result.DiagnosticDiffs[0].DivergentPath)
}

func TestDiff_DiagnosticEventTypeMismatch(t *testing.T) {
	local := makeResp("success", nil, []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID"), Topics: []string{"fn_call"}, Data: "ok"},
	}, nil)
	onChain := makeResp("success", nil, []simulator.DiagnosticEvent{
		{EventType: "system", ContractID: ptr("CID"), Topics: []string{"fn_call"}, Data: "ok"},
	}, nil)
	result := Diff(local, onChain)
	require.Len(t, result.DiagnosticDiffs, 1)
	assert.True(t, result.DiagnosticDiffs[0].Divergent)
	assert.True(t, result.DiagnosticDiffs[0].DivergentPath, "different event type = divergent path")
}

func TestDiff_DiagnosticContractIDMismatch(t *testing.T) {
	local := makeResp("success", nil, []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID_A"), Topics: []string{"transfer"}, Data: "ok"},
	}, nil)
	onChain := makeResp("success", nil, []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID_B"), Topics: []string{"transfer"}, Data: "ok"},
	}, nil)
	result := Diff(local, onChain)
	require.Len(t, result.DiagnosticDiffs, 1)
	assert.True(t, result.DiagnosticDiffs[0].DivergentPath, "different contract ID = divergent path")
	require.Len(t, result.CallPathDivergences, 1)
	assert.Contains(t, result.CallPathDivergences[0].Reason, "CID_A")
}

func TestDiff_DiagnosticDataMismatch_NotPathDivergent(t *testing.T) {
	local := makeResp("success", nil, []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID"), Topics: []string{"log"}, Data: "old"},
	}, nil)
	onChain := makeResp("success", nil, []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID"), Topics: []string{"log"}, Data: "new"},
	}, nil)
	result := Diff(local, onChain)
	require.Len(t, result.DiagnosticDiffs, 1)
	assert.True(t, result.DiagnosticDiffs[0].Divergent, "data mismatch = divergent")
	assert.False(t, result.DiagnosticDiffs[0].DivergentPath, "same type+contract = not path-divergent")
	assert.Len(t, result.CallPathDivergences, 0)
}

// ─── BudgetDiff ───────────────────────────────────────────────────────────────

func TestDiff_BudgetNil(t *testing.T) {
	local := makeResp("success", nil, nil, nil)
	onChain := makeResp("success", nil, nil, nil)
	result := Diff(local, onChain)
	assert.Nil(t, result.BudgetDiff, "no budget diff when both sides are nil")
}

func TestDiff_BudgetDeltaCalculation(t *testing.T) {
	local := makeResp("success", nil, nil, &simulator.BudgetUsage{
		CPUInstructions: 2000,
		MemoryBytes:     512,
		OperationsCount: 3,
	})
	onChain := makeResp("success", nil, nil, &simulator.BudgetUsage{
		CPUInstructions: 1500,
		MemoryBytes:     256,
		OperationsCount: 2,
	})
	result := Diff(local, onChain)
	require.NotNil(t, result.BudgetDiff)
	assert.Equal(t, int64(500), result.BudgetDiff.CPUDelta)
	assert.Equal(t, int64(256), result.BudgetDiff.MemoryDelta)
	assert.Equal(t, 1, result.BudgetDiff.OpsDelta)
}

func TestDiff_BudgetNegativeDelta(t *testing.T) {
	local := makeResp("success", nil, nil, &simulator.BudgetUsage{
		CPUInstructions: 1000,
	})
	onChain := makeResp("success", nil, nil, &simulator.BudgetUsage{
		CPUInstructions: 1200,
	})
	result := Diff(local, onChain)
	require.NotNil(t, result.BudgetDiff)
	assert.Equal(t, int64(-200), result.BudgetDiff.CPUDelta, "local uses fewer instructions than on-chain")
}

// ─── CallPathDivergences ──────────────────────────────────────────────────────

func TestDiff_CallPathDivergences_Multiple(t *testing.T) {
	localDiag := []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID_A"), Topics: []string{"fn1"}, Data: "ok"},
		{EventType: "contract", ContractID: ptr("CID_A"), Topics: []string{"fn2"}, Data: "ok"},
	}
	onChainDiag := []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID_A"), Topics: []string{"fn1"}, Data: "ok"},
		{EventType: "contract", ContractID: ptr("CID_B"), Topics: []string{"fn2"}, Data: "ok"},
	}
	local := makeResp("success", nil, localDiag, nil)
	onChain := makeResp("success", nil, onChainDiag, nil)
	result := Diff(local, onChain)
	assert.Len(t, result.CallPathDivergences, 1, "only the second event differs in path")
	assert.Equal(t, 1, result.CallPathDivergences[0].EventIndex)
}

func TestDiff_CallPathDivergences_AbsentEvent(t *testing.T) {
	localDiag := []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID"), Topics: []string{"fn1"}, Data: "ok"},
		{EventType: "contract", ContractID: ptr("CID"), Topics: []string{"fn2"}, Data: "ok"},
	}
	onChainDiag := []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("CID"), Topics: []string{"fn1"}, Data: "ok"},
	}
	local := makeResp("success", nil, localDiag, nil)
	onChain := makeResp("success", nil, onChainDiag, nil)
	result := Diff(local, onChain)
	assert.Len(t, result.CallPathDivergences, 1)
	assert.Contains(t, result.CallPathDivergences[0].Reason, "event present in one run only")
	assert.Equal(t, "<absent>", result.CallPathDivergences[0].OnChainSummary)
}

// ─── Summary fields ───────────────────────────────────────────────────────────

func TestDiff_SummaryCounters(t *testing.T) {
	local := makeResp("success", []string{"e1", "e2", "e3"}, nil, nil)
	onChain := makeResp("success", []string{"e1", "e2", "X3"}, nil, nil)
	result := Diff(local, onChain)
	assert.Equal(t, 3, result.TotalEvents)
	assert.Equal(t, 2, result.IdenticalEvents)
	assert.Equal(t, 1, result.DivergentEvents)
}

// ─── truncate helper ─────────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 6))
	assert.Equal(t, "abc", truncate("abcdef", 3))
}

// ─── formatDelta helpers ──────────────────────────────────────────────────────

func TestFormatDelta(t *testing.T) {
	assert.Equal(t, "+42", formatDelta(42))
	assert.Equal(t, "-42", formatDelta(-42))
	assert.Equal(t, "0", formatDelta(0))
}

// ─── Render smoke test ────────────────────────────────────────────────────────

// TestRender_NoError verifies Render does not panic on any valid DiffResult.
func TestRender_NoError(t *testing.T) {
	local := makeResp("success", []string{"evt:mint"}, []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("C1"), Topics: []string{"mint"}, Data: "1000"},
	}, &simulator.BudgetUsage{
		CPUInstructions: 1000, MemoryBytes: 512, OperationsCount: 2,
		CPULimit: 10000, MemoryLimit: 5120,
	})
	onChain := makeResp("success", []string{"evt:mint"}, []simulator.DiagnosticEvent{
		{EventType: "contract", ContractID: ptr("C1"), Topics: []string{"mint"}, Data: "999"},
	}, &simulator.BudgetUsage{
		CPUInstructions: 900, MemoryBytes: 480, OperationsCount: 2,
		CPULimit: 10000, MemoryLimit: 5120,
	})

	result := Diff(local, onChain)
	assert.NotPanics(t, func() {
		Render(result)
	})
}

func TestRender_NilResult_NoError(t *testing.T) {
	assert.NotPanics(t, func() {
		Render(nil)
	})
}
