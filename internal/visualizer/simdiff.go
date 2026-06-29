// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package visualizer provides terminal rendering helpers for simulation output.
// This file implements colored before/after ledger state diff rendering.
package visualizer

import (
	"bytes"
	"encoding/base64"
	"fmt"
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
	sortLedgerKeys(keys)

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

// sortLedgerKeys sorts base64-encoded XDR ledger keys by the byte order of their
// decoded binary values, rather than the lexicographic order of the base64 text.
// This matches the canonical ordering of the underlying ledger keys. Keys that
// fail to decode fall back to comparison of their raw base64 string, and decoded
// keys always sort before undecodable ones to keep the ordering deterministic.
func sortLedgerKeys(keys []string) {
	decoded := make(map[string][]byte, len(keys))
	for _, k := range keys {
		if b, err := base64.StdEncoding.DecodeString(k); err == nil {
			decoded[k] = b
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		bi, oki := decoded[keys[i]]
		bj, okj := decoded[keys[j]]
		switch {
		case oki && okj:
			return bytes.Compare(bi, bj) < 0
		case oki != okj:
			return oki // decodable keys sort before undecodable ones
		default:
			return keys[i] < keys[j]
		}
	})
}

// RenderLedgerStateDiff prints a colored before/after diff of ledger state changes
// to stdout. Unchanged entries are omitted unless showUnchanged is true.
func RenderLedgerStateDiff(before, after map[string]string, showUnchanged bool) {
	entries := DiffLedgerEntries(before, after)

	added, removed, modified, unchanged := 0, 0, 0, 0
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

	printLedgerDiffHeader(len(before), len(after), added, removed, modified)

	if added+removed+modified == 0 {
		fmt.Printf("\n  %s Ledger state is identical — no entries changed.\n\n",
			Colorize("[=]", "dim"))
		return
	}

	// Column headers — pad manually to avoid ANSI escape codes skewing %-*s width.
	fmt.Println()
	beforeHeader := Colorize("BEFORE", "dim") + strings.Repeat(" ", ledgerColWidth-len("BEFORE"))
	afterHeader := Colorize("AFTER", "dim") + strings.Repeat(" ", ledgerColWidth-len("AFTER"))
	fmt.Printf("  %-4s  %s%s%s\n", "", beforeHeader, ledgerColSep, afterHeader)
	fmt.Printf("  %s\n", Colorize(strings.Repeat("─", 4+2+ledgerColWidth*2+len(ledgerColSep)), "dim"))

	for _, e := range entries {
		if e.ChangeKind == ledgerUnchanged && !showUnchanged {
			continue
		}
		renderLedgerEntry(e)
	}

	fmt.Println()
	printLedgerDiffSummary(added, removed, modified, unchanged)
}

// renderLedgerEntry prints a single ledger entry diff row.
func renderLedgerEntry(e LedgerDiffEntry) {
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

	fmt.Printf("  %s   %s\n",
		Colorize(marker, ledgerChangeColor(e.ChangeKind)),
		Colorize(keyDisplay, "dim"),
	)
	fmt.Printf("       %s%s%s\n",
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

// printLedgerDiffHeader prints the section header for the ledger state diff.
func printLedgerDiffHeader(beforeCount, afterCount, added, removed, modified int) {
	sep := strings.Repeat("═", ledgerColWidth*2+len(ledgerColSep)+8)
	fmt.Println()
	fmt.Println(Colorize("╔"+sep+"╗", "cyan"))
	title := "  LEDGER STATE DIFF  ─  Before vs After Transaction  "
	pad := len(sep) - len(title)
	if pad < 0 {
		pad = 0
	}
	fmt.Printf(Colorize("║", "cyan")+"%s"+strings.Repeat(" ", pad)+Colorize("║", "cyan")+"\n", title)
	fmt.Println(Colorize("╚"+sep+"╝", "cyan"))

	fmt.Printf("\n  Entries before: %s   after: %s\n",
		Colorize(fmt.Sprintf("%d", beforeCount), "dim"),
		Colorize(fmt.Sprintf("%d", afterCount), "dim"),
	)
	fmt.Printf("  Changes: %s added  %s removed  %s modified\n",
		Colorize(fmt.Sprintf("+%d", added), "green"),
		Colorize(fmt.Sprintf("-%d", removed), "red"),
		Colorize(fmt.Sprintf("~%d", modified), "yellow"),
	)
}

// printLedgerDiffSummary prints the summary footer for the ledger state diff.
func printLedgerDiffSummary(added, removed, modified, unchanged int) {
	sep := strings.Repeat("─", ledgerColWidth*2+len(ledgerColSep)+8)
	fmt.Println(Colorize("  "+sep, "dim"))
	fmt.Printf("  %s  %s added  %s removed  %s modified  %s unchanged\n\n",
		Colorize("Summary:", "bold"),
		Colorize(fmt.Sprintf("%d", added), "green"),
		Colorize(fmt.Sprintf("%d", removed), "red"),
		Colorize(fmt.Sprintf("%d", modified), "yellow"),
		Colorize(fmt.Sprintf("%d", unchanged), "dim"),
	)
}
