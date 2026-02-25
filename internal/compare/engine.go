// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package compare implements the "Compare Replay" diffing engine for issue #105.
// It simultaneously replays a transaction against a local WASM file and the
// on-chain (mainnet/testnet) WASM, then produces a structured diff of events,
// diagnostic data, budget usage, and divergent call paths.
package compare

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
)

// Side labels used throughout the diff output.
const (
	SideLocal   = "local"
	SideOnChain = "on-chain"
)

// EventDiff represents a single positional divergence between two event slices.
type EventDiff struct {
	// Index is the 0-based position in the event stream.
	Index int

	// LocalEvent is the event from the local-WASM run ("" if absent).
	LocalEvent string

	// OnChainEvent is the event from the on-chain run ("" if absent).
	OnChainEvent string

	// Divergent is true when the two events differ.
	Divergent bool
}

// DiagnosticDiff is a positional divergence in the DiagnosticEvents slice.
type DiagnosticDiff struct {
	Index     int
	Local     *simulator.DiagnosticEvent
	OnChain   *simulator.DiagnosticEvent
	Divergent bool
	// DivergentPath is true when the contract ID or event type differs,
	// indicating the execution took a different call path.
	DivergentPath bool
}

// BudgetDiff holds the delta between local and on-chain budget consumption.
type BudgetDiff struct {
	CPUDelta    int64
	MemoryDelta int64
	OpsDelta    int

	LocalCPU   uint64
	OnChainCPU uint64
	LocalMem   uint64
	OnChainMem uint64
	LocalOps   int
	OnChainOps int
}

// StatusDiff holds the comparison of top-level execution status.
type StatusDiff struct {
	Match         bool
	LocalStatus   string
	OnChainStatus string
	LocalError    string
	OnChainError  string
}

// CallPathDivergence records a specific point where the two runs took different paths.
type CallPathDivergence struct {
	// EventIndex is the position in the diagnostic event stream where paths diverged.
	EventIndex int
	// Reason describes what differs (contract ID, event type, topic count, etc.).
	Reason string
	// LocalSummary is a short description of what happened locally at this point.
	LocalSummary string
	// OnChainSummary is a short description of what happened on-chain at this point.
	OnChainSummary string
}

// DiffResult holds the complete comparison output for a single replay pair.
type DiffResult struct {
	StatusDiff          StatusDiff
	EventDiffs          []EventDiff
	DiagnosticDiffs     []DiagnosticDiff
	BudgetDiff          *BudgetDiff
	CallPathDivergences []CallPathDivergence

	// Summary fields
	TotalEvents     int
	DivergentEvents int
	IdenticalEvents int
	HasDivergence   bool
}

// Diff compares two SimulationResponse objects (local vs on-chain) and returns
// a fully-populated DiffResult. Neither argument may be nil.
func Diff(local, onChain *simulator.SimulationResponse) *DiffResult {
	result := &DiffResult{}

	// 1. Status comparison
	result.StatusDiff = compareStatus(local, onChain)

	// 2. Raw event diff (backward-compat events slice)
	result.EventDiffs = compareRawEvents(local.Events, onChain.Events)

	// 3. Diagnostic event diff (structured)
	result.DiagnosticDiffs = compareDiagnosticEvents(local.DiagnosticEvents, onChain.DiagnosticEvents)

	// 4. Budget diff
	if local.BudgetUsage != nil || onChain.BudgetUsage != nil {
		result.BudgetDiff = compareBudget(local.BudgetUsage, onChain.BudgetUsage)
	}

	// 5. Call-path divergences (extracted from diagnostic diff)
	result.CallPathDivergences = extractCallPathDivergences(result.DiagnosticDiffs)

	// 6. Aggregate counters
	total := len(result.EventDiffs)
	div := 0
	for _, d := range result.EventDiffs {
		if d.Divergent {
			div++
		}
	}
	result.TotalEvents = total
	result.DivergentEvents = div
	result.IdenticalEvents = total - div
	result.HasDivergence = result.StatusDiff.Match == false ||
		div > 0 ||
		len(result.CallPathDivergences) > 0

	return result
}

// ─── internal helpers ────────────────────────────────────────────────────────

func compareStatus(local, onChain *simulator.SimulationResponse) StatusDiff {
	sd := StatusDiff{
		LocalStatus:   local.Status,
		OnChainStatus: onChain.Status,
		LocalError:    local.Error,
		OnChainError:  onChain.Error,
	}
	sd.Match = local.Status == onChain.Status
	return sd
}

func compareRawEvents(local, onChain []string) []EventDiff {
	maxLen := len(local)
	if len(onChain) > maxLen {
		maxLen = len(onChain)
	}

	diffs := make([]EventDiff, maxLen)
	for i := 0; i < maxLen; i++ {
		var le, oe string
		var leMissing, oeMissing bool
		if i < len(local) {
			le = local[i]
		} else {
			leMissing = true
		}
		if i < len(onChain) {
			oe = onChain[i]
		} else {
			oeMissing = true
		}
		divergent := le != oe
		if leMissing {
			le = "<absent>"
		}
		if oeMissing {
			oe = "<absent>"
		}
		diffs[i] = EventDiff{
			Index:        i,
			LocalEvent:   le,
			OnChainEvent: oe,
			Divergent:    divergent,
		}
	}
	return diffs
}

func compareDiagnosticEvents(local, onChain []simulator.DiagnosticEvent) []DiagnosticDiff {
	maxLen := len(local)
	if len(onChain) > maxLen {
		maxLen = len(onChain)
	}

	diffs := make([]DiagnosticDiff, maxLen)
	for i := 0; i < maxLen; i++ {
		var le, oe *simulator.DiagnosticEvent
		if i < len(local) {
			cp := local[i]
			le = &cp
		}
		if i < len(onChain) {
			cp := onChain[i]
			oe = &cp
		}

		dd := DiagnosticDiff{
			Index:   i,
			Local:   le,
			OnChain: oe,
		}

		if le == nil || oe == nil {
			dd.Divergent = true
			dd.DivergentPath = true
		} else {
			dd.Divergent = !diagnosticEventsEqual(*le, *oe)
			dd.DivergentPath = le.EventType != oe.EventType ||
				contractIDStr(le.ContractID) != contractIDStr(oe.ContractID)
		}

		diffs[i] = dd
	}
	return diffs
}

func diagnosticEventsEqual(a, b simulator.DiagnosticEvent) bool {
	if a.EventType != b.EventType {
		return false
	}
	if contractIDStr(a.ContractID) != contractIDStr(b.ContractID) {
		return false
	}
	if a.Data != b.Data {
		return false
	}
	if len(a.Topics) != len(b.Topics) {
		return false
	}
	for i := range a.Topics {
		if a.Topics[i] != b.Topics[i] {
			return false
		}
	}
	return true
}

func contractIDStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func compareBudget(local, onChain *simulator.BudgetUsage) *BudgetDiff {
	bd := &BudgetDiff{}
	if local != nil {
		bd.LocalCPU = local.CPUInstructions
		bd.LocalMem = local.MemoryBytes
		bd.LocalOps = local.OperationsCount
	}
	if onChain != nil {
		bd.OnChainCPU = onChain.CPUInstructions
		bd.OnChainMem = onChain.MemoryBytes
		bd.OnChainOps = onChain.OperationsCount
	}
	bd.CPUDelta = int64(bd.LocalCPU) - int64(bd.OnChainCPU)
	bd.MemoryDelta = int64(bd.LocalMem) - int64(bd.OnChainMem)
	bd.OpsDelta = bd.LocalOps - bd.OnChainOps
	return bd
}

func extractCallPathDivergences(diffs []DiagnosticDiff) []CallPathDivergence {
	var divergences []CallPathDivergence
	for _, d := range diffs {
		if !d.DivergentPath {
			continue
		}

		var reasons []string
		localSummary := "<absent>"
		onChainSummary := "<absent>"

		if d.Local != nil {
			localSummary = fmt.Sprintf("type=%s contract=%s", d.Local.EventType, contractIDStr(d.Local.ContractID))
		}
		if d.OnChain != nil {
			onChainSummary = fmt.Sprintf("type=%s contract=%s", d.OnChain.EventType, contractIDStr(d.OnChain.ContractID))
		}

		if d.Local == nil || d.OnChain == nil {
			reasons = append(reasons, "event present in one run only")
		} else {
			if d.Local.EventType != d.OnChain.EventType {
				reasons = append(reasons, fmt.Sprintf("event type: %q vs %q", d.Local.EventType, d.OnChain.EventType))
			}
			if contractIDStr(d.Local.ContractID) != contractIDStr(d.OnChain.ContractID) {
				reasons = append(reasons, fmt.Sprintf("contract ID: %q vs %q",
					contractIDStr(d.Local.ContractID), contractIDStr(d.OnChain.ContractID)))
			}
		}

		divergences = append(divergences, CallPathDivergence{
			EventIndex:     d.Index,
			Reason:         strings.Join(reasons, "; "),
			LocalSummary:   localSummary,
			OnChainSummary: onChainSummary,
		})
	}
	return divergences
}
