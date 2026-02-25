// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package demangle_test

import (
	"testing"

	"github.com/dotandev/hintents/internal/demangle"
)

// ---------------------------------------------------------------------------
// DemangleSymbol
// ---------------------------------------------------------------------------

func TestDemangleSymbol_LegacyScheme(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "two-part path with hash",
			input: "_ZN11my_contract6invoke17h1a2b3c4d5e6f7890E",
			want:  "my_contract::invoke",
		},
		{
			name:  "three-part path with hash",
			input: "_ZN11my_contract6client4call17hdeadbeef1234E",
			want:  "my_contract::client::call",
		},
		{
			name:  "soroban sdk symbol",
			input: "_ZN11soroban_sdk3log17habcdef1234567890E",
			want:  "soroban_sdk::log",
		},
		{
			name:  "no hash suffix",
			input: "_ZN11my_contract6invokeE",
			want:  "my_contract::invoke",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := demangle.DemangleSymbol(tc.input)
			if got != tc.want {
				t.Errorf("DemangleSymbol(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestDemangleSymbol_AlreadyReadable(t *testing.T) {
	inputs := []string{
		"my_contract::invoke",
		"soroban_sdk::log",
		"transfer",
		"",
	}
	for _, input := range inputs {
		got := demangle.DemangleSymbol(input)
		if got != input {
			t.Errorf("DemangleSymbol(%q) = %q, want unchanged %q", input, got, input)
		}
	}
}

func TestDemangleSymbol_UnknownScheme(t *testing.T) {
	input := "some_unknown_symbol"
	got := demangle.DemangleSymbol(input)
	if got != input {
		t.Errorf("DemangleSymbol(%q) = %q, want %q", input, got, input)
	}
}

// ---------------------------------------------------------------------------
// DemangleTrace
// ---------------------------------------------------------------------------

func TestDemangleTrace_ReplacesKnownIndex(t *testing.T) {
	table := demangle.SymbolTable{
		42: "_ZN11my_contract6invoke17h1a2b3c4d5e6f7890E",
	}
	got := demangle.DemangleTrace("call func[42]", table)
	want := "call my_contract::invoke"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDemangleTrace_PreservesUnknownIndex(t *testing.T) {
	table := demangle.SymbolTable{
		42: "_ZN11my_contract6invoke17h1a2b3c4d5e6f7890E",
	}
	got := demangle.DemangleTrace("call func[99]", table)
	want := "call func[99]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDemangleTrace_ReplacesMultipleIndices(t *testing.T) {
	table := demangle.SymbolTable{
		42: "_ZN11my_contract6invoke17h1a2b3c4d5e6f7890E",
		7:  "_ZN11soroban_sdk3log17habcdef1234567890E",
	}
	got := demangle.DemangleTrace("call func[42] -> func[7]", table)
	want := "call my_contract::invoke -> soroban_sdk::log"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDemangleTrace_NilTable(t *testing.T) {
	input := "call func[42] -> func[7]"
	got := demangle.DemangleTrace(input, nil)
	if got != input {
		t.Errorf("nil table changed trace: got %q, want %q", got, input)
	}
}

func TestDemangleTrace_NoFuncRefs(t *testing.T) {
	table := demangle.SymbolTable{42: "_ZN11my_contract6invoke17h1a2b3c4d5e6f7890E"}
	input := "contract.invoke error: host trap"
	got := demangle.DemangleTrace(input, table)
	if got != input {
		t.Errorf("no func refs changed trace: got %q, want %q", got, input)
	}
}

// ---------------------------------------------------------------------------
// BuildSymbolTable
// ---------------------------------------------------------------------------

func TestBuildSymbolTable(t *testing.T) {
	entries := []demangle.SymbolEntry{
		{Index: 0, MangledName: "_ZN11my_contract6invoke17h1a2b3c4d5e6f7890E"},
		{Index: 1, MangledName: "_ZN11soroban_sdk3log17habcdef1234567890E"},
	}
	table := demangle.BuildSymbolTable(entries)

	if len(table) != 2 {
		t.Fatalf("got %d entries, want 2", len(table))
	}
	if table[0] != entries[0].MangledName {
		t.Errorf("table[0] = %q, want %q", table[0], entries[0].MangledName)
	}
	if table[1] != entries[1].MangledName {
		t.Errorf("table[1] = %q, want %q", table[1], entries[1].MangledName)
	}
}

func TestBuildSymbolTable_Nil(t *testing.T) {
	table := demangle.BuildSymbolTable(nil)
	if len(table) != 0 {
		t.Errorf("got %d entries, want 0", len(table))
	}
}