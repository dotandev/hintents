// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/trace"
)

// StateRow is a single key-value pair shown in the right pane.
type StateRow struct {
	Key   string
	Value string
}

// TraceView is the split-screen controller that wires a trace.TreeRenderer
// (left pane) to a state table (right pane).
//
// Typical lifecycle:
//
//	tv := ui.NewTraceView(root)
//	layout := ui.NewSplitLayout()
//	resize := layout.ListenResize()
//	kr := ui.NewKeyReader()
//
//	for {
//	    tv.Render(layout)
//	    key, _ := kr.Read()
//	    if done := tv.HandleKey(key, layout); done { break }
//	    select {
//	    case <-resize:
//	        tv.Resize(layout.Width, layout.Height)
//	    default:
//	    }
//	}
type TraceView struct {
	tree        *trace.TreeRenderer
	stateRows   []StateRow
	stateScroll int
	stateSel    int
}

func NewTraceView(root *trace.TraceNode) *TraceView {
	w, h := TermSize()
	tv := &TraceView{
		tree: trace.NewTreeRenderer(w/2, h-3),
	}
	tv.tree.RenderTree(root)
	tv.refreshState()
	return tv
}

func (tv *TraceView) Resize(w, h int) {
	tv.tree = trace.NewTreeRenderer(w/2, h-3)
}

func (tv *TraceView) HandleKey(k Key, layout *SplitLayout) (done bool) {
	switch k {
	case KeyQuit:
		return true

	case KeyTab:
		layout.ToggleFocus()

	case KeyLeft:
		layout.SetFocus(PaneTrace)

	case KeyRight:
		layout.SetFocus(PaneState)

	case KeyUp:
		if layout.Focus == PaneTrace {
			tv.tree.SelectUp()
			tv.refreshState()
		} else {
			tv.stateScrollUp()
		}

	case KeyDown:
		if layout.Focus == PaneTrace {
			tv.tree.SelectDown()
			tv.refreshState()
		} else {
			tv.stateScrollDown()
		}

	case KeyEnter:
		if layout.Focus == PaneTrace {
			if node := tv.tree.GetSelectedNode(); node != nil {
				node.ToggleExpanded()
				root := treeRoot(node)
				tv.tree.RenderTree(root)
				tv.refreshState()
			}
		}
	}

	return false
}

func (tv *TraceView) Render(layout *SplitLayout) {
	lw := layout.LeftWidth()
	rw := layout.RightWidth()
	contentRows := layout.Height - 3

	leftLines := tv.renderTraceLines(lw, contentRows)
	rightLines := tv.renderStateLines(rw, contentRows)

	layout.Render(leftLines, rightLines)
}

// ──────────────────────────────────────────────────────────────────────────────
// Left pane — Trace tree
// ──────────────────────────────────────────────────────────────────────────────

func (tv *TraceView) renderTraceLines(width, maxRows int) []string {
	// Re-render the tree into a string and split on newlines.
	raw := tv.tree.Render()
	all := strings.Split(raw, "\n")

	// Clip to maxRows and pad to width.
	lines := make([]string, maxRows)
	for i := 0; i < maxRows; i++ {
		text := ""
		if i < len(all) {
			text = all[i]
		}
		lines[i] = padOrClip(text, width)
	}
	return lines
}

// ──────────────────────────────────────────────────────────────────────────────
// Right pane — State table
// ──────────────────────────────────────────────────────────────────────────────

// refreshState rebuilds stateRows from the currently selected trace node.
func (tv *TraceView) refreshState() {
	node := tv.tree.GetSelectedNode()
	tv.stateRows = nodeToStateRows(node)
	// Keep selection in bounds.
	if tv.stateSel >= len(tv.stateRows) {
		tv.stateSel = len(tv.stateRows) - 1
	}
	if tv.stateSel < 0 {
		tv.stateSel = 0
	}
	tv.stateScroll = 0
}

func (tv *TraceView) stateScrollUp() {
	if tv.stateSel > 0 {
		tv.stateSel--
	}
	if tv.stateSel < tv.stateScroll {
		tv.stateScroll = tv.stateSel
	}
}

func (tv *TraceView) stateScrollDown() {
	if tv.stateSel < len(tv.stateRows)-1 {
		tv.stateSel++
	}
}

func (tv *TraceView) renderStateLines(width, maxRows int) []string {
	lines := make([]string, maxRows)

	// Header row.
	keyW := width / 3
	valW := width - keyW - 3 // "  │ "
	if keyW < 4 {
		keyW = 4
	}
	if valW < 4 {
		valW = 4
	}
	header := fmt.Sprintf("  %-*s  %s", keyW, "Key", "Value")
	lines[0] = padOrClip(header, width)

	divider := "  " + strings.Repeat("─", width-2)
	lines[1] = padOrClip(divider, width)

	// Data rows starting at line 2.
	visStart := tv.stateScroll
	row := 2
	for i := visStart; i < len(tv.stateRows) && row < maxRows; i++ {
		sr := tv.stateRows[i]
		prefix := "  "
		if i == tv.stateSel {
			prefix = "▸ "
		}
		key := padOrClip(sr.Key, keyW)
		val := padOrClip(sr.Value, valW)
		line := fmt.Sprintf("%s%-*s  %s", prefix, keyW, key, val)
		lines[row] = padOrClip(line, width)
		row++
	}

	// Empty rows already zero-value strings (""); pad them.
	for ; row < maxRows; row++ {
		lines[row] = strings.Repeat(" ", width)
	}

	if len(tv.stateRows) == 0 {
		msg := "  (no state for selected node)"
		lines[2] = padOrClip(msg, width)
	}

	return lines
}

// nodeToStateRows converts a TraceNode into display rows for the state table.
func nodeToStateRows(node *trace.TraceNode) []StateRow {
	if node == nil {
		return nil
	}
	var rows []StateRow

	add := func(k, v string) {
		rows = append(rows, StateRow{Key: k, Value: v})
	}

	add("type", node.Type)
	if node.ContractID != "" {
		add("contract_id", node.ContractID)
	}
	if node.Function != "" {
		add("function", node.Function)
	}
	add("depth", fmt.Sprintf("%d", node.Depth))
	if node.EventData != "" {
		add("event_data", node.EventData)
	}
	if node.Error != "" {
		add("error", node.Error)
	}
	if node.CPUDelta != nil {
		add("cpu_delta", fmt.Sprintf("%d instructions", *node.CPUDelta))
	}
	if node.MemoryDelta != nil {
		add("mem_delta", fmt.Sprintf("%d bytes", *node.MemoryDelta))
	}
	if node.SourceRef != nil {
		ref := node.SourceRef
		loc := fmt.Sprintf("%s:%d", ref.File, ref.Line)
		if ref.Column > 0 {
			loc = fmt.Sprintf("%s:%d", loc, ref.Column)
		}
		add("source", loc)
		if ref.Function != "" {
			add("src_function", ref.Function)
		}
	}
	add("children", fmt.Sprintf("%d", len(node.Children)))
	if node.IsLeaf() {
		add("leaf", "true")
	}
	if node.IsCrossContractCall() {
		add("cross_contract", "true")
	}

	return rows
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// padOrClip pads s with spaces to exactly width, or clips it if longer.
func padOrClip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// treeRoot walks parent pointers to find the root TraceNode.
func treeRoot(n *trace.TraceNode) *trace.TraceNode {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}
