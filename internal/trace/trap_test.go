// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/dwarf"
)

func TestIdentifyTrapType(t *testing.T) {
	td := &TrapDetector{}

	tests := []struct {
		name     string
		errorMsg string
		want     TrapType
	}{
		{
			name:     "memory out of bounds",
			errorMsg: "memory out of bounds",
			want:     TrapMemoryOutOfBounds,
		},
		{
			name:     "index out of bounds",
			errorMsg: "index out of bounds",
			want:     TrapIndexOutOfBounds,
		},
		{
			name:     "array index out of bounds",
			errorMsg: "array index out of bounds",
			want:     TrapIndexOutOfBounds,
		},
		{
			name:     "division by zero",
			errorMsg: "division by zero",
			want:     TrapDivisionByZero,
		},
		{
			name:     "divide by zero",
			errorMsg: "divide by zero",
			want:     TrapDivisionByZero,
		},
		{
			name:     "integer overflow",
			errorMsg: "integer overflow",
			want:     TrapOverflow,
		},
		{
			name:     "arithmetic overflow",
			errorMsg: "arithmetic overflow",
			want:     TrapOverflow,
		},
		{
			name:     "underflow",
			errorMsg: "underflow",
			want:     TrapUnderflow,
		},
		{
			name:     "panic",
			errorMsg: "contract panicked",
			want:     TrapPanic,
		},
		{
			name:     "trap",
			errorMsg: "contract trapped",
			want:     TrapPanic,
		},
		{
			name:     "unknown error",
			errorMsg: "something went wrong",
			want:     TrapUnknown,
		},
		{
			name:     "empty error",
			errorMsg: "",
			want:     TrapUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := td.identifyTrapType(tt.errorMsg)
			if got != tt.want {
				t.Errorf("identifyTrapType(%q) = %v, want %v", tt.errorMsg, got, tt.want)
			}
		})
	}
}

// TestDetectTrap tests trap detection
func TestDetectTrap(t *testing.T) {
	tests := []struct {
		name    string
		state   *ExecutionState
		wantNil bool
	}{
		{
			name:    "nil state",
			state:   nil,
			wantNil: true,
		},
		{
			name: "empty error",
			state: &ExecutionState{
				Error: "",
			},
			wantNil: true,
		},
		{
			name: "memory out of bounds error",
			state: &ExecutionState{
				Error:    "memory out of bounds at address 0x1000",
				Function: "transfer",
			},
			wantNil: false,
		},
		{
			name: "index out of bounds error",
			state: &ExecutionState{
				Error:    "index out of bounds: len=5, index=10",
				Function: "get_balance",
			},
			wantNil: false,
		},
		{
			name: "unknown error",
			state: &ExecutionState{
				Error: "some other error",
			},
			wantNil: false, // Will detect as unknown trap type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := &TrapDetector{}
			got := td.DetectTrap(tt.state)
			if tt.wantNil && got != nil {
				t.Errorf("DetectTrap() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("DetectTrap() = nil, want non-nil")
			}
		})
	}
}

// TestFindTrapPoint tests finding the trap point in a trace
func TestFindTrapPoint(t *testing.T) {
	td := &TrapDetector{}

	// Create a trace with a trap
	trace := &ExecutionTrace{
		States: []ExecutionState{
			{
				Step:      0,
				Timestamp: time.Now(),
				Operation: "call",
				Function:  "init",
			},
			{
				Step:      1,
				Timestamp: time.Now(),
				Operation: "call",
				Function:  "transfer",
			},
			{
				Step:      2,
				Timestamp: time.Now(),
				Operation: "error",
				Function:  "transfer",
				Error:     "index out of bounds: len=5, index=10",
			},
		},
	}

	trap := td.FindTrapPoint(trace)
	if trap == nil {
		t.Fatal("Expected to find trap point, got nil")
	}

	if trap.Type != TrapIndexOutOfBounds {
		t.Errorf("Expected trap type %v, got %v", TrapIndexOutOfBounds, trap.Type)
	}

	if trap.Function != "transfer" {
		t.Errorf("Expected function 'transfer', got %q", trap.Function)
	}
}

// TestExtractCallStack tests call stack extraction
func TestExtractCallStack(t *testing.T) {
	td := &TrapDetector{}

	trace := &ExecutionTrace{
		States: []ExecutionState{
			{
				Step:       0,
				Operation:  "call",
				Function:   "init",
				ContractID: "CAAAA...",
			},
			{
				Step:       1,
				Operation:  "call",
				Function:   "transfer",
				ContractID: "CAAAA...",
			},
			{
				Step:       2,
				Operation:  "call",
				Function:   "inner",
				ContractID: "CAAAA...",
			},
			{
				Step:       3,
				Operation:  "error",
				Function:   "inner",
				ContractID: "CAAAA...",
				Error:      "index out of bounds",
			},
		},
	}

	stack := td.extractCallStack(trace, 3)

	// Should have 3 frames
	if len(stack) != 3 {
		t.Errorf("Expected 3 stack frames, got %d", len(stack))
	}

	// Check that it contains expected functions
	if len(stack) >= 2 {
		if stack[1] != "CAAAA...::transfer" {
			t.Errorf("Expected second frame to be 'transfer', got %q", stack[1])
		}
	}
}

// TestFormatTrapInfo tests trap info formatting
func TestFormatTrapInfo(t *testing.T) {
	trap := &TrapInfo{
		Type:     TrapIndexOutOfBounds,
		Message:  "index out of bounds: len=5, index=10",
		Function: "transfer",
		SourceLocation: &dwarf.SourceLocation{
			File: "token.rs",
			Line: 45,
		},
		LocalVars: []LocalVarInfo{
			{
				Name:          "balance",
				DemangledName: "balance",
				Type:          "i128",
				Location:      "0x1000",
			},
			{
				Name:          "amount",
				DemangledName: "amount",
				Type:          "u64",
				Location:      "0x1008",
			},
		},
		CallStack: []string{
			"init",
			"transfer",
			"inner",
		},
	}

	output := FormatTrapInfo(trap)

	// Check for expected content
	expected := []string{
		"index_out_of_bounds",
		"index out of bounds",
		"token.rs",
		"transfer",
		"balance",
		"amount",
	}

	for _, exp := range expected {
		if !contains(output, exp) {
			t.Errorf("Expected output to contain %q", exp)
		}
	}
}

// TestIsMemoryTrap tests memory trap detection
func TestIsMemoryTrap(t *testing.T) {
	tests := []struct {
		name string
		trap *TrapInfo
		want bool
	}{
		{
			name: "nil trap",
			trap: nil,
			want: false,
		},
		{
			name: "memory out of bounds",
			trap: &TrapInfo{
				Type: TrapMemoryOutOfBounds,
			},
			want: true,
		},
		{
			name: "index out of bounds",
			trap: &TrapInfo{
				Type: TrapIndexOutOfBounds,
			},
			want: true,
		},
		{
			name: "overflow",
			trap: &TrapInfo{
				Type: TrapOverflow,
			},
			want: false,
		},
		{
			name: "panic",
			trap: &TrapInfo{
				Type: TrapPanic,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMemoryTrap(tt.trap)
			if got != tt.want {
				t.Errorf("IsMemoryTrap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLocalVarInfo tests LocalVarInfo struct
func TestLocalVarInfo(t *testing.T) {
	lv := LocalVarInfo{
		Name:          "_RNv5balance",
		DemangledName: "balance",
		Type:          "i128",
		Location:      "0x1000",
		Value:         int64(1000),
		SourceLocation: &dwarf.SourceLocation{
			File: "token.rs",
			Line: 20,
		},
	}

	if lv.DemangledName != "balance" {
		t.Errorf("Expected demangled name 'balance', got %q", lv.DemangledName)
	}

	if lv.Type != "i128" {
		t.Errorf("Expected type 'i128', got %q", lv.Type)
	}

	if lv.SourceLocation.Line != 20 {
		t.Errorf("Expected line 20, got %d", lv.SourceLocation.Line)
	}
}

// TestNewTrapDetector tests trap detector creation
func TestNewTrapDetector(t *testing.T) {
	td, err := NewTrapDetector(nil)
	if err != nil {
		t.Errorf("NewTrapDetector() error = %v", err)
	}
	if td == nil {
		t.Error("Expected non-nil TrapDetector")
	}

	// Test with empty WASM (should not panic)
	td2, err2 := NewTrapDetector([]byte{})
	if err2 != nil {
		t.Errorf("NewTrapDetector() with empty bytes error = %v", err2)
	}
	_ = td2
}

// =============================================================================
// Inlined function handling tests
// =============================================================================

// TestInlinedFrame tests the InlinedFrame struct.
func TestInlinedFrame(t *testing.T) {
	frame := InlinedFrame{
		Function: "check_balance",
		CallSite: SourceLocation{
			File:   "token.rs",
			Line:   42,
			Column: 5,
		},
		InlinedAt: SourceLocation{
			File:   "balance.rs",
			Line:   10,
		},
	}

	if frame.Function != "check_balance" {
		t.Errorf("Function = %q, want %q", frame.Function, "check_balance")
	}
	if frame.CallSite.File != "token.rs" {
		t.Errorf("CallSite.File = %q, want %q", frame.CallSite.File, "token.rs")
	}
	if frame.CallSite.Line != 42 {
		t.Errorf("CallSite.Line = %d, want 42", frame.CallSite.Line)
	}
	if frame.InlinedAt.File != "balance.rs" {
		t.Errorf("InlinedAt.File = %q, want %q", frame.InlinedAt.File, "balance.rs")
	}
}

// TestFormatTrapInfo_WithInlinedChain verifies that FormatTrapInfo includes the
// inlined call chain section when TrapInfo.InlinedChain is populated.
func TestFormatTrapInfo_WithInlinedChain(t *testing.T) {
	trap := &TrapInfo{
		Type:    TrapIndexOutOfBounds,
		Message: "index out of bounds: len=5, index=10",
		Function: "check_balance",
		SourceLocation: &SourceLocation{
			File: "balance.rs",
			Line: 10,
		},
		InlinedChain: []InlinedFrame{
			{
				Function: "transfer",
				CallSite: SourceLocation{File: "token.rs", Line: 55},
				InlinedAt: SourceLocation{File: "balance.rs", Line: 10},
			},
			{
				Function: "check_balance",
				CallSite: SourceLocation{File: "balance.rs", Line: 10},
				InlinedAt: SourceLocation{File: "balance.rs", Line: 22},
			},
		},
	}

	output := FormatTrapInfo(trap)

	// The output must mention the inlined chain section header.
	if !contains(output, "Inlined Call Chain") {
		t.Error("expected output to contain 'Inlined Call Chain'")
	}
	// Both function names must appear.
	if !contains(output, "transfer") {
		t.Error("expected output to contain 'transfer'")
	}
	if !contains(output, "check_balance") {
		t.Error("expected output to contain 'check_balance'")
	}
	// Call-site file must appear.
	if !contains(output, "token.rs") {
		t.Error("expected output to contain 'token.rs'")
	}
	// Inlined-at file must appear.
	if !contains(output, "balance.rs") {
		t.Error("expected output to contain 'balance.rs'")
	}
}

// TestFormatTrapInfo_NoInlinedChain verifies that FormatTrapInfo does not emit
// the inlined chain section when InlinedChain is empty.
func TestFormatTrapInfo_NoInlinedChain(t *testing.T) {
	trap := &TrapInfo{
		Type:    TrapPanic,
		Message: "contract panicked",
		Function: "transfer",
	}

	output := FormatTrapInfo(trap)

	if contains(output, "Inlined Call Chain") {
		t.Error("expected output NOT to contain 'Inlined Call Chain' when chain is empty")
	}
}

// TestResolveInlinedChain_NoDwarfParser verifies that resolveInlinedChain
// returns without modifying the trap when no DWARF parser is set.
func TestResolveInlinedChain_NoDwarfParser(t *testing.T) {
	td := &TrapDetector{dwarfParser: nil}

	trap := &TrapInfo{
		Type:     TrapPanic,
		Message:  "panic",
		Function: "transfer",
		SourceLocation: &SourceLocation{
			File: "token.rs",
			Line: 10,
		},
	}
	originalFunc := trap.Function
	originalFile := trap.SourceLocation.File

	td.resolveInlinedChain(trap, 0x1000, nil)

	if trap.Function != originalFunc {
		t.Errorf("Function changed unexpectedly: %q -> %q", originalFunc, trap.Function)
	}
	if trap.SourceLocation.File != originalFile {
		t.Errorf("SourceLocation.File changed unexpectedly")
	}
	if len(trap.InlinedChain) != 0 {
		t.Errorf("expected empty InlinedChain, got %d frames", len(trap.InlinedChain))
	}
}

// TestTrapInfo_InlinedChainFieldExists ensures TrapInfo has the InlinedChain
// field and that it defaults to nil (not allocated) for zero-value structs.
func TestTrapInfo_InlinedChainFieldExists(t *testing.T) {
	var trap TrapInfo
	if trap.InlinedChain != nil {
		t.Error("expected nil InlinedChain for zero-value TrapInfo")
	}

	trap.InlinedChain = []InlinedFrame{
		{Function: "foo"},
	}
	if len(trap.InlinedChain) != 1 {
		t.Errorf("expected 1 frame, got %d", len(trap.InlinedChain))
	}
}

// TestDetectTrap_FunctionUpdatedFromInlined verifies that DetectTrap does not
// crash when called without DWARF data and still returns a populated TrapInfo.
func TestDetectTrap_FunctionUpdatedFromInlined(t *testing.T) {
	td := &TrapDetector{dwarfParser: nil}

	state := &ExecutionState{
		Error:    "index out of bounds: len=3, index=5",
		Function: "swap",
	}

	trap := td.DetectTrap(state)
	if trap == nil {
		t.Fatal("expected non-nil TrapInfo")
	}
	if trap.Function != "swap" {
		t.Errorf("Function = %q, want %q", trap.Function, "swap")
	}
	if trap.Type != TrapIndexOutOfBounds {
		t.Errorf("Type = %v, want %v", trap.Type, TrapIndexOutOfBounds)
	}
	// Without DWARF data no inlined chain can be resolved.
	if len(trap.InlinedChain) != 0 {
		t.Errorf("expected empty InlinedChain without DWARF data, got %d frames", len(trap.InlinedChain))
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
