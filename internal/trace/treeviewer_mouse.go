// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// TreeViewerWithMouse provides an interactive tree-based trace viewer with mouse support
type TreeViewerWithMouse struct {
	trace        *ExecutionTrace
	renderer     *TreeRenderer
	mouseTracker *MouseTracker
	screenWidth  int
	screenHeight int
	running      bool
}

// NewTreeViewerWithMouse creates a new tree viewer with mouse support
func NewTreeViewerWithMouse(trace *ExecutionTrace) *TreeViewerWithMouse {
	return &TreeViewerWithMouse{
		trace:        trace,
		renderer:     NewTreeRenderer(80, 24),
		mouseTracker: NewMouseTracker(),
		screenWidth:  80,
		screenHeight: 24,
		running:      false,
	}
}

// StartWithMouse launches the tree viewer with mouse support
func (tv *TreeViewerWithMouse) StartWithMouse() error {
	// Save terminal state
	initialTermState, err := tv.saveTerminalState()
	if err != nil {
		return fmt.Errorf("failed to save terminal state: %w", err)
	}

	// Enable raw mode and mouse tracking
	if err := tv.enableRawMode(); err != nil {
		return fmt.Errorf("failed to enable raw mode: %w", err)
	}
	defer func() { _ = tv.restoreTerminalState(initialTermState) }() //nolint:errcheck // Best-effort terminal restore on exit

	// Enable mouse tracking
	if err := tv.mouseTracker.Enable(); err != nil {
		return fmt.Errorf("failed to enable mouse tracking: %w", err)
	}
	defer func() { _ = tv.mouseTracker.Disable() }() //nolint:errcheck // Best-effort mouse tracking disable on exit

	// Build initial tree
	nodes := make([]*TraceNode, 0)
	if tv.trace != nil && len(tv.trace.States) > 0 {
		// For now, create a simplified tree from trace states
		root := NewTraceNode("root", "trace")
		for i, state := range tv.trace.States {
			node := NewTraceNode(fmt.Sprintf("step-%d", i), state.Operation)
			node.Function = state.Function
			node.ContractID = state.ContractID
			node.Error = state.Error
			root.AddChild(node)
		}
		nodes = append(nodes, root)
	}

	if len(nodes) > 0 {
		tv.renderer.RenderTree(nodes[0])
	}

	tv.running = true
	defer func() { tv.running = false }()

	// Setup signal handling for clean exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Render initial view
	tv.renderView()

	// Main event loop
	for tv.running {
		select {
		case <-sigChan:
			return nil
		case <-time.After(50 * time.Millisecond):
			// Check for input (non-blocking)
			if tv.handleInput() {
				return nil
			}
		}
	}

	return nil
}

// handleInput processes keyboard and mouse input
func (tv *TreeViewerWithMouse) handleInput() bool {
	// Read escape sequences
	buf := make([]byte, 100)
	n, err := syscall.Read(0, buf)
	if err != nil || n == 0 {
		return false
	}

	input := string(buf[:n])

	// Handle mouse events
	if strings.HasPrefix(input, "\x1b[<") || strings.HasPrefix(input, "\x1b[M") {
		// Extract mouse sequence
		var sequence string
		if strings.HasPrefix(input, "\x1b[<") {
			// SGR format
			end := strings.Index(input[3:], "M")
			if end == -1 {
				end = strings.Index(input[3:], "m")
			}
			if end > 0 {
				sequence = input[2 : 3+end+1]
			}
		} else {
			// Basic format
			sequence = input[2:5]
		}

		if evt, err := ParseMouseEvent(sequence); err == nil {
			tv.handleMouseEvent(evt)
			tv.renderView()
			return false
		}
	}

	// Handle keyboard input
	switch input {
	case "n": // Next step/instruction
		tv.NextStep()
		tv.renderView()
		return false

	case "\x1b[A", "k": // Up arrow or k
		tv.renderer.SelectUp()
		tv.renderView()
		return false

	case "\x1b[B", "j": // Down arrow or j
		tv.renderer.SelectDown()
		tv.renderView()
		return false

	case " ", "\r": // Space or Enter - toggle expand
		node := tv.renderer.GetSelectedNode()
		if node != nil && !node.IsLeaf() {
			node.ToggleExpanded()
			tv.renderer.RenderTree(tv.getTraceRoot())
			tv.renderView()
		}
		return false

	case "e": // Expand all
		if root := tv.getTraceRoot(); root != nil {
			root.ExpandAll()
			tv.renderer.RenderTree(root)
			tv.renderView()
		}
		return false

	case "c": // Collapse all
		if root := tv.getTraceRoot(); root != nil {
			root.CollapseAll()
			tv.renderer.RenderTree(root)
			tv.renderView()
		}
		return false

	case "q", "\x03": // q or Ctrl+C - quit
		return true

	case "h": // Help
		tv.showHelp()
		return false
	}

	return false
}

// handleMouseEvent processes a mouse event
func (tv *TreeViewerWithMouse) handleMouseEvent(evt *MouseEvent) {
	if evt.IsScrollEvent() {
		switch evt.Button {
		case ScrollUp:
			tv.renderer.SelectUp()
		case ScrollDown:
			tv.renderer.SelectDown()
		}
	} else if evt.IsClickEvent() {
		toggled := tv.renderer.HandleMouseClick(evt.Col, evt.Row)
		if toggled {
			root := tv.getTraceRoot()
			tv.renderer.RenderTree(root)
		}
	}
}

// getTraceRoot returns the root node of the current tree
func (tv *TreeViewerWithMouse) getTraceRoot() *TraceNode {
	nodes := tv.renderer.GetAllNodes()
	if len(nodes) > 0 {
		return nodes[0].Node
	}
	return nil
}

// renderView clears and renders the current view
func (tv *TreeViewerWithMouse) renderView() {
	// Clear screen
	_, _ = fmt.Print("\x1b[2J\x1b[H") //nolint:errcheck // Terminal output, write failure is non-critical

	// Render header
	_, _ = fmt.Printf("ERST Interactive Trace Tree Viewer (Mouse Support Enabled)\n")                  //nolint:errcheck // Terminal output, write failure is non-critical
	_, _ = fmt.Printf("Transaction: %s | Steps: %d\n", tv.trace.TransactionHash, len(tv.trace.States)) //nolint:errcheck // Terminal output, write failure is non-critical
	_, _ = fmt.Print("─────────────────────────────────────────────────────────\n")                    //nolint:errcheck // Terminal output, write failure is non-critical

	// Render tree
	_, _ = fmt.Print(tv.renderer.Render()) //nolint:errcheck // Terminal output, write failure is non-critical

	// Render footer
	_, _ = fmt.Print("\n─────────────────────────────────────────────────────────\n")                                                   //nolint:errcheck // Terminal output, write failure is non-critical
	_, _ = fmt.Print("Controls: n=next-step | ↑↓/kj=navigate | Space/Enter=expand | e=expand-all | c=collapse-all | h=help | q=quit\n") //nolint:errcheck // Terminal output, write failure is non-critical
	_, _ = fmt.Print(tv.renderCurrentStateView())                                                                                       //nolint:errcheck // Terminal output, write failure is non-critical
}

// showHelp displays help information
func (tv *TreeViewerWithMouse) showHelp() {
	helpText := `
╔════════════════════════════════════════════════════════════════════════════╗
║                              KEYBOARD SHORTCUTS                            ║
├────────────────────────────────────────────────────────────────────────────┤
║ Navigation:                                                                ║
║   n              Move to next instruction/event                            ║
║   ↑ / k          Move up                                                   ║
║   ↓ / j          Move down                                                 ║
║   Space/Enter    Toggle expand/collapse on selected node                  ║
║   e              Expand all nodes                                          ║
║   c              Collapse all nodes                                        ║
║                                                                            ║
║ Mouse:                                                                     ║
║   Click [+/-]    Toggle expand/collapse node (tree UI only)               ║
║   Scroll         Scroll through tree                                       ║
║   Click node     Select node                                              ║
║                                                                            ║
║ Other:                                                                     ║
║   h              Show this help                                            ║
║   q / Ctrl+C     Quit viewer                                               ║
╚════════════════════════════════════════════════════════════════════════════╝

The tree shows your execution trace with expandable nodes. Click on [+] or [-]
symbols with your mouse, or use keyboard shortcuts to navigate.

Press any key to continue...
`
	_, _ = fmt.Print(helpText) //nolint:errcheck // Terminal output, write failure is non-critical

	// Wait for input
	buf := make([]byte, 1)
	_, _ = syscall.Read(0, buf) //nolint:errcheck // Best-effort input wait in help view

	tv.renderView()
}

// NextStep advances to the next execution step and synchronizes tree focus
// to the corresponding node in the rendered list.
func (tv *TreeViewerWithMouse) NextStep() {
	if tv.trace == nil || len(tv.trace.States) == 0 {
		return
	}

	state, err := tv.trace.StepForward()
	if err != nil {
		return
	}

	// Tree nodes are currently built as step-<index>; sync focus to the new step.
	tv.renderer.SelectNodeByID(fmt.Sprintf("step-%d", state.Step))
}

func (tv *TreeViewerWithMouse) renderCurrentStateView() string {
	if tv.trace == nil || len(tv.trace.States) == 0 {
		return "State: no trace loaded\n"
	}

	state, err := tv.trace.GetCurrentState()
	if err != nil {
		return "State: unavailable\n"
	}

	operation := state.Operation
	if operation == "" {
		operation = "-"
	}
	eventType := state.EventType
	if eventType == "" {
		eventType = "inferred"
	}
	function := state.Function
	if function == "" {
		function = "-"
	}

	return fmt.Sprintf(
		"State: step=%d/%d op=%s event=%s fn=%s\n",
		state.Step,
		len(tv.trace.States)-1,
		operation,
		eventType,
		function,
	)
}

// Terminal control methods

func (tv *TreeViewerWithMouse) saveTerminalState() (string, error) {
	// Save terminal settings using stty
	// For now, return empty string - would use stty in production
	return "", nil
}

func (tv *TreeViewerWithMouse) restoreTerminalState(state string) error {
	// Restore terminal settings
	// In production, would use stty
	_, _ = fmt.Print("\x1b[?25h") // Show cursor
	return nil
}

func (tv *TreeViewerWithMouse) enableRawMode() error {
	// Enable raw mode using stty-like behavior
	// Hide cursor
	_, _ = fmt.Print("\x1b[?25l") //nolint:errcheck // Terminal output, write failure is non-critical
	return nil
}
