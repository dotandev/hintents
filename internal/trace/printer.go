// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

// ─────────────────────────────────────────────────────────────────────────────
// Options
// ─────────────────────────────────────────────────────────────────────────────

// PrintOptions controls the appearance of the printed trace tree.
type PrintOptions struct {
	// NoColor disables ANSI colour output. The NO_COLOR environment variable
	// and any piped (non-tty) output are also honoured automatically by the
	// underlying colour library; this flag makes it explicit.
	NoColor bool

	// MaxWidth sets the maximum column width for wrapped fields.
	// 0 means auto-detect from the terminal (falls back to 80).
	MaxWidth int

	// Output is the writer to send the report to. nil defaults to os.Stdout.
	Output io.Writer
}

func (o PrintOptions) writer() io.Writer {
	if o.Output != nil {
		return o.Output
	}
	return os.Stdout
}

func (o PrintOptions) maxWidth() int {
	if o.MaxWidth > 0 {
		return o.MaxWidth
	}
	w := getTermWidth()
	if w < 40 {
		return 40
	}
	return w
}

// ─────────────────────────────────────────────────────────────────────────────
// Color palette  (re-created on every call so NoColor is respected at runtime)
// ─────────────────────────────────────────────────────────────────────────────

type palette struct {
	header      *color.Color
	separator   *color.Color
	txRoot      *color.Color
	contractFn  *color.Color
	hostFn      *color.Color
	authFn      *color.Color
	eventFn     *color.Color
	errorFn     *color.Color
	logFn       *color.Color
	trapFn      *color.Color
	otherFn     *color.Color
	dimmed      *color.Color
	returnVal   *color.Color
	errorMsg    *color.Color
	budgetLabel *color.Color
	budgetVal   *color.Color
	stepNum     *color.Color
	opLabel     *color.Color
	contractID  *color.Color
	summaryKey  *color.Color
	summaryVal  *color.Color
}

func newColor(noColor bool, attrs ...color.Attribute) *color.Color {
	c := color.New(attrs...)
	if noColor {
		c.DisableColor()
	}
	return c
}

func newPalette(noColor bool) palette {
	// Create a "no-op" colour (plain text), still respecting the noColor flag.
	nop := newColor(noColor) // no attributes → plain text
	return palette{
		header:      newColor(noColor, color.FgWhite, color.Bold),
		separator:   newColor(noColor, color.FgHiBlack),
		txRoot:      newColor(noColor, color.FgWhite, color.Bold),
		contractFn:  newColor(noColor, color.FgCyan, color.Bold),
		hostFn:      newColor(noColor, color.FgGreen),
		authFn:      newColor(noColor, color.FgMagenta),
		eventFn:     newColor(noColor, color.FgYellow),
		errorFn:     newColor(noColor, color.FgRed, color.Bold),
		logFn:       newColor(noColor, color.FgBlue),
		trapFn:      newColor(noColor, color.FgRed, color.Bold, color.Underline),
		otherFn:     nop,
		dimmed:      newColor(noColor, color.FgHiBlack),
		returnVal:   newColor(noColor, color.FgGreen),
		errorMsg:    newColor(noColor, color.FgRed),
		budgetLabel: newColor(noColor, color.FgHiBlack),
		budgetVal:   newColor(noColor, color.FgHiWhite),
		stepNum:     newColor(noColor, color.FgHiBlack),
		opLabel:     newColor(noColor, color.FgHiWhite),
		contractID:  newColor(noColor, color.FgHiBlack),
		summaryKey:  newColor(noColor, color.FgHiBlack),
		summaryVal:  newColor(noColor, color.FgWhite, color.Bold),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Public API
// ─────────────────────────────────────────────────────────────────────────────

// PrintExecutionTrace renders a rich, colour-coded ASCII execution-trace tree
// to the writer specified in opts (default: os.Stdout).
//
// The trace is presented as a flat sequence under a root "transaction" node
// because ExecutionTrace states do not carry an explicit depth / parent field.
// Each state is colour-coded by its inferred event type, return values are
// highlighted in green and errors in red.
//
// Supports --no-color and responds to terminal width.
func PrintExecutionTrace(t *ExecutionTrace, opts PrintOptions) {
	p := newPalette(opts.NoColor)
	out := opts.writer()
	maxW := opts.maxWidth()

	sep := strings.Repeat("─", maxW)

	// ── header ───────────────────────────────────────────────────────────────
	_, _ = fmt.Fprintln(out)
	_, _ = p.header.Fprintln(out, " Transaction Execution Trace")
	_, _ = fmt.Fprintf(out, " Hash  : %s\n", truncateHash(t.TransactionHash, maxW-10))
	if !t.StartTime.IsZero() {
		_, _ = fmt.Fprintf(out, " Start : %s\n", t.StartTime.UTC().Format(time.RFC3339))
	}
	_, _ = fmt.Fprintf(out, " Steps : %d\n", len(t.States))
	_, _ = p.separator.Fprintln(out, sep)

	// ── root node ────────────────────────────────────────────────────────────
	_, _ = p.txRoot.Fprintf(out, "▸ TX  %s\n", truncateHash(t.TransactionHash, maxW-6))

	// ── state nodes ──────────────────────────────────────────────────────────
	total := len(t.States)
	errorCount := 0
	var finalStatus string

	for i, state := range t.States {
		isLast := i == total-1
		connector, continuation := treeConnectors(isLast)

		evtType := ClassifyEventType(&state)
		opColor := p.colorForType(evtType)
		// Use operation-specific icon when available, fall back to event type icon.
		icon := iconForType(state.Operation)
		if icon == "·" {
			icon = iconForType(evtType)
		}

		// ── first line: step + operation + function + contract ───────────────
		opName := strings.ToUpper(state.Operation)
		if opName == "" {
			opName = strings.ToUpper(evtType)
		}

		funcPart := ""
		if state.Function != "" {
			funcPart = "  " + opColor.Sprint(state.Function)
		}

		contractPart := ""
		if state.ContractID != "" {
			contractPart = "  " + p.contractID.Sprint(shortID(state.ContractID, maxW))
		}

		returnPart := ""
		if state.ReturnValue != nil && state.Error == "" {
			rv := fmt.Sprintf("%v", state.ReturnValue)
			if len(rv) > 40 {
				rv = rv[:37] + "…"
			}
			returnPart = "  → " + p.returnVal.Sprint(rv)
		}

		_, _ = fmt.Fprintf(out, "%s%s %s%s%s%s%s\n",
			connector,
			p.stepNum.Sprintf("[%d]", state.Step),
			p.opLabel.Sprint(icon+" "+opName),
			funcPart,
			contractPart,
			returnPart,
			"",
		)

		// ── second line: error (if any) ───────────────────────────────────────
		if state.Error != "" {
			errorCount++
			errLine := wrapText(state.Error, maxW-len(continuation)-6)
			for j, line := range strings.Split(errLine, "\n") {
				if j == 0 {
					_, _ = fmt.Fprintf(out, "%s  %s %s\n",
						continuation,
						p.errorFn.Sprint("[FAIL]"),
						p.errorMsg.Sprint(line),
					)
				} else {
					_, _ = fmt.Fprintf(out, "%s    %s\n", continuation, p.errorMsg.Sprint(line))
				}
			}
		}

		// ── detect final status from host_state ───────────────────────────────
		if state.HostState != nil {
			if v, ok := state.HostState["status"]; ok {
				finalStatus = fmt.Sprintf("%v", v)
			}
		}
	}

	// ── footer ───────────────────────────────────────────────────────────────
	_, _ = p.separator.Fprintln(out, sep)
	printSummaryLine(out, p, total, errorCount, finalStatus)
	_, _ = fmt.Fprintln(out)
}

// PrintTraceTree renders a rich, colour-coded ASCII tree from a TraceNode
// hierarchy to the writer specified in opts (default: os.Stdout).
//
// This function is suitable for traces built from ParseSimulationResponse or
// CreateMockTrace where parent/child relationships are already established.
func PrintTraceTree(root *TraceNode, opts PrintOptions) {
	if root == nil {
		return
	}
	p := newPalette(opts.NoColor)
	out := opts.writer()
	maxW := opts.maxWidth()

	sep := strings.Repeat("─", maxW)

	// ── header ───────────────────────────────────────────────────────────────
	_, _ = fmt.Fprintln(out)
	_, _ = p.header.Fprintln(out, " Transaction Execution Trace")
	_, _ = p.separator.Fprintln(out, sep)

	// ── recursive tree print ─────────────────────────────────────────────────
	stats := &treeStats{}
	printTreeNode(out, p, root, "", true, maxW, stats)

	// ── footer ───────────────────────────────────────────────────────────────
	_, _ = p.separator.Fprintln(out, sep)
	printSummaryLine(out, p, stats.total, stats.errors, "")
	_, _ = fmt.Fprintln(out)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tree rendering helpers
// ─────────────────────────────────────────────────────────────────────────────

type treeStats struct {
	total  int
	errors int
}

func printTreeNode(out io.Writer, p palette, node *TraceNode, prefix string, isLast bool, maxW int, stats *treeStats) {
	if node == nil {
		return
	}
	stats.total++
	if node.Error != "" {
		stats.errors++
	}

	connector, childPrefix := prefixParts(prefix, isLast)
	evtType := node.Type
	opColor := p.colorForType(evtType)
	icon := iconForType(evtType)

	// ── node label ────────────────────────────────────────────────────────────
	var sb strings.Builder

	// expand indicator
	if !node.IsLeaf() {
		if node.Expanded {
			sb.WriteString("▾ ")
		} else {
			sb.WriteString("▸ ")
		}
	} else {
		sb.WriteString("  ")
	}

	// function / event data / type label
	label := ""
	switch {
	case node.Function != "":
		label = opColor.Sprint(icon + " " + node.Function)
	case node.EventData != "":
		label = opColor.Sprint(icon + " " + node.EventData)
	default:
		label = opColor.Sprint(icon + " [" + node.Type + "]")
	}
	sb.WriteString(label)

	// contract ID
	if node.ContractID != "" {
		sb.WriteString("  ")
		sb.WriteString(p.contractID.Sprint(shortID(node.ContractID, maxW)))
	}

	// event data when function is also set
	if node.Function != "" && node.EventData != "" && node.Type != "log" {
		ev := node.EventData
		if len(ev) > 40 {
			ev = ev[:37] + "…"
		}
		sb.WriteString("  ")
		sb.WriteString(p.returnVal.Sprint("→ " + ev))
	}

	_, _ = fmt.Fprintf(out, "%s%s%s\n", prefix, connector, sb.String())

	// ── error sub-line ────────────────────────────────────────────────────────
	if node.Error != "" {
		errLine := wrapText(node.Error, maxW-len(childPrefix)-6)
		for j, line := range strings.Split(errLine, "\n") {
			if j == 0 {
				_, _ = fmt.Fprintf(out, "%s  %s %s\n",
					childPrefix,
					p.errorFn.Sprint("[FAIL]"),
					p.errorMsg.Sprint(line),
				)
			} else {
				_, _ = fmt.Fprintf(out, "%s    %s\n", childPrefix, p.errorMsg.Sprint(line))
			}
		}
	}

	// ── budget metrics ────────────────────────────────────────────────────────
	if node.CPUDelta != nil || node.MemoryDelta != nil {
		var budget strings.Builder
		budget.WriteString(childPrefix)
		budget.WriteString("  ")
		if node.CPUDelta != nil {
			budget.WriteString(p.budgetLabel.Sprint("CPU: "))
			budget.WriteString(p.budgetVal.Sprint(formatNum(*node.CPUDelta)))
		}
		if node.CPUDelta != nil && node.MemoryDelta != nil {
			budget.WriteString(p.dimmed.Sprint("  "))
		}
		if node.MemoryDelta != nil {
			budget.WriteString(p.budgetLabel.Sprint("MEM: "))
			budget.WriteString(p.budgetVal.Sprint(formatBytes(*node.MemoryDelta)))
		}
		_, _ = fmt.Fprintln(out, budget.String())
	}

	// ── children ──────────────────────────────────────────────────────────────
	if node.Expanded {
		for i, child := range node.Children {
			childIsLast := i == len(node.Children)-1
			printTreeNode(out, p, child, childPrefix, childIsLast, maxW, stats)
		}
	} else if len(node.Children) > 0 {
		// collapsed indicator
		_, _ = fmt.Fprintf(out, "%s  %s\n",
			childPrefix,
			p.dimmed.Sprintf("… %d children collapsed", len(node.Children)),
		)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Summary line
// ─────────────────────────────────────────────────────────────────────────────

func printSummaryLine(out io.Writer, p palette, total, errorCount int, status string) {
	var parts []string
	parts = append(parts, p.summaryKey.Sprint("Steps: ")+p.summaryVal.Sprint(fmt.Sprintf("%d", total)))
	if errorCount > 0 {
		parts = append(parts, p.errorFn.Sprintf("Errors: %d", errorCount))
	} else {
		parts = append(parts, p.summaryKey.Sprint("Errors: ")+p.returnVal.Sprint("0"))
	}
	if status != "" {
		valColor := p.returnVal
		if status == "failed" || status == "error" {
			valColor = p.errorFn
		}
		parts = append(parts, p.summaryKey.Sprint("Status: ")+valColor.Sprint(status))
	}
	_, _ = fmt.Fprintln(out, " "+strings.Join(parts, p.dimmed.Sprint("  │  ")))
}

// ─────────────────────────────────────────────────────────────────────────────
// colour mapping
// ─────────────────────────────────────────────────────────────────────────────

func (p palette) colorForType(t string) *color.Color {
	switch t {
	case EventTypeContractCall, "contract_init", "balance_check":
		return p.contractFn
	case EventTypeHostFunction, "host_fn":
		return p.hostFn
	case EventTypeAuth:
		return p.authFn
	case "event":
		return p.eventFn
	case "error":
		return p.errorFn
	case "log":
		return p.logFn
	case EventTypeTrap:
		return p.trapFn
	case "transaction", "simulation":
		return p.txRoot
	case "collapsed":
		return p.dimmed
	default:
		return p.otherFn
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Icons
// ─────────────────────────────────────────────────────────────────────────────

func iconForType(t string) string {
	switch t {
	case EventTypeContractCall,
		"contract_init", "balance_check":
		return "◆"
	case EventTypeHostFunction, "host_fn":
		return "⚙"
	case EventTypeAuth:
		return "🔐"
	case "event":
		return "◉"
	case "error":
		return "[FAIL]"
	case "log":
		return "▪"
	case EventTypeTrap:
		return "[READY]"
	case "transaction", "simulation":
		return "▸"
	case "transaction_complete":
		return "■"
	case "error_handling":
		return "⚠"
	case "collapsed":
		return "…"
	default:
		return "·"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tree connector helpers
// ─────────────────────────────────────────────────────────────────────────────

// treeConnectors returns the branch line and the continuation prefix for a
// top-level (depth-0) child. Used by PrintExecutionTrace's flat list.
func treeConnectors(isLast bool) (connector, continuation string) {
	if isLast {
		return "└─ ", "   "
	}
	return "├─ ", "│  "
}

// prefixParts returns the visual connector to print before a node and the
// prefix to use for that node's children/sub-lines. prefix is the accumulated
// indentation from the node's ancestors.
func prefixParts(prefix string, isLast bool) (connector, childPrefix string) {
	if isLast {
		return "└─ ", prefix + "   "
	}
	return "├─ ", prefix + "│  "
}

// ─────────────────────────────────────────────────────────────────────────────
// Formatting utilities
// ─────────────────────────────────────────────────────────────────────────────

// shortID truncates a contract/account ID to fit within available width,
// preserving the first 6 and last 4 characters when truncation is needed.
func shortID(id string, maxW int) string {
	const minKeep = 12
	if len(id) <= minKeep || maxW >= len(id)+6 {
		return id
	}
	return id[:6] + "…" + id[len(id)-4:]
}

// truncateHash truncates a transaction hash for display, keeping some entropy.
func truncateHash(hash string, maxLen int) string {
	if maxLen <= 0 || len(hash) <= maxLen {
		return hash
	}
	return hash[:maxLen-1] + "…"
}

// wrapText wraps text to the given max width, breaking on spaces when possible.
func wrapText(text string, maxW int) string {
	if maxW < 20 {
		maxW = 20
	}
	runes := []rune(text)
	if len(runes) <= maxW {
		return text
	}
	var lines []string
	for len(runes) > 0 {
		if len(runes) <= maxW {
			lines = append(lines, string(runes))
			break
		}
		cut := maxW
		// try to break at a space (working on runes, not bytes)
		for i := cut - 1; i >= 0; i-- {
			if runes[i] == ' ' {
				cut = i
				break
			}
		}
		lines = append(lines, string(runes[:cut]))
		// advance past any leading spaces in the remaining runes
		next := cut
		for next < len(runes) && runes[next] == ' ' {
			next++
		}
		runes = runes[next:]
	}
	return strings.Join(lines, "\n")
}

// formatNum formats a uint64 with comma separators.
func formatNum(n uint64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(ch))
	}
	return string(out)
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(n uint64) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
