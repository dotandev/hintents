// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/dotandev/hintents/internal/dwarf"
	"github.com/dotandev/hintents/internal/visualizer"
)

// InteractiveViewer provides a terminal-based interactive trace navigation interface
type InteractiveViewer struct {
	trace       *ExecutionTrace
	reader      *bufio.Reader
	eventFilter string   // one of EventTypeTrap, EventTypeContractCall, EventTypeHostFunction, EventTypeAuth, or ""
	filterCycle []string // order for cycling: off, trap, contract_call, host_function, auth
	hideStdLib  bool
	trap        *TrapInfo
	dwarfParser *dwarf.Parser
}

// NewInteractiveViewer creates a new interactive trace viewer
func NewInteractiveViewer(trace *ExecutionTrace) *InteractiveViewer {
	viewer := &InteractiveViewer{
		trace:       trace,
		reader:      bufio.NewReader(os.Stdin),
		eventFilter: "",
		filterCycle: []string{"", EventTypeTrap, EventTypeContractCall, EventTypeHostFunction, EventTypeAuth},
	}

	// Detect any traps in the trace
	detector := &TrapDetector{}
	viewer.trap = detector.FindTrapPoint(trace)

	return viewer
}

// NewInteractiveViewerWithWASM creates a new interactive trace viewer with WASM data for local variable inspection
func NewInteractiveViewerWithWASM(trace *ExecutionTrace, wasmData []byte) *InteractiveViewer {
	viewer := &InteractiveViewer{
		trace:       trace,
		reader:      bufio.NewReader(os.Stdin),
		eventFilter: "",
		filterCycle: []string{"", EventTypeTrap, EventTypeContractCall, EventTypeHostFunction, EventTypeAuth},
	}

	// Initialize DWARF parser if WASM data is provided
	if len(wasmData) > 0 {
		parser, err := dwarf.NewParser(wasmData)
		if err == nil && parser.HasDebugInfo() {
			viewer.dwarfParser = parser
		}
	}

	// Detect any traps in the trace
	detector := &TrapDetector{}
	viewer.trap = detector.FindTrapPoint(trace)

	return viewer
}

// Start begins the interactive trace viewing session.
// It installs a terminal-resize handler so that long contract IDs and XDR
// strings reflow correctly whenever the window size changes.
func (v *InteractiveViewer) Start() error {
	termW := getTermWidth()
	fmt.Printf("%s ERST Interactive Trace Viewer\n", visualizer.Symbol("magnify"))
	fmt.Println(separator(termW))
	fmt.Printf("Transaction: %s\n", v.trace.TransactionHash)
	fmt.Printf("Total Steps: %d\n\n", len(v.trace.States))

	// Show trap info at startup if detected
	if v.trap != nil {
		fmt.Printf("%s Memory Trap Detected!\n", visualizer.Symbol("warn"))
		fmt.Printf("Type: %s\n", v.trap.Type)
		if v.trap.SourceLocation != nil {
			fmt.Printf("Location: %s:%d\n", v.trap.SourceLocation.File, v.trap.SourceLocation.Line)
		}
		fmt.Println("  Use 't' or 'trap' command to see local variables")
	}

	// Resize handling: on SIGWINCH (Unix), reflow the current state display.
	resizeCh := make(chan os.Signal, 1)
	watchResize(resizeCh)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-resizeCh:
				// Reprint current state with updated terminal width.
				fmt.Print("\n")
				v.displayCurrentState()
				fmt.Print("\n> ")
			case <-done:
				return
			}
		}
	}()

	v.showHelp()
	v.displayCurrentState()

	for {
		fmt.Print("\n> ")
		input, err := v.reader.ReadString('\n')
		if err != nil {
			close(done)
			signal.Stop(resizeCh)
			return fmt.Errorf("failed to read input: %w", err)
		}

		command := strings.TrimSpace(input)
		if command == "" {
			continue
		}

		if v.handleCommand(command) {
			break
		}
	}

	close(done)
	signal.Stop(resizeCh)
	return nil
}

// handleCommand processes user commands and returns true if exit is requested
func (v *InteractiveViewer) handleCommand(command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}

	cmdExact := parts[0]
	cmd := strings.ToLower(cmdExact)

	// Handle case-sensitive 'S' for the stdlib toggle before the lowercased switch
	if cmdExact == "S" {
		v.hideStdLib = !v.hideStdLib
		status := "shown"
		if v.hideStdLib {
			status = "hidden"
		}
		fmt.Printf("%s Rust core::* traces are now %s\n", visualizer.Symbol("eye"), status)
		return false
	}

	switch cmd {
	case "n", "next", "forward":
		v.stepForward()
	case "p", "prev", "back", "backward":
		v.stepBackward()
	case "f", "filter":
		v.cycleEventFilter()
	case "j", "jump":
		if len(parts) > 1 {
			v.jumpToStep(parts[1])
		} else {
			fmt.Println("Usage: jump <step_number>")
		}
	case "s", "show", "state":
		v.displayCurrentState()
	case "r", "reconstruct":
		if len(parts) > 1 {
			v.reconstructState(parts[1])
		} else {
			v.reconstructCurrentState()
		}
	case "t", "trap":
		v.displayTrapInfo()
	case "i", "info":
		v.showNavigationInfo()
	case "sp", "split":
		v.showSplitPane()
	case "l", "list":
		if len(parts) > 1 {
			v.listSteps(parts[1])
		} else {
			v.listSteps("10")
		}
	case "?", "h", "help":
		v.showHelp()
	case "q", "quit", "exit":
		fmt.Printf("Goodbye! %s\n", visualizer.Symbol("wave"))
		return true
	case "y", "yank", "copy":
		if len(parts) > 1 {
			v.handleYank(parts[1:])
		} else {
			fmt.Println("Usage: yank <a/r> [index]")
		}
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", cmdExact)
	}

	return false
}

// stepForward moves to the next step, respecting the event filter and hideStdLib toggle.
func (v *InteractiveViewer) stepForward() {
	for {
		var state *ExecutionState
		var err error
		if v.eventFilter != "" {
			state, err = v.trace.FilteredStepForward(v.eventFilter)
		} else {
			state, err = v.trace.StepForward()
		}
		if err != nil {
			fmt.Printf("%s %s\n", visualizer.Error(), err)
			return
		}

		if v.hideStdLib && strings.HasPrefix(state.Function, "core::") {
			continue
		}

		fmt.Printf("%s  Stepped forward to step %d\n", visualizer.Symbol("arrow_r"), state.Step)
		v.displayCurrentState()
		return
	}
}

// stepBackward moves to the previous step, respecting the event filter and hideStdLib toggle.
func (v *InteractiveViewer) stepBackward() {
	for {
		var state *ExecutionState
		var err error
		if v.eventFilter != "" {
			state, err = v.trace.FilteredStepBackward(v.eventFilter)
		} else {
			state, err = v.trace.StepBackward()
		}
		if err != nil {
			fmt.Printf("%s %s\n", visualizer.Error(), err)
			return
		}

		if v.hideStdLib && strings.HasPrefix(state.Function, "core::") {
			continue
		}

		fmt.Printf("%s  Stepped backward to step %d\n", visualizer.Symbol("arrow_l"), state.Step)
		v.displayCurrentState()
		return
	}
}

// cycleEventFilter cycles through filter options: off -> trap -> contract_call -> host_function -> auth -> off
func (v *InteractiveViewer) cycleEventFilter() {
	for i, f := range v.filterCycle {
		if f == v.eventFilter {
			next := (i + 1) % len(v.filterCycle)
			v.eventFilter = v.filterCycle[next]
			break
		}
	}
	if v.eventFilter == "" {
		fmt.Println("Filter: off (all steps)")
	} else {
		matching := v.trace.FilteredStepCount(v.eventFilter)
		fmt.Printf("Filter: %s (%d matching steps)\n", v.eventFilter, matching)
	}
}

// jumpToStep jumps to a specific step
func (v *InteractiveViewer) jumpToStep(stepStr string) {
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		fmt.Printf("%s Invalid step number: %s\n", visualizer.Error(), stepStr)
		return
	}

	state, err := v.trace.JumpToStep(step)
	if err != nil {
		fmt.Printf("%s %s\n", visualizer.Error(), err)
		return
	}

	fmt.Printf("%s Jumped to step %d\n", visualizer.Symbol("target"), state.Step)
	v.displayCurrentState()
}

// displayCurrentState shows the current execution state, reflowing long
// contract IDs and XDR strings to fit the current terminal width.
func (v *InteractiveViewer) displayCurrentState() {
	state, err := v.trace.GetCurrentState()
	if err != nil {
		fmt.Printf("%s %s\n", visualizer.Error(), err)
		return
	}

	termW := getTermWidth()
	fmt.Printf("\n%s Current State\n", visualizer.Symbol("pin"))
	fmt.Println(separator(termW))

	if v.eventFilter != "" {
		filteredIdx := v.trace.FilteredCurrentIndex(v.eventFilter)
		filteredTotal := v.trace.FilteredStepCount(v.eventFilter)
		fmt.Printf("Step: %d/%d (filter %s: %d/%d)\n", state.Step, len(v.trace.States)-1, v.eventFilter, filteredIdx, filteredTotal)
	} else {
		fmt.Printf("Step: %d/%d\n", state.Step, len(v.trace.States)-1)
	}

	fmt.Printf("Time: %s\n", state.Timestamp.Format("15:04:05.000"))
	fmt.Printf("Operation: %s\n", state.Operation)

	if state.ContractID != "" {
		fmt.Println(wrapField("Contract", state.ContractID, termW))
	}

	// Indicate cross-contract transition from previous step
	if state.Step > 0 && state.ContractID != "" {
		prev := &v.trace.States[state.Step-1]
		if prev.ContractID != "" && prev.ContractID != state.ContractID {
			fmt.Printf("%s\n", visualizer.ContractBoundary(prev.ContractID, state.ContractID))
		}
	}
	if state.Function != "" {
		fmt.Println(wrapField("Function", state.Function, termW))
	}
	if len(state.Arguments) > 0 {
		fmt.Println(wrapField("Arguments", fmt.Sprintf("%v", state.Arguments), termW))
	}
	if state.ReturnValue != nil {
		fmt.Println(wrapField("Return", fmt.Sprintf("%v", state.ReturnValue), termW))
	}
	if state.WasmInstruction != "" {
		fmt.Printf("WASM Instruction: %s\n", state.WasmInstruction)
	}
	if state.Error != "" {
		indicator := visualizer.Error() + " "
		fmt.Printf("%s%s\n", indicator, wrapField("Error", state.Error, termW-len(indicator)))
		if v.trap != nil && IsMemoryTrap(v.trap) {
			fmt.Println("\n  Use 't' or 'trap' to see local variables at this point")
		}
	}

	if len(state.HostState) > 0 {
		fmt.Printf("Host State: %d entries\n", len(state.HostState))
	}
	if len(state.Memory) > 0 {
		fmt.Printf("Memory: %d entries\n", len(state.Memory))
	}
}

// reconstructCurrentState reconstructs and displays the current state
func (v *InteractiveViewer) reconstructCurrentState() {
	state, err := v.trace.ReconstructStateAt(v.trace.CurrentStep)
	if err != nil {
		fmt.Printf("%s Failed to reconstruct state: %s\n", visualizer.Error(), err)
		return
	}

	termW := getTermWidth()
	fmt.Printf("\n%s Reconstructed State\n", visualizer.Symbol("wrench"))
	fmt.Println(separator(termW))
	v.displayState(state)
}

// reconstructState reconstructs and displays state at a specific step
func (v *InteractiveViewer) reconstructState(stepStr string) {
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		fmt.Printf("%s Invalid step number: %s\n", visualizer.Error(), stepStr)
		return
	}

	state, err := v.trace.ReconstructStateAt(step)
	if err != nil {
		fmt.Printf("%s Failed to reconstruct state: %s\n", visualizer.Error(), err)
		return
	}

	termW := getTermWidth()
	fmt.Printf("\n%s Reconstructed State at Step %d\n", visualizer.Symbol("wrench"), step)
	fmt.Println(separator(termW))
	v.displayState(state)
}

// displayState displays a complete state, reflowing long values to fit the
// current terminal width.
func (v *InteractiveViewer) displayState(state *ExecutionState) {
	termW := getTermWidth()
	fmt.Printf("Step: %d\n", state.Step)
	fmt.Printf("Time: %s\n", state.Timestamp.Format("15:04:05.000"))
	fmt.Printf("Operation: %s\n", state.Operation)

	if state.ContractID != "" {
		fmt.Println(wrapField("Contract", state.ContractID, termW))
	}
	if state.Function != "" {
		fmt.Println(wrapField("Function", state.Function, termW))
	}

	if len(state.HostState) > 0 {
		fmt.Println("\nHost State:")
		for k, val := range state.HostState {
			fmt.Printf("  %s\n", wrapField(k, fmt.Sprintf("%v", val), termW-2))
		}
	}

	if len(state.Memory) > 0 {
		fmt.Println("\nMemory:")
		for k, val := range state.Memory {
			fmt.Printf("  %s\n", wrapField(k, fmt.Sprintf("%v", val), termW-2))
		}
	}
}

// showNavigationInfo displays navigation information
func (v *InteractiveViewer) showNavigationInfo() {
	info := v.trace.GetNavigationInfo()

	termW := getTermWidth()
	fmt.Printf("\n%s Navigation Info\n", visualizer.Symbol("chart"))
	fmt.Println(separator(termW))
	fmt.Printf("Total Steps: %d\n", info["total_steps"])
	fmt.Printf("Current Step: %d\n", info["current_step"])
	if v.eventFilter != "" {
		fmt.Printf("Filter: %s (%d matching)\n", v.eventFilter, v.trace.FilteredStepCount(v.eventFilter))
		fmt.Printf("Filtered Index: %d\n", v.trace.FilteredCurrentIndex(v.eventFilter))
	} else {
		fmt.Printf("Filter: off\n")
	}
	fmt.Printf("Can Step Back: %t\n", info["can_step_back"])
	fmt.Printf("Can Step Forward: %t\n", info["can_step_forward"])
	fmt.Printf("Snapshots: %d\n", info["snapshots_count"])

	if v.trap != nil {
		fmt.Printf("\n%s Trap Detected: %s\n", visualizer.Error(), v.trap.Type)
		fmt.Println("  Type 't' or 'trap' to see details with local variables")
	}
}

// displayTrapInfo displays trap information including local variables
func (v *InteractiveViewer) displayTrapInfo() {
	if v.trap == nil {
		fmt.Printf("%s No trap detected in this trace\n", visualizer.Symbol("check"))
		return
	}

	fmt.Println("\n" + FormatTrapInfo(v.trap))
}

// listSteps shows a list of steps around the current position.
// Each line is truncated to the terminal width to keep the tree readable.
func (v *InteractiveViewer) listSteps(countStr string) {
	count, err := strconv.Atoi(countStr)
	if err != nil {
		count = 10
	}

	termW := getTermWidth()
	current := v.trace.CurrentStep
	start := max(0, current-count/2)
	end := min(len(v.trace.States)-1, start+count-1)

	fmt.Printf("\n%s Steps %d-%d\n", visualizer.Symbol("list"), start, end)
	if v.eventFilter != "" {
		fmt.Printf("Filter: %s\n", v.eventFilter)
	}
	if v.hideStdLib {
		fmt.Println("(Filtering out core::* traces)")
	}
	fmt.Println(separator(termW))

	prevContractID := ""
	if start > 0 {
		prevContractID = v.trace.States[start-1].ContractID
	}

	for i := start; i <= end; i++ {
		state := &v.trace.States[i]

		// Highlight cross-contract call boundary
		if state.ContractID != "" && prevContractID != "" && state.ContractID != prevContractID {
			fmt.Printf("     %s\n", visualizer.ContractBoundary(prevContractID, state.ContractID))
		}
		if state.ContractID != "" {
			prevContractID = state.ContractID
		}

		if v.hideStdLib && strings.HasPrefix(state.Function, "core::") {
			continue
		}

		marker := "  "
		if i == current {
			marker = visualizer.Symbol("play")
		}

		typeTag := ""
		if v.eventFilter != "" && v.trace.StepMatchesFilter(i, v.eventFilter) {
			typeTag = " [" + v.eventFilter + "]"
		} else if v.eventFilter != "" {
			typeTag = " (" + ClassifyEventType(state) + ")"
		}

		line := fmt.Sprintf("%s %3d: %s", marker, i, state.Operation)
		if state.Function != "" {
			line += fmt.Sprintf(" (%s)", state.Function)
		}
		if state.Error != "" {
			line += fmt.Sprintf(" %s", visualizer.Error())
		}
		line += typeTag

		// Truncate to terminal width to preserve tree alignment.
		if len(line) > termW && termW > 3 {
			line = line[:termW-3] + "..."
		}
		fmt.Println(line)
	}
}

// showSplitPane renders the horizontal split-pane view for the current step.
func (v *InteractiveViewer) showSplitPane() {
	state, err := v.trace.GetCurrentState()
	if err != nil {
		fmt.Printf("%s %s\n", visualizer.Error(), err)
		return
	}
	node := executionStateToNode(state)
	var src *SourceContext
	if node.SourceRef != nil {
		src, _ = LoadSourceContext(*node.SourceRef, defaultRadius)
	}
	pane := DefaultSplitPane()
	pane.Render(os.Stdout, node, src)
}

// executionStateToNode derives a TraceNode from an ExecutionState for display
// in the split pane. The SourceRef field is populated when the state carries
// enough information to identify a source location.
func executionStateToNode(state *ExecutionState) *TraceNode {
	node := NewTraceNode(fmt.Sprintf("step-%d", state.Step), state.Operation)
	node.ContractID = state.ContractID
	node.Function = state.Function
	if state.Error != "" {
		node.Error = state.Error
		node.Type = "error"
	}
	return node
}

// showHelp displays available keyboard shortcuts
func (v *InteractiveViewer) showHelp() {
	termW := getTermWidth()
	fmt.Printf("\n%s Keyboard Shortcuts\n", visualizer.Symbol("book"))
	fmt.Println(separator(termW))
	fmt.Println("Navigation:")
	fmt.Println("  n, next, forward        - Step forward")
	fmt.Println("  p, prev, back           - Step backward")
	fmt.Println("  j, jump <step>          - Jump to specific step")
	fmt.Println()
	fmt.Println("Display:")
	fmt.Println("  s, show, state          - Show current state")
	fmt.Println("  S                       - Toggle hiding/showing Rust core::* traces")
	fmt.Println("  r, reconstruct [step]   - Reconstruct state")
	fmt.Println("  t, trap                 - Show trap info with local variables")
	fmt.Println("  l, list [count]         - List steps (default: 10)")
	fmt.Println("  i, info                 - Show navigation info")
	fmt.Println("  sp, split               - Split-pane trace and source view")
	fmt.Println("  e, expand               - Expand current node")
	fmt.Println("  c, collapse             - Collapse current node")
	fmt.Println("  E                       - Toggle expand/collapse all")
	fmt.Println()
	fmt.Println("Filter:")
	fmt.Println("  f, filter               - Cycle filter by event type (trap, contract_call, host_function, auth)")
	fmt.Println()
	fmt.Println("Search:")
	fmt.Println("  /                       - Start search")
	fmt.Println("  n                       - Next search match")
	fmt.Println("  N                       - Previous search match")
	fmt.Println("  ESC                     - Clear search / cancel input")
	fmt.Println()
	fmt.Println("Other:")
	fmt.Println("  h, help              - Show this help")
	fmt.Println("  y, yank <a/r> [idx]  - Copy raw XDR (a: arg, r: return)")
	fmt.Println("  q, quit, exit        - Exit viewer")
	fmt.Println("  ?, h, help              - Show this help")
	fmt.Println("  q, quit, exit           - Exit viewer")
}

// handleYank copies raw XDR values to the clipboard
func (v *InteractiveViewer) handleYank(args []string) {
	state, err := v.trace.GetCurrentState()
	if err != nil {
		fmt.Printf("%s %s\n", visualizer.Error(), err)
		return
	}

	subcmd := strings.ToLower(args[0])
	var value string

	switch subcmd {
	case "a", "arg", "argument":
		index := 0
		if len(args) > 1 {
			index, err = strconv.Atoi(args[1])
			if err != nil {
				fmt.Printf("%s Invalid argument index: %s\n", visualizer.Error(), args[1])
				return
			}
		}

		if index < 0 || index >= len(state.RawArguments) {
			fmt.Printf("%s Argument index %d out of bounds (0-%d)\n",
				visualizer.Error(), index, len(state.RawArguments)-1)
			return
		}
		value = state.RawArguments[index]

	case "r", "ret", "return":
		if state.RawReturnValue == "" {
			fmt.Printf("%s No raw return value available at this step\n", visualizer.Error())
			return
		}
		value = state.RawReturnValue

	default:
		fmt.Printf("%s Unknown yank subcommand: %s. Use 'a' for arguments or 'r' for return value.\n",
			visualizer.Error(), subcmd)
		return
	}

	if err := clipboard.WriteAll(value); err != nil {
		fmt.Printf("%s Failed to copy to clipboard: %v\n", visualizer.Error(), err)
		// Fallback: just print it so the user can see it
		fmt.Printf("Raw XDR: %s\n", value)
		return
	}

	fmt.Printf("%s Copied raw XDR to clipboard\n", visualizer.Symbol("sparkles"))
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
