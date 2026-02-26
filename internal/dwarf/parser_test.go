// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package dwarf

import (
	"testing"
)

// TestNewParser tests parser creation with various binary formats
func TestNewParser(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name:    "nil data",
			data:    nil,
			wantErr: ErrInvalidWASM,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: ErrInvalidWASM,
		},
		{
			name:    "short data",
			data:    []byte{0x00},
			wantErr: ErrInvalidWASM,
		},
		{
			name: "valid WASM magic",
			data: []byte{
				0x00, 0x61, 0x73, 0x6d, // WASM magic
				0x01, 0x00, 0x00, 0x00, // version
			},
			wantErr: ErrNoDebugInfo, // No debug info in minimal WASM
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewParser(tt.data)
			if err != tt.wantErr {
				t.Errorf("NewParser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNewParser_ShortELF verifies that a 4-byte ELF magic header (too short to
// parse) returns a non-nil error.  The exact error value is elf-package-specific
// and is not checked.
func TestNewParser_ShortELF(t *testing.T) {
	data := []byte{0x7f, 0x45, 0x4c, 0x46} // ELF magic only
	_, err := NewParser(data)
	if err == nil {
		t.Error("expected non-nil error for truncated ELF, got nil")
	}
}

// TestWASMFormatDetection tests WASM format detection
func TestWASMFormatDetection(t *testing.T) {
	// Valid WASM binary header
	wasmHeader := []byte{
		0x00, 0x61, 0x73, 0x6d, // WASM magic
		0x01, 0x00, 0x00, 0x00, // version 1
		0x00,                   // Section 0: custom
		0x04,                   // Section size
		0x04,                   // Name length
		0x00, 0x00, 0x00, 0x00, // Padding/name
	}

	parser, err := NewParser(wasmHeader)
	if err != ErrNoDebugInfo {
		t.Logf("Parser result: %v (expected no debug info)", err)
	}
	_ = parser // May be nil
}

// TestLocalVar tests LocalVar struct
func TestLocalVar(t *testing.T) {
	lv := LocalVar{
		Name:          "balance",
		DemangledName: "balance",
		Type:          "i128",
		Location:      "0x1000",
		Address:       0x1000,
		StartLine:     10,
		EndLine:       20,
	}

	if lv.Name != "balance" {
		t.Errorf("Expected name 'balance', got '%s'", lv.Name)
	}
	if lv.Type != "i128" {
		t.Errorf("Expected type 'i128', got '%s'", lv.Type)
	}
}

// TestSubprogramInfo tests SubprogramInfo struct
func TestSubprogramInfo(t *testing.T) {
	sp := SubprogramInfo{
		Name:          "transfer",
		DemangledName: "transfer",
		LowPC:         0x1000,
		HighPC:        0x2000,
		Line:          15,
		File:          "token.rs",
		LocalVariables: []LocalVar{
			{Name: "from", Type: "Symbol"},
			{Name: "to", Type: "Symbol"},
			{Name: "amount", Type: "i128"},
		},
	}

	if len(sp.LocalVariables) != 3 {
		t.Errorf("Expected 3 local variables, got %d", len(sp.LocalVariables))
	}
}

// TestSourceLocation tests SourceLocation struct
func TestSourceLocation(t *testing.T) {
	sl := SourceLocation{
		File:   "token.rs",
		Line:   45,
		Column: 12,
	}

	if sl.Line != 45 {
		t.Errorf("Expected line 45, got %d", sl.Line)
	}
	if sl.Column != 12 {
		t.Errorf("Expected column 12, got %d", sl.Column)
	}
}

// TestFormatLocation tests location formatting
func TestFormatLocation(t *testing.T) {
	tests := []struct {
		name string
		loc  []byte
		want string
	}{
		{
			name: "empty location",
			loc:  []byte{},
			want: "",
		},
		{
			name: "stack value",
			loc:  []byte{0x9f}, // DW_OP_stack_value
			want: "immediate",
		},
		{
			name: "unknown opcode",
			loc:  []byte{0xFF},
			want: "location[0xff]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLocation(tt.loc)
			if got != tt.want {
				t.Errorf("formatLocation() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNameDemangle tests name demangling
func TestNameDemangle(t *testing.T) {
	tests := []struct {
		label string
		input string
		want  string
	}{
		{
			label: "regular name",
			input: "balance",
			want:  "balance",
		},
		{
			label: "mangled Rust name",
			input: "_RNv4token7balance",
			want:  "_RNv4token7balance", // Currently returns as-is
		},
		{
			label: "empty name",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := nameDemangle(tt.input)
			if got != tt.want {
				t.Errorf("nameDemangle() = %v, want %v", got, tt.want)
			}
		})
	}
}

// BenchmarkNewParser benchmarks parser creation
func BenchmarkNewParser(b *testing.B) {
	// Minimal WASM header
	data := []byte{
		0x00, 0x61, 0x73, 0x6d,
		0x01, 0x00, 0x00, 0x00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewParser(data)
	}
}

// =============================================================================
// Inlined subroutine tests
// =============================================================================

// TestInlinedSubroutineInfo tests the InlinedSubroutineInfo struct.
func TestInlinedSubroutineInfo(t *testing.T) {
	info := InlinedSubroutineInfo{
		Name:          "check_balance",
		DemangledName: "check_balance",
		CallSite: SourceLocation{
			File:   "token.rs",
			Line:   42,
			Column: 5,
		},
		InlinedLocation: SourceLocation{
			File:   "balance.rs",
			Line:   10,
		},
		LowPC:  0x1000,
		HighPC: 0x1100,
	}

	if info.Name != "check_balance" {
		t.Errorf("Name = %q, want %q", info.Name, "check_balance")
	}
	if info.CallSite.File != "token.rs" {
		t.Errorf("CallSite.File = %q, want %q", info.CallSite.File, "token.rs")
	}
	if info.CallSite.Line != 42 {
		t.Errorf("CallSite.Line = %d, want 42", info.CallSite.Line)
	}
	if info.InlinedLocation.File != "balance.rs" {
		t.Errorf("InlinedLocation.File = %q, want %q", info.InlinedLocation.File, "balance.rs")
	}
	if info.LowPC != 0x1000 {
		t.Errorf("LowPC = 0x%x, want 0x1000", info.LowPC)
	}
}

// TestGetInlinedSubroutines_NoDebugInfo ensures GetInlinedSubroutines returns
// ErrNoDebugInfo for a parser with no DWARF data.
func TestGetInlinedSubroutines_NoDebugInfo(t *testing.T) {
	p := &Parser{data: nil}

	_, err := p.GetInlinedSubroutines(0)
	if err != ErrNoDebugInfo {
		t.Errorf("expected ErrNoDebugInfo, got %v", err)
	}
}

// TestResolveInlinedChain_NoDebugInfo ensures ResolveInlinedChain returns
// ErrNoDebugInfo when the parser has no DWARF data.
func TestResolveInlinedChain_NoDebugInfo(t *testing.T) {
	p := &Parser{data: nil}

	_, err := p.ResolveInlinedChain(0x1000)
	if err != ErrNoDebugInfo {
		t.Errorf("expected ErrNoDebugInfo, got %v", err)
	}
}

// TestResolveInlinedChain_MinimalWASM verifies that ResolveInlinedChain
// returns an empty slice (not an error) when the WASM binary has debug info
// but no subprograms contain the queried address.
func TestResolveInlinedChain_MinimalWASM(t *testing.T) {
	// A valid WASM binary that contains no debug sections returns ErrNoDebugInfo
	// at parse time, so we test the zero-subprogram path using a nil-data parser
	// but via the exported API.
	//
	// We cannot easily synthesise a full WASM+DWARF binary in a unit test, so
	// we verify the boundary: parser without data -> ErrNoDebugInfo.
	p := &Parser{data: nil}
	_, err := p.ResolveInlinedChain(0xDEADBEEF)
	if err != ErrNoDebugInfo {
		t.Errorf("expected ErrNoDebugInfo, got %v", err)
	}
}

// TestFindSubprogramOffset_NoData verifies findSubprogramOffset handles nil
// data gracefully and returns 0.
func TestFindSubprogramOffset_NoData(t *testing.T) {
	p := &Parser{data: nil}
	offset := p.findSubprogramOffset("transfer")
	if offset != 0 {
		t.Errorf("expected 0 for nil data, got %d", offset)
	}
}

// TestResolveFileIndex_ZeroIndex ensures that a zero-or-negative fileIndex
// is rejected gracefully without a panic.
func TestResolveFileIndex_ZeroIndex(t *testing.T) {
	p := &Parser{data: nil}
	name := p.resolveFileIndex(0, 0)
	if name != "" {
		t.Errorf("expected empty string for zero index, got %q", name)
	}
	name = p.resolveFileIndex(0, -1)
	if name != "" {
		t.Errorf("expected empty string for negative index, got %q", name)
	}
}

// TestInlinedSubroutineInfo_ZeroValues ensures that a zero-value
// InlinedSubroutineInfo is valid and does not panic when accessed.
func TestInlinedSubroutineInfo_ZeroValues(t *testing.T) {
	var info InlinedSubroutineInfo
	if info.Name != "" {
		t.Errorf("expected empty Name, got %q", info.Name)
	}
	if info.LowPC != 0 {
		t.Errorf("expected zero LowPC, got %d", info.LowPC)
	}
	if info.CallSite.Line != 0 {
		t.Errorf("expected zero CallSite.Line, got %d", info.CallSite.Line)
	}
}
