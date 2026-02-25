// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/dwarf"
	"github.com/dotandev/hintents/internal/visualizer"
)

// TrapType categorizes different types of traps/errors
type TrapType string

const (
	TrapMemoryOutOfBounds TrapType = "memory_out_of_bounds"
	TrapIndexOutOfBounds  TrapType = "index_out_of_bounds"
	TrapDivisionByZero    TrapType = "division_by_zero"
	TrapOverflow          TrapType = "overflow"
	TrapUnderflow         TrapType = "underflow"
	TrapPanic             TrapType = "panic"
	TrapUnknown           TrapType = "unknown"
)

// TrapInfo contains information about a trap that occurred during execution
type TrapInfo struct {
	Type           TrapType              // Type of trap
	Message        string                // Error message
	SourceLocation *dwarf.SourceLocation // Source location if available
	LocalVars      []LocalVarInfo        // Local variables at trap point
	Function       string                // Function where trap occurred
	CallStack      []string              // Call stack at trap point
}

// LocalVarInfo represents a local variable with its value at trap time
type LocalVarInfo struct {
	Name           string                // Variable name
	DemangledName  string                // Demangled name for display
	Type           string                // Variable type
	Value          interface{}           // Value at trap time (if available)
	Location       string                // Memory location
	SourceLocation *dwarf.SourceLocation // Where in source the variable is defined
}

// TrapDetector detects and analyzes traps in execution traces
type TrapDetector struct {
	dwarfParser *dwarf.Parser
	wasmData    []byte
}

// NewTrapDetector creates a new trap detector
func NewTrapDetector(wasmData []byte) (*TrapDetector, error) {
	td := &TrapDetector{
		wasmData: wasmData,
	}

	// Try to parse DWARF info if WASM data is provided
	if len(wasmData) > 0 {
		parser, err := dwarf.NewParser(wasmData)
		if err == nil && parser.HasDebugInfo() {
			td.dwarfParser = parser
		}
	}

	return td, nil
}

// DetectTrap detects if the given state represents a trap
func (td *TrapDetector) DetectTrap(state *ExecutionState) *TrapInfo {
	if state == nil || state.Error == "" {
		return nil
	}

	errorMsg := strings.ToLower(state.Error)
	trapType := td.identifyTrapType(errorMsg)

	trap := &TrapInfo{
		Type:    trapType,
		Message: state.Error,
	}

	// Extract function information
	if state.Function != "" {
		trap.Function = state.Function
	}

	// Try to get source location from DWARF
	if td.dwarfParser != nil && state.Arguments != nil {
		// Use function address if available (would need to be extracted from trace)
		// For now, we'll try to find the subprogram based on the function name
		subprograms, _ := td.dwarfParser.GetSubprograms()
		for _, sp := range subprograms {
			if strings.Contains(sp.Name, state.Function) || strings.Contains(sp.DemangledName, state.Function) {
				trap.SourceLocation = &dwarf.SourceLocation{
					File: sp.File,
					Line: sp.Line,
				}
				trap.LocalVars = td.extractLocalVars(&sp)
				break
			}
		}
	}

	return trap
}

// identifyTrapType identifies the type of trap from the error message
func (td *TrapDetector) identifyTrapType(errorMsg string) TrapType {
	// Memory out of bounds patterns
	memOutOfBoundsPatterns := []string{
		"memory out of bounds",
		"out of bounds memory access",
		"failed to access memory",
		"memory access out of bounds",
	}
	for _, pattern := range memOutOfBoundsPatterns {
		if strings.Contains(errorMsg, pattern) {
			return TrapMemoryOutOfBounds
		}
	}

	// Index out of bounds patterns
	indexOutOfBoundsPatterns := []string{
		"index out of bounds",
		"index out of range",
		"array index out of bounds",
		"vec index out of bounds",
		"slice index out of bounds",
	}
	for _, pattern := range indexOutOfBoundsPatterns {
		if strings.Contains(errorMsg, pattern) {
			return TrapIndexOutOfBounds
		}
	}

	// Division by zero
	if strings.Contains(errorMsg, "division by zero") || strings.Contains(errorMsg, "divide by zero") {
		return TrapDivisionByZero
	}

	// Overflow patterns
	overflowPatterns := []string{
		"overflow",
		"arithmetic overflow",
		"integer overflow",
		"attempt to add with overflow",
		"attempt to multiply with overflow",
	}
	for _, pattern := range overflowPatterns {
		if strings.Contains(errorMsg, pattern) {
			return TrapOverflow
		}
	}

	// Underflow patterns
	underflowPatterns := []string{
		"underflow",
		"arithmetic underflow",
		"attempt to subtract with overflow", // Rust calls underflow "subtract with overflow"
	}
	for _, pattern := range underflowPatterns {
		if strings.Contains(errorMsg, pattern) {
			return TrapUnderflow
		}
	}

	// Panic patterns (generic)
	if strings.Contains(errorMsg, "panic") || strings.Contains(errorMsg, "trap") {
		return TrapPanic
	}

	return TrapUnknown
}

// extractLocalVars extracts local variable information from a subprogram
func (td *TrapDetector) extractLocalVars(sp *dwarf.SubprogramInfo) []LocalVarInfo {
	var vars []LocalVarInfo

	for _, lv := range sp.LocalVariables {
		info := LocalVarInfo{
			Name:          lv.Name,
			DemangledName: lv.DemangledName,
			Type:          lv.Type,
			Location:      lv.Location,
		}

		// Try to get source location
		if lv.StartLine > 0 {
			info.SourceLocation = &dwarf.SourceLocation{
				Line: lv.StartLine,
			}
		}

		vars = append(vars, info)
	}

	return vars
}

// FindTrapPoint finds the step where a trap occurred in the trace
func (td *TrapDetector) FindTrapPoint(trace *ExecutionTrace) *TrapInfo {
	for i := range trace.States {
		state := &trace.States[i]
		if state.Error != "" {
			// Check if this is a trap-like error
			if td.identifyTrapType(strings.ToLower(state.Error)) != TrapUnknown {
				// Found a trap, analyze it
				trap := td.DetectTrap(state)
				trap.CallStack = td.extractCallStack(trace, i)
				return trap
			}
		}
	}

	return nil
}

// extractCallStack extracts the call stack at a given step
func (td *TrapDetector) extractCallStack(trace *ExecutionTrace, step int) []string {
	var stack []string

	for i := 0; i <= step && i < len(trace.States); i++ {
		state := &trace.States[i]
		if state.Function != "" {
			entry := state.Function
			if state.ContractID != "" {
				entry = state.ContractID + "::" + state.Function
			}
			// Avoid duplicates
			if len(stack) == 0 || stack[len(stack)-1] != entry {
				stack = append(stack, entry)
			}
		}
	}

	return stack
}

// FormatTrapInfo formats trap information for display
func FormatTrapInfo(trap *TrapInfo) string {
	var sb strings.Builder

	// Header
	sb.WriteString(visualizer.Error() + " ")
	sb.WriteString("Trap Detected: ")
	sb.WriteString(string(trap.Type))
	sb.WriteString("\n")

	// Error message
	sb.WriteString("\n" + visualizer.Symbol("warn") + " Error: ")
	sb.WriteString(trap.Message)
	sb.WriteString("\n")

	// Source location
	if trap.SourceLocation != nil {
		sb.WriteString("\n" + visualizer.Symbol("pin") + " Location: ")
		if trap.SourceLocation.File != "" {
			sb.WriteString(trap.SourceLocation.File)
			sb.WriteString(":")
		}
		sb.WriteString(fmt.Sprintf("%d", trap.SourceLocation.Line))
		sb.WriteString("\n")
	}

	// Function
	if trap.Function != "" {
		sb.WriteString("\n" + visualizer.Symbol("wrench") + " Function: ")
		sb.WriteString(trap.Function)
		sb.WriteString("\n")
	}

	// Local variables
	if len(trap.LocalVars) > 0 {
		sb.WriteString("\n" + visualizer.Symbol("list") + " Local Variables at Trap Point:\n")
		for _, v := range trap.LocalVars {
			sb.WriteString("  - ")
			sb.WriteString(v.DemangledName)
			sb.WriteString(": ")
			sb.WriteString(v.Type)
			if v.Location != "" {
				sb.WriteString(" @ ")
				sb.WriteString(v.Location)
			}
			if v.Value != nil {
				sb.WriteString(" = ")
				sb.WriteString(formatVarValue(v.Value))
			}
			sb.WriteString("\n")
		}
	}

	// Call stack
	if len(trap.CallStack) > 0 {
		sb.WriteString("\n" + visualizer.Symbol("list") + " Call Stack:\n")
		for i, frame := range trap.CallStack {
			sb.WriteString("  ")
			sb.WriteString(fmt.Sprintf("%d", i))
			sb.WriteString(": ")
			sb.WriteString(frame)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// formatVarValue formats a variable value for display
func formatVarValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return `"` + val + `"`
	case int, int8, int16, int32, int64:
		return formatInt(val)
	case uint, uint8, uint16, uint32, uint64:
		return formatUint(val)
	case float32, float64:
		return formatFloat(val)
	default:
		return toString(val)
	}
}

func formatInt(v interface{}) string {
	switch val := v.(type) {
	case int:
		return toString(val)
	case int8:
		return toString(val)
	case int16:
		return toString(val)
	case int32:
		return toString(val)
	case int64:
		return toString(val)
	default:
		return "<unknown int>"
	}
}

func formatUint(v interface{}) string {
	switch val := v.(type) {
	case uint:
		return toString(val)
	case uint8:
		return toString(val)
	case uint16:
		return toString(val)
	case uint32:
		return toString(val)
	case uint64:
		return toString(val)
	default:
		return "<unknown uint>"
	}
}

func formatFloat(v interface{}) string {
	switch val := v.(type) {
	case float32:
		return toString(val)
	case float64:
		return toString(val)
	default:
		return "<unknown float>"
	}
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return "<complex>"
	}
}

// IsMemoryTrap checks if a trap is memory-related
func IsMemoryTrap(trap *TrapInfo) bool {
	if trap == nil {
		return false
	}
	return trap.Type == TrapMemoryOutOfBounds || trap.Type == TrapIndexOutOfBounds
}
