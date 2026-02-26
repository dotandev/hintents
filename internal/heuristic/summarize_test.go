// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package heuristic

import (
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
)

func strPtr(s string) *string { return &s }

func TestSummarize_Success(t *testing.T) {
	in := Input{
		TxHash:  "abcdef123456",
		Network: "mainnet",
		Status:  "success",
	}
	got := Summarize(in)
	if !strings.Contains(got, "successfully") {
		t.Fatalf("expected success message, got: %s", got)
	}
}

func TestSummarize_AuthCrossContract(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "testnet",
		Status:  "error",
		Error:   "Error(Auth, InvalidAction)",
		DiagnosticEvents: []simulator.DiagnosticEvent{
			{ContractID: strPtr("CABC"), EventType: "contract", Topics: []string{"call"}},
			{ContractID: strPtr("CDEF"), EventType: "contract", Topics: []string{"require_auth"}},
		},
	}
	got := Summarize(in)
	if !strings.Contains(got, "CABC") || !strings.Contains(got, "CDEF") {
		t.Fatalf("expected both contract IDs in summary, got: %s", got)
	}
	if !strings.Contains(got, "authorization") {
		t.Fatalf("expected 'authorization' in summary, got: %s", got)
	}
}

func TestSummarize_AuthSingleContract(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "Error(Auth, NotAuthorized)",
		DiagnosticEvents: []simulator.DiagnosticEvent{
			{ContractID: strPtr("CXYZ"), EventType: "contract", Topics: []string{"require_auth"}},
		},
	}
	got := Summarize(in)
	if !strings.Contains(got, "CXYZ") {
		t.Fatalf("expected contract ID in summary, got: %s", got)
	}
	if !strings.Contains(got, "authorization") {
		t.Fatalf("expected 'authorization' in summary, got: %s", got)
	}
}

func TestSummarize_AuthNoContracts(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "not authorized",
	}
	got := Summarize(in)
	if !strings.Contains(got, "authorization") {
		t.Fatalf("expected authorization mention, got: %s", got)
	}
}

func TestSummarize_CPUBudgetExceeded(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "Error(Budget, CpuLimitExceeded)",
	}
	got := Summarize(in)
	if !strings.Contains(got, "CPU") {
		t.Fatalf("expected CPU mention, got: %s", got)
	}
}

func TestSummarize_CPUBudgetExceededViaBudgetUsage(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		BudgetUsage: &simulator.BudgetUsage{
			CPUUsagePercent: 100.5,
		},
	}
	got := Summarize(in)
	if !strings.Contains(got, "CPU") {
		t.Fatalf("expected CPU mention, got: %s", got)
	}
}

func TestSummarize_MemBudgetExceeded(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "Error(Budget, MemLimitExceeded)",
	}
	got := Summarize(in)
	if !strings.Contains(got, "memory") {
		t.Fatalf("expected memory mention, got: %s", got)
	}
}

func TestSummarize_MemBudgetExceededViaBudgetUsage(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		BudgetUsage: &simulator.BudgetUsage{
			MemoryUsagePercent: 101.0,
		},
	}
	got := Summarize(in)
	if !strings.Contains(got, "memory") {
		t.Fatalf("expected memory mention, got: %s", got)
	}
}

func TestSummarize_BothBudgetsExceeded(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		BudgetUsage: &simulator.BudgetUsage{
			CPUUsagePercent:    100.0,
			MemoryUsagePercent: 100.0,
		},
	}
	got := Summarize(in)
	if !strings.Contains(got, "CPU") || !strings.Contains(got, "memory") {
		t.Fatalf("expected both CPU and memory mention, got: %s", got)
	}
}

func TestSummarize_InsufficientBalance(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "testnet",
		Status:  "error",
		Error:   "insufficient_balance",
	}
	got := Summarize(in)
	if !strings.Contains(got, "balance") {
		t.Fatalf("expected balance mention, got: %s", got)
	}
}

func TestSummarize_MissingEntry(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "Error(Storage, MissingValue)",
	}
	got := Summarize(in)
	if !strings.Contains(got, "ledger entry") {
		t.Fatalf("expected ledger entry mention, got: %s", got)
	}
}

func TestSummarize_WasmTrap(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "wasm trap: unreachable",
	}
	got := Summarize(in)
	if !strings.Contains(got, "WASM trap") {
		t.Fatalf("expected WASM trap mention, got: %s", got)
	}
}

func TestSummarize_FallbackWithError(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Error:   "some unknown simulator error",
	}
	got := Summarize(in)
	if !strings.Contains(got, "some unknown simulator error") {
		t.Fatalf("expected raw error in fallback, got: %s", got)
	}
}

func TestSummarize_FallbackNoError(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
	}
	got := Summarize(in)
	if !strings.Contains(got, "failed") {
		t.Fatalf("expected failure mention in fallback, got: %s", got)
	}
}

func TestSummarize_AuthDetectedFromEvents(t *testing.T) {
	in := Input{
		TxHash:  "aaaaaa000000bbbbbb",
		Network: "mainnet",
		Status:  "error",
		Events:  []string{"require_auth check failed for contract CXYZ"},
	}
	got := Summarize(in)
	if !strings.Contains(got, "authorization") {
		t.Fatalf("expected authorization mention from event signal, got: %s", got)
	}
}

func TestSummarize_ShortHashPassthrough(t *testing.T) {
	in := Input{
		TxHash:  "short",
		Network: "testnet",
		Status:  "success",
	}
	got := Summarize(in)
	if !strings.Contains(got, "short") {
		t.Fatalf("expected full short hash in output, got: %s", got)
	}
}
