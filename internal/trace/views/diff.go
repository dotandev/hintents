// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package views provides terminal-based view components for the interactive
// trace viewer, including a side-by-side variable diff display.
package views

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dotandev/hintents/internal/visualizer"
)

// VarDiffCategory classifies a single variable change.
type VarDiffCategory int

const (
	// VarAdded means the variable exists only in the "after" state.
	VarAdded VarDiffCategory = iota
	// VarRemoved means the variable exists only in the "before" state.
	VarRemoved
	// VarModified means the variable exists in both states but its value differs.
	VarModified
	// VarUnchanged means the variable exists in both states with the same value.
	VarUnchanged
)

// VarDiffEntry describes a single variable change between two execution states.
type VarDiffEntry struct {
	// Name is the variable key (e.g. "balance", "counter").
	Name string
	// Before is the string representation of the value before the step.
	Before string
	// After is the string representation of the value after the step.
	After string
	// Category classifies the kind of change.
	Category VarDiffCategory
	// Section indicates which map the variable lives in: "HostState" or "Memory".
	Section string
}

// VarDiffResult holds the complete diff between two data maps.
type VarDiffResult struct {
	Entries   []VarDiffEntry
	Added     int
	Removed   int
	Modified  int
	Unchanged int
}

// ComputeVarDiff computes a side-by-side diff of HostState and Memory variables
// between two steps. Each parameter is a map of variable names to their values.
// The before and after states are typically consecutive steps in an execution trace.
func ComputeVarDiff(beforeHost, afterHost, beforeMem, afterMem map[string]interface{}) *VarDiffResult {
	result := &VarDiffResult{
		Entries: make([]VarDiffEntry, 0),
	}

	// ── Collect all unique keys across HostState ──────────────────────────
	hostKeys := make(map[string]struct{})
	for k := range beforeHost {
		hostKeys[k] = struct{}{}
	}
	for k := range afterHost {
		hostKeys[k] = struct{}{}
	}

	hostKeyList := make([]string, 0, len(hostKeys))
	for k := range hostKeys {
		hostKeyList = append(hostKeyList, k)
	}
	sort.Strings(hostKeyList)

	for _, k := range hostKeyList {
		bv, inBefore := beforeHost[k]
		av, inAfter := afterHost[k]

		entry := VarDiffEntry{
			Name:    k,
			Section: "HostState",
		}

		switch {
		case inBefore && inAfter:
			bStr := fmt.Sprintf("%v", bv)
			aStr := fmt.Sprintf("%v", av)
			if bStr == aStr {
				entry.Category = VarUnchanged
				result.Unchanged++
			} else {
				entry.Category = VarModified
				result.Modified++
			}
			entry.Before = bStr
			entry.After = aStr
		case inBefore && !inAfter:
			entry.Category = VarRemoved
			result.Removed++
			entry.Before = fmt.Sprintf("%v", bv)
			entry.After = "<removed>"
		case !inBefore && inAfter:
			entry.Category = VarAdded
			result.Added++
			entry.Before = "<absent>"
			entry.After = fmt.Sprintf("%v", av)
		}

		result.Entries = append(result.Entries, entry)
	}

	// ── Collect all unique keys across Memory ─────────────────────────────
	memKeys := make(map[string]struct{})
	for k := range beforeMem {
		memKeys[k] = struct{}{}
	}
	for k := range afterMem {
		memKeys[k] = struct{}{}
	}

	memKeyList := make([]string, 0, len(memKeys))
	for k := range memKeys {
		memKeyList = append(memKeyList, k)
	}
	sort.Strings(memKeyList)

	for _, k := range memKeyList {
		bv, inBefore := beforeMem[k]
		av, inAfter := afterMem[k]

		entry := VarDiffEntry{
			Name:    k,
			Section: "Memory",
		}

		switch {
		case inBefore && inAfter:
			bStr := fmt.Sprintf("%v", bv)
			aStr := fmt.Sprintf("%v", av)
			if bStr == aStr {
				entry.Category = VarUnchanged
				result.Unchanged++
			} else {
				entry.Category = VarModified
				result.Modified++
			}
			entry.Before = bStr
			entry.After = aStr
		case inBefore && !inAfter:
			entry.Category = VarRemoved
			result.Removed++
			entry.Before = fmt.Sprintf("%v", bv)
			entry.After = "<removed>"
		case !inBefore && inAfter:
			entry.Category = VarAdded
			result.Added++
			entry.Before = "<absent>"
			entry.After = fmt.Sprintf("%v", av)
		}

		result.Entries = append(result.Entries, entry)
	}

	return result
}

// RenderVarDiff writes a colored, side-by-side variable diff to w.
// The width parameter controls the total column count for the terminal layout.
func RenderVarDiff(w io.Writer, result *VarDiffResult, width int) {
	if width < 40 {
		width = 80
	}

	colW := (width - 6) / 2 // each value column gets roughly half the screen
	if colW < 20 {
		colW = 20
	}
	sep := " │ "

	// ── No changes? ────────────────────────────────────────────────────────
	if result.Added+result.Removed+result.Modified == 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, visualizer.Colorize("  No variable changes between steps.", "dim"))
		_, _ = fmt.Fprintln(w)
		return
	}

	// ── Header ─────────────────────────────────────────────────────────────
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "  ╔══════════════════════════════════════════════════════════════════════╗")
	_, _ = fmt.Fprintln(w, "  ║  VARIABLE DIFF  —  Before vs After Step                              ║")
	_, _ = fmt.Fprintln(w, "  ╚══════════════════════════════════════════════════════════════════════╝")
	_, _ = fmt.Fprintln(w)

	beforeH := visualizer.Colorize("BEFORE", "dim")
	afterH := visualizer.Colorize("AFTER", "dim")
	_, _ = fmt.Fprintf(w, "  %-4s  %-*s%s%-*s\n", "", colW, beforeH, sep, colW, afterH)

	segment := strings.Repeat("─", colW*2+3+6)
	_, _ = fmt.Fprintln(w, "  "+visualizer.Colorize(segment, "dim"))

	// ── Render each section ────────────────────────────────────────────────
	showHostState := false
	showMemory := false
	for _, e := range result.Entries {
		if e.Category != VarUnchanged {
			if e.Section == "HostState" {
				showHostState = true
			} else {
				showMemory = true
			}
		}
	}

	if showHostState {
		_, _ = fmt.Fprintln(w, "  "+visualizer.Colorize("Host State:", "cyan"))
		for _, e := range result.Entries {
			if e.Section != "HostState" || e.Category == VarUnchanged {
				continue
			}
			renderDiffEntry(w, e, colW, sep)
		}
	}

	if showMemory {
		if showHostState {
			_, _ = fmt.Fprintln(w) // blank line between sections
		}
		_, _ = fmt.Fprintln(w, "  "+visualizer.Colorize("Memory:", "magenta"))
		for _, e := range result.Entries {
			if e.Section != "Memory" || e.Category == VarUnchanged {
				continue
			}
			renderDiffEntry(w, e, colW, sep)
		}
	}

	// ── Footer summary ─────────────────────────────────────────────────────
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "  "+visualizer.Colorize(segment, "dim"))
	_, _ = fmt.Fprintf(w, "  %s  %s added  %s removed  %s modified\n\n",
		visualizer.Colorize("Summary:", "bold"),
		visualizer.Colorize(fmt.Sprintf("%d", result.Added), "green"),
		visualizer.Colorize(fmt.Sprintf("%d", result.Removed), "red"),
		visualizer.Colorize(fmt.Sprintf("%d", result.Modified), "yellow"),
	)
}

// renderDiffEntry writes a single variable change row.
func renderDiffEntry(w io.Writer, e VarDiffEntry, colW int, sepCol string) {
	markerSym, beforeColor, afterColor := entryStyle(e.Category)

	beforePadded := padOrTrunc(e.Before, colW)
	afterPadded := padOrTrunc(e.After, colW)

	_, _ = fmt.Fprintf(w, "  %s  %s\n",
		visualizer.Colorize(markerSym, entryColor(e.Category)),
		visualizer.Colorize(e.Name, "bold"),
	)
	_, _ = fmt.Fprintf(w, "       %s%s%s\n",
		visualizer.Colorize(beforePadded, beforeColor),
		sepCol,
		visualizer.Colorize(afterPadded, afterColor),
	)
}

// entryStyle returns the marker symbol and color names for before/after columns.
func entryStyle(kind VarDiffCategory) (marker, beforeColor, afterColor string) {
	switch kind {
	case VarAdded:
		return "+", "dim", "green"
	case VarRemoved:
		return "-", "red", "dim"
	case VarModified:
		return "~", "yellow", "green"
	default:
		return "=", "dim", "dim"
	}
}

// entryColor returns the ANSI color name for the change marker.
func entryColor(kind VarDiffCategory) string {
	switch kind {
	case VarAdded:
		return "green"
	case VarRemoved:
		return "red"
	case VarModified:
		return "yellow"
	default:
		return "dim"
	}
}

// padOrTrunc ensures s is exactly n columns wide: pads on the right if
// shorter, or truncates with "…" if longer.
func padOrTrunc(s string, n int) string {
	if len(s) == n {
		return s
	}
	if len(s) < n {
		return s + strings.Repeat(" ", n-len(s))
	}
	// Truncate, preserving at least 3 characters.
	if n <= 3 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
