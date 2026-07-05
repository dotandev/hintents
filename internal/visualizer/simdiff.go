// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package visualizer provides terminal rendering helpers for simulation output.
// This file implements colored before/after ledger state diff rendering.
package visualizer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const (
	ledgerColWidth = 56 // width of each value column in the ledger diff table
	ledgerColSep   = " │ "
)

// LedgerDiffEntry represents a single ledger entry change.
type LedgerDiffEntry struct {
	Key        string
	Before     string // empty string means the entry did not exist before
	After      string // empty string means the entry was removed
	ChangeKind ledgerChangeKind
}

type ledgerChangeKind int

const (
	ledgerAdded     ledgerChangeKind = iota // entry exists only in After
	ledgerRemoved                           // entry exists only in Before
	ledgerModified                          // entry exists in both but values differ
	ledgerUnchanged                         // entry exists in both with same value
)

// DiffLedgerEntries computes the diff between two ledger entry maps.
// Keys are base64-encoded XDR ledger keys; values are base64-encoded XDR ledger entries.
func DiffLedgerEntries(before, after map[string]string) []LedgerDiffEntry {
	allKeys := make(map[string]struct{})
	for k := range before {
		allKeys[k] = struct{}{}
	}
	for k := range after {
		allKeys[k] = struct{}{}
	}

	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	entries := make([]LedgerDiffEntry, 0, len(keys))
	for _, k := range keys {
		bv, inBefore := before[k]
		av, inAfter := after[k]

		var kind ledgerChangeKind
		switch {
		case inBefore && inAfter && bv != av:
			kind = ledgerModified
		case inBefore && !inAfter:
			kind = ledgerRemoved
		case !inBefore && inAfter:
			kind = ledgerAdded
		default:
			kind = ledgerUnchanged
		}

		entries = append(entries, LedgerDiffEntry{
			Key:        k,
			Before:     bv,
			After:      av,
			ChangeKind: kind,
		})
	}
	return entries
}

// summarizeLedgerDiff counts the change kinds across a set of diff entries.
func summarizeLedgerDiff(entries []LedgerDiffEntry) (added, removed, modified, unchanged int) {
	for _, e := range entries {
		switch e.ChangeKind {
		case ledgerAdded:
			added++
		case ledgerRemoved:
			removed++
		case ledgerModified:
			modified++
		case ledgerUnchanged:
			unchanged++
		}
	}
	return added, removed, modified, unchanged
}

// RenderLedgerStateDiff prints a colored before/after diff of ledger state changes
// to stdout. Unchanged entries are omitted unless showUnchanged is true.
//
// It is a thin convenience wrapper around RenderLedgerStateDiffTo; callers that
// need to capture the output (tests, headless integrations) should target an
// explicit io.Writer instead.
func RenderLedgerStateDiff(before, after map[string]string, showUnchanged bool) {
	RenderLedgerStateDiffTo(os.Stdout, before, after, showUnchanged)
}

// RenderLedgerStateDiffTo writes a colored before/after diff of ledger state
// changes to w. Unchanged entries are omitted unless showUnchanged is true.
// The formatting is identical to RenderLedgerStateDiff; only the IO target
// differs, which keeps the rendering logic decoupled from stdout.
func RenderLedgerStateDiffTo(w io.Writer, before, after map[string]string, showUnchanged bool) {
	entries := DiffLedgerEntries(before, after)
	added, removed, modified, unchanged := summarizeLedgerDiff(entries)

	printLedgerDiffHeader(w, len(before), len(after), added, removed, modified)

	if added+removed+modified == 0 {
		fmt.Fprintf(w, "\n  %s Ledger state is identical — no entries changed.\n\n",
			Colorize("[=]", "dim"))
		return
	}

	// Column headers — pad manually to avoid ANSI escape codes skewing %-*s width.
	fmt.Fprintln(w)
	beforeHeader := Colorize("BEFORE", "dim") + strings.Repeat(" ", ledgerColWidth-len("BEFORE"))
	afterHeader := Colorize("AFTER", "dim") + strings.Repeat(" ", ledgerColWidth-len("AFTER"))
	fmt.Fprintf(w, "  %-4s  %s%s%s\n", "", beforeHeader, ledgerColSep, afterHeader)
	fmt.Fprintf(w, "  %s\n", Colorize(strings.Repeat("─", 4+2+ledgerColWidth*2+len(ledgerColSep)), "dim"))

	for _, e := range entries {
		if e.ChangeKind == ledgerUnchanged && !showUnchanged {
			continue
		}
		renderLedgerEntry(w, e)
	}

	fmt.Fprintln(w)
	printLedgerDiffSummary(w, added, removed, modified, unchanged)
}

// renderLedgerEntry writes a single ledger entry diff row to w.
func renderLedgerEntry(w io.Writer, e LedgerDiffEntry) {
	marker, beforeColor, afterColor := ledgerEntryStyle(e.ChangeKind)

	// Truncate long base64 values for readability; full values are in the raw XDR.
	beforeRaw := truncateLedgerValue(e.Before, ledgerColWidth)
	afterRaw := truncateLedgerValue(e.After, ledgerColWidth)

	// Pad the raw (uncolored) strings to ledgerColWidth before colorizing,
	// so ANSI escape codes don't skew column alignment.
	beforePadded := padRight(beforeRaw, ledgerColWidth)
	afterPadded := padRight(afterRaw, ledgerColWidth)

	// Shorten the key for display.
	keyDisplay := shortenLedgerKey(e.Key)

	fmt.Fprintf(w, "  %s   %s\n",
		Colorize(marker, ledgerChangeColor(e.ChangeKind)),
		Colorize(keyDisplay, "dim"),
	)
	fmt.Fprintf(w, "       %s%s%s\n",
		Colorize(beforePadded, beforeColor),
		ledgerColSep,
		Colorize(afterPadded, afterColor),
	)
}

// padRight pads s with spaces on the right to reach width w.
// If s is already longer than w, it is returned unchanged.
func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

// ledgerEntryStyle returns the marker symbol and color names for before/after columns.
func ledgerEntryStyle(kind ledgerChangeKind) (marker, beforeColor, afterColor string) {
	switch kind {
	case ledgerAdded:
		return "+", "dim", "green"
	case ledgerRemoved:
		return "-", "red", "dim"
	case ledgerModified:
		return "~", "yellow", "green"
	default:
		return "=", "dim", "dim"
	}
}

// ledgerChangeColor returns the ANSI color name for the change marker.
func ledgerChangeColor(kind ledgerChangeKind) string {
	switch kind {
	case ledgerAdded:
		return "green"
	case ledgerRemoved:
		return "red"
	case ledgerModified:
		return "yellow"
	default:
		return "dim"
	}
}

// truncateLedgerValue shortens a base64 XDR value for display.
// Returns "<none>" for empty values (entry absent or deleted).
// Shows the first maxLen-1 characters followed by "…" if truncated.
func truncateLedgerValue(val string, maxLen int) string {
	if val == "" {
		return "<none>"
	}
	if len(val) <= maxLen {
		return val
	}
	return val[:maxLen-1] + "…"
}

// shortenLedgerKey returns a compact display form of a base64 XDR ledger key.
// Shows the first 8 and last 8 characters separated by "…" for keys longer than 20 chars.
func shortenLedgerKey(key string) string {
	const maxKeyDisplay = 40
	if len(key) <= maxKeyDisplay {
		return key
	}
	return key[:8] + "…" + key[len(key)-8:]
}

// printLedgerDiffHeader writes the section header for the ledger state diff to w.
func printLedgerDiffHeader(w io.Writer, beforeCount, afterCount, added, removed, modified int) {
	sep := strings.Repeat("═", ledgerColWidth*2+len(ledgerColSep)+8)
	fmt.Fprintln(w)
	fmt.Fprintln(w, Colorize("╔"+sep+"╗", "cyan"))
	title := "  LEDGER STATE DIFF  ─  Before vs After Transaction  "
	pad := len(sep) - len(title)
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(w, Colorize("║", "cyan")+"%s"+strings.Repeat(" ", pad)+Colorize("║", "cyan")+"\n", title)
	fmt.Fprintln(w, Colorize("╚"+sep+"╝", "cyan"))

	fmt.Fprintf(w, "\n  Entries before: %s   after: %s\n",
		Colorize(fmt.Sprintf("%d", beforeCount), "dim"),
		Colorize(fmt.Sprintf("%d", afterCount), "dim"),
	)
	fmt.Fprintf(w, "  Changes: %s added  %s removed  %s modified\n",
		Colorize(fmt.Sprintf("+%d", added), "green"),
		Colorize(fmt.Sprintf("-%d", removed), "red"),
		Colorize(fmt.Sprintf("~%d", modified), "yellow"),
	)
}

// printLedgerDiffSummary writes the summary footer for the ledger state diff to w.
func printLedgerDiffSummary(w io.Writer, added, removed, modified, unchanged int) {
	sep := strings.Repeat("─", ledgerColWidth*2+len(ledgerColSep)+8)
	fmt.Fprintln(w, Colorize("  "+sep, "dim"))
	fmt.Fprintf(w, "  %s  %s added  %s removed  %s modified  %s unchanged\n\n",
		Colorize("Summary:", "bold"),
		Colorize(fmt.Sprintf("%d", added), "green"),
		Colorize(fmt.Sprintf("%d", removed), "red"),
		Colorize(fmt.Sprintf("%d", modified), "yellow"),
		Colorize(fmt.Sprintf("%d", unchanged), "dim"),
	)
}

// LedgerDiffChange is the JSON-friendly name of a ledger entry's change kind.
type LedgerDiffChange string

const (
	ChangeAdded     LedgerDiffChange = "added"
	ChangeRemoved   LedgerDiffChange = "removed"
	ChangeModified  LedgerDiffChange = "modified"
	ChangeUnchanged LedgerDiffChange = "unchanged"
)

// LedgerDiffEntryReport is a single ledger entry change in structured,
// machine-readable form suitable for headless (non-terminal) integration.
// Before is nil when the entry did not exist before the transaction; After is
// nil when the entry was removed. Both are populated for modified/unchanged
// entries.
type LedgerDiffEntryReport struct {
	Key    string           `json:"key"`
	Change LedgerDiffChange `json:"change"`
	Before *string          `json:"before"`
	After  *string          `json:"after"`
}

// LedgerStateDiffReport is the structured form of a ledger state diff. It carries
// the same information RenderLedgerStateDiff prints, minus the ANSI colors and
// terminal layout, so it can be serialized to JSON and consumed by other tools.
type LedgerStateDiffReport struct {
	BeforeCount int                     `json:"before_count"`
	AfterCount  int                     `json:"after_count"`
	Added       int                     `json:"added"`
	Removed     int                     `json:"removed"`
	Modified    int                     `json:"modified"`
	Unchanged   int                     `json:"unchanged"`
	Entries     []LedgerDiffEntryReport `json:"entries"`
}

// ledgerChangeName maps an internal change kind to its JSON-friendly name.
func ledgerChangeName(kind ledgerChangeKind) LedgerDiffChange {
	switch kind {
	case ledgerAdded:
		return ChangeAdded
	case ledgerRemoved:
		return ChangeRemoved
	case ledgerModified:
		return ChangeModified
	default:
		return ChangeUnchanged
	}
}

// BuildLedgerStateDiffReport computes a structured, JSON-serializable diff of two
// ledger entry maps. Unchanged entries are included in Entries only when
// includeUnchanged is true, mirroring the showUnchanged behavior of
// RenderLedgerStateDiff; the summary counters always reflect the full diff.
func BuildLedgerStateDiffReport(before, after map[string]string, includeUnchanged bool) LedgerStateDiffReport {
	entries := DiffLedgerEntries(before, after)
	added, removed, modified, unchanged := summarizeLedgerDiff(entries)

	report := LedgerStateDiffReport{
		BeforeCount: len(before),
		AfterCount:  len(after),
		Added:       added,
		Removed:     removed,
		Modified:    modified,
		Unchanged:   unchanged,
		Entries:     make([]LedgerDiffEntryReport, 0, len(entries)),
	}

	for _, e := range entries {
		if e.ChangeKind == ledgerUnchanged && !includeUnchanged {
			continue
		}

		item := LedgerDiffEntryReport{
			Key:    e.Key,
			Change: ledgerChangeName(e.ChangeKind),
		}

		// Populate before/after pointers based on the change kind so an absent
		// side is encoded as JSON null rather than an empty string.
		switch e.ChangeKind {
		case ledgerAdded:
			av := e.After
			item.After = &av
		case ledgerRemoved:
			bv := e.Before
			item.Before = &bv
		default: // modified or unchanged: present on both sides
			bv, av := e.Before, e.After
			item.Before = &bv
			item.After = &av
		}

		report.Entries = append(report.Entries, item)
	}

	return report
}

// MarshalLedgerStateDiffJSON returns the indented JSON encoding of the structured
// ledger state diff between two ledger entry maps.
func MarshalLedgerStateDiffJSON(before, after map[string]string, includeUnchanged bool) ([]byte, error) {
	return json.MarshalIndent(BuildLedgerStateDiffReport(before, after, includeUnchanged), "", "  ")
}

// RenderLedgerStateDiffJSON writes the structured ledger state diff as indented
// JSON (followed by a trailing newline) to w. It is the headless counterpart of
// RenderLedgerStateDiff, intended for machine consumption rather than a terminal.
func RenderLedgerStateDiffJSON(w io.Writer, before, after map[string]string, includeUnchanged bool) error {
	data, err := MarshalLedgerStateDiffJSON(before, after, includeUnchanged)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}
