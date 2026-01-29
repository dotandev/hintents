// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package compare

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
)

// Diff represents differences between two simulation results
type Diff struct {
	StatusChanged bool
	ErrorChanged  bool
	EventsDiff    []EventDiff
	LogsDiff      []LogDiff
	Summary       string
}

// EventDiff represents a difference in events
type EventDiff struct {
	Index   int
	OnChain string
	Local   string
	Type    string // "added", "removed", "modified", "unchanged"
}

// LogDiff represents a difference in logs
type LogDiff struct {
	Index   int
	OnChain string
	Local   string
	Type    string // "added", "removed", "modified", "unchanged"
}

// CompareResults compares two SimulationResponse objects and returns a Diff
func CompareResults(onChain, local *simulator.SimulationResponse) *Diff {
	diff := &Diff{
		StatusChanged: onChain.Status != local.Status,
		ErrorChanged:  onChain.Error != local.Error,
		EventsDiff:    compareEvents(onChain.Events, local.Events),
		LogsDiff:      compareLogs(onChain.Logs, local.Logs),
	}

	diff.Summary = buildSummary(diff)
	return diff
}

func compareEvents(onChain, local []string) []EventDiff {
	var diffs []EventDiff
	maxLen := len(onChain)
	if len(local) > maxLen {
		maxLen = len(local)
	}

	for i := 0; i < maxLen; i++ {
		var ed EventDiff
		ed.Index = i

		if i < len(onChain) && i < len(local) {
			ed.OnChain = onChain[i]
			ed.Local = local[i]
			if onChain[i] == local[i] {
				ed.Type = "unchanged"
			} else {
				ed.Type = "modified"
			}
		} else if i < len(onChain) {
			ed.OnChain = onChain[i]
			ed.Local = ""
			ed.Type = "removed"
		} else {
			ed.OnChain = ""
			ed.Local = local[i]
			ed.Type = "added"
		}

		diffs = append(diffs, ed)
	}

	return diffs
}

func compareLogs(onChain, local []string) []LogDiff {
	var diffs []LogDiff
	maxLen := len(onChain)
	if len(local) > maxLen {
		maxLen = len(local)
	}

	for i := 0; i < maxLen; i++ {
		var ld LogDiff
		ld.Index = i

		if i < len(onChain) && i < len(local) {
			ld.OnChain = onChain[i]
			ld.Local = local[i]
			if onChain[i] == local[i] {
				ld.Type = "unchanged"
			} else {
				ld.Type = "modified"
			}
		} else if i < len(onChain) {
			ld.OnChain = onChain[i]
			ld.Local = ""
			ld.Type = "removed"
		} else {
			ld.OnChain = ""
			ld.Local = local[i]
			ld.Type = "added"
		}

		diffs = append(diffs, ld)
	}

	return diffs
}

func buildSummary(diff *Diff) string {
	var parts []string

	if diff.StatusChanged {
		parts = append(parts, "status changed")
	}
	if diff.ErrorChanged {
		parts = append(parts, "error changed")
	}

	eventChanges := countChanges(diff.EventsDiff)
	if eventChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d event(s) differ", eventChanges))
	}

	logChanges := countChangesLogs(diff.LogsDiff)
	if logChanges > 0 {
		parts = append(parts, fmt.Sprintf("%d log(s) differ", logChanges))
	}

	if len(parts) == 0 {
		return "No differences found"
	}

	return strings.Join(parts, ", ")
}

func countChanges(diffs []EventDiff) int {
	count := 0
	for _, d := range diffs {
		if d.Type != "unchanged" {
			count++
		}
	}
	return count
}

func countChangesLogs(diffs []LogDiff) int {
	count := 0
	for _, d := range diffs {
		if d.Type != "unchanged" {
			count++
		}
	}
	return count
}

// FormatSideBySide formats the diff as a side-by-side comparison
func (d *Diff) FormatSideBySide() string {
	var b strings.Builder

	b.WriteString("=== Comparison Summary ===\n")
	b.WriteString(d.Summary + "\n\n")

	if len(d.EventsDiff) > 0 {
		b.WriteString("=== Events ===\n")
		for _, ed := range d.EventsDiff {
			if ed.Type == "unchanged" {
				continue // skip unchanged for brevity
			}
			b.WriteString(fmt.Sprintf("[%d] %s\n", ed.Index, ed.Type))
			if ed.OnChain != "" {
				b.WriteString(fmt.Sprintf("  On-Chain: %s\n", ed.OnChain))
			}
			if ed.Local != "" {
				b.WriteString(fmt.Sprintf("  Local:    %s\n", ed.Local))
			}
			b.WriteString("\n")
		}
	}

	if len(d.LogsDiff) > 0 {
		b.WriteString("=== Logs ===\n")
		for _, ld := range d.LogsDiff {
			if ld.Type == "unchanged" {
				continue
			}
			b.WriteString(fmt.Sprintf("[%d] %s\n", ld.Index, ld.Type))
			if ld.OnChain != "" {
				b.WriteString(fmt.Sprintf("  On-Chain: %s\n", ld.OnChain))
			}
			if ld.Local != "" {
				b.WriteString(fmt.Sprintf("  Local:    %s\n", ld.Local))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}
