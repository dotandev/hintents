package trace

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// InteractiveViewer provides a terminal-based interactive trace navigation interface
type InteractiveViewer struct {
	trace  *ExecutionTrace
	reader *bufio.Reader
}

// NewInteractiveViewer creates a new interactive trace viewer
func NewInteractiveViewer(trace *ExecutionTrace) *InteractiveViewer {
	return &InteractiveViewer{
		trace:  trace,
		reader: bufio.NewReader(os.Stdin),
	}
}

// Start begins the interactive trace viewing session
func (v *InteractiveViewer) Start() error {
	fmt.Println("🔍 ERST Interactive Trace Viewer")
	fmt.Println("=================================")
	fmt.Printf("Transaction: %s\n", v.trace.TransactionHash)
	fmt.Printf("Total Steps: %d\n\n", len(v.trace.States))

	v.showHelp()
	v.displayCurrentState()

	for {
		fmt.Print("\n> ")
		input, err := v.reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		command := strings.TrimSpace(input)
		if command == "" {
			continue
		}

		if v.handleCommand(command) {
			break // Exit requested
		}
	}

	return nil
}

// handleCommand processes user commands and returns true if exit is requested
func (v *InteractiveViewer) handleCommand(command string) bool {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "n", "next", "forward":
		v.stepForward()
	case "p", "prev", "back", "backward":
		v.stepBackward()
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
	case "i", "info":
		v.showNavigationInfo()
	case "l", "list":
		if len(parts) > 1 {
			v.listSteps(parts[1])
		} else {
			v.listSteps("10")
		}
	case "h", "help":
		v.showHelp()
	case "q", "quit", "exit":
		fmt.Println("Goodbye! 👋")
		return true
	default:
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", cmd)
	}

	return false
}

// stepForward moves to the next step
func (v *InteractiveViewer) stepForward() {
	state, err := v.trace.StepForward()
	if err != nil {
		fmt.Printf("❌ %s\n", err)
		return
	}

	fmt.Printf("➡️  Stepped forward to step %d\n", state.Step)
	v.displayCurrentState()
}

// stepBackward moves to the previous step
func (v *InteractiveViewer) stepBackward() {
	state, err := v.trace.StepBackward()
	if err != nil {
		fmt.Printf("❌ %s\n", err)
		return
	}

	fmt.Printf("⬅️  Stepped backward to step %d\n", state.Step)
	v.displayCurrentState()
}

// jumpToStep jumps to a specific step
func (v *InteractiveViewer) jumpToStep(stepStr string) {
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		fmt.Printf("❌ Invalid step number: %s\n", stepStr)
		return
	}

	state, err := v.trace.JumpToStep(step)
	if err != nil {
		fmt.Printf("❌ %s\n", err)
		return
	}

	fmt.Printf("🎯 Jumped to step %d\n", state.Step)
	v.displayCurrentState()
}

// displayCurrentState shows the current execution state
func (v *InteractiveViewer) displayCurrentState() {
	state, err := v.trace.GetCurrentState()
	if err != nil {
		fmt.Printf("❌ %s\n", err)
		return
	}

	fmt.Println("\n📍 Current State")
	fmt.Println("================")
	fmt.Printf("Step: %d/%d\n", state.Step, len(v.trace.States)-1)
	fmt.Printf("Time: %s\n", state.Timestamp.Format("15:04:05.000"))
	fmt.Printf("Operation: %s\n", state.Operation)

	if state.ContractID != "" {
		fmt.Printf("Contract: %s\n", state.ContractID)
	}
	if state.Function != "" {
		fmt.Printf("Function: %s\n", state.Function)
	}
	if len(state.Arguments) > 0 {
		fmt.Printf("Arguments: %v\n", state.Arguments)
	}
	if state.ReturnValue != nil {
		fmt.Printf("Return: %v\n", state.ReturnValue)
	}
	if state.Error != "" {
		fmt.Printf("❌ Error: %s\n", state.Error)
	}

	// Show memory/state summary
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
		fmt.Printf("❌ Failed to reconstruct state: %s\n", err)
		return
	}

	fmt.Println("\n🔧 Reconstructed State")
	fmt.Println("======================")
	v.displayState(state)
}

// reconstructState reconstructs and displays state at a specific step
func (v *InteractiveViewer) reconstructState(stepStr string) {
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		fmt.Printf("❌ Invalid step number: %s\n", stepStr)
		return
	}

	state, err := v.trace.ReconstructStateAt(step)
	if err != nil {
		fmt.Printf("❌ Failed to reconstruct state: %s\n", err)
		return
	}

	fmt.Printf("\n🔧 Reconstructed State at Step %d\n", step)
	fmt.Println("==================================")
	v.displayState(state)
}

// displayState displays a complete state
func (v *InteractiveViewer) displayState(state *ExecutionState) {
	fmt.Printf("Step: %d\n", state.Step)
	fmt.Printf("Time: %s\n", state.Timestamp.Format("15:04:05.000"))
	fmt.Printf("Operation: %s\n", state.Operation)

	if state.ContractID != "" {
		fmt.Printf("Contract: %s\n", state.ContractID)
	}
	if state.Function != "" {
		fmt.Printf("Function: %s\n", state.Function)
	}

	if len(state.HostState) > 0 {
		fmt.Println("\nHost State:")
		for k, v := range state.HostState {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	if len(state.Memory) > 0 {
		fmt.Println("\nMemory:")
		for k, v := range state.Memory {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}
}

// showNavigationInfo displays navigation information
func (v *InteractiveViewer) showNavigationInfo() {
	info := v.trace.GetNavigationInfo()

	fmt.Println("\n📊 Navigation Info")
	fmt.Println("==================")
	fmt.Printf("Total Steps: %d\n", info["total_steps"])
	fmt.Printf("Current Step: %d\n", info["current_step"])
	fmt.Printf("Can Step Back: %t\n", info["can_step_back"])
	fmt.Printf("Can Step Forward: %t\n", info["can_step_forward"])
	fmt.Printf("Snapshots: %d\n", info["snapshots_count"])
}

// listSteps shows a list of steps around the current position
func (v *InteractiveViewer) listSteps(countStr string) {
	count, err := strconv.Atoi(countStr)
	if err != nil {
		count = 10
	}

	current := v.trace.CurrentStep
	start := max(0, current-count/2)
	end := min(len(v.trace.States)-1, start+count-1)

	fmt.Printf("\n📋 Steps %d-%d\n", start, end)
	fmt.Println("===============")

	for i := start; i <= end; i++ {
		state := &v.trace.States[i]
		marker := "  "
		if i == current {
			marker = "▶️"
		}

		fmt.Printf("%s %3d: %s", marker, i, state.Operation)
		if state.Function != "" {
			fmt.Printf(" (%s)", state.Function)
		}
		if state.Error != "" {
			fmt.Printf(" ❌")
		}
		fmt.Println()
	}
}

// showHelp displays available commands
func (v *InteractiveViewer) showHelp() {
	fmt.Println("\n📖 Available Commands")
	fmt.Println("=====================")
	fmt.Println("Navigation:")
	fmt.Println("  n, next, forward     - Step forward")
	fmt.Println("  p, prev, back        - Step backward")
	fmt.Println("  j, jump <step>       - Jump to specific step")
	fmt.Println()
	fmt.Println("Display:")
	fmt.Println("  s, show, state       - Show current state")
	fmt.Println("  r, reconstruct [step] - Reconstruct state")
	fmt.Println("  l, list [count]      - List steps (default: 10)")
	fmt.Println("  i, info              - Show navigation info")
	fmt.Println()
	fmt.Println("Other:")
	fmt.Println("  h, help              - Show this help")
	fmt.Println("  q, quit, exit        - Exit viewer")
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
