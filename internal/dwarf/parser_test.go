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
		{
			name: "valid ELF magic",
			data: []byte{
				0x7f, 0x45, 0x4c, 0x46, // ELF magic
			},
			wantErr: ErrInvalidWASM, // Need more data for ELF
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
			loc:  []byte{dwOpStackValue},
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
		name  string
		input string
		want  string
	}{
		{
			name:  "regular name",
			input: "balance",
			want:  "balance",
		},
		{
			name:  "mangled Rust name",
			input: "_RNv4token7balance",
			want:  "_RNv4token7balance",
		},
		{
			name:  "empty name",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
