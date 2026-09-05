// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package wat

import (
	"strings"
	"testing"
)

// =============================================================================
// Gas Cost Tests
// =============================================================================

func TestGetGasCost(t *testing.T) {
	tests := []struct {
		mnemonic     string
		expectedCost uint64
	}{
		{"i32.add", 1},
		{"i32.mul", 3},
		{"i32.div_s", 5},
		{"i64.clz", 12},
		{"call", 5},
		{"memory.grow", 10},
		{"select", 3},
		{"unknown_instruction", 1}, // Default cost
	}

	for _, test := range tests {
		t.Run(test.mnemonic, func(t *testing.T) {
			cost := GetGasCost(test.mnemonic)
			if cost != test.expectedCost {
				t.Errorf("GetGasCost(%s) = %d, expected %d", test.mnemonic, cost, test.expectedCost)
			}
		})
	}
}

func TestGasCostAnalysis_SimpleFunction(t *testing.T) {
	// Function body: i32.const 1, i32.const 2, i32.add, drop
	body := []byte{
		0x41, 0x01, // i32.const 1
		0x41, 0x02, // i32.const 2
		0x6a,       // i32.add
		0x1a,       // drop
	}
	wasm := buildMinimalWasm(body)

	dis := NewDisassembler(wasm)
	if !dis.IsValidWasm() {
		t.Fatal("Invalid WASM module")
	}

	analysis, err := dis.AnalyzeGasCosts()
	if err != nil {
		t.Fatalf("AnalyzeGasCosts failed: %v", err)
	}

	if analysis.TotalInstructions == 0 {
		t.Error("Expected at least one instruction")
	}

	if analysis.TotalGasCost == 0 {
		t.Error("Expected non-zero total gas cost")
	}

	// Verify specific instruction counts
	if analysis.InstructionCounts["i32.const"] != 2 {
		t.Errorf("Expected 2 i32.const instructions, got %d", analysis.InstructionCounts["i32.const"])
	}

	if analysis.InstructionCounts["i32.add"] != 1 {
		t.Errorf("Expected 1 i32.add instruction, got %d", analysis.InstructionCounts["i32.add"])
	}

	// Test the format method
	output := analysis.Format()
	if output == "" {
		t.Error("Expected non-empty formatted output")
	}

	if !strings.Contains(output, "Gas Cost Analysis") {
		t.Error("Expected analysis header in output")
	}
}

func TestGasCostAnalysis_ComplexFunction(t *testing.T) {
	// Function body with various instruction types
	body := []byte{
		0x41, 0x0a, // i32.const 10
		0x41, 0x14, // i32.const 20
		0x6a,       // i32.add
		0x41, 0x03, // i32.const 3
		0x6c,       // i32.mul
		0x41, 0x02, // i32.const 2
		0x6e,       // i32.div_u
		0x1a,       // drop
	}
	wasm := buildMinimalWasm(body)

	dis := NewDisassembler(wasm)
	analysis, err := dis.AnalyzeGasCosts()
	if err != nil {
		t.Fatalf("AnalyzeGasCosts failed: %v", err)
	}

	// Verify we have the expected instruction types
	expectedInstructions := []string{"i32.const", "i32.add", "i32.mul", "i32.div_u", "drop"}
	for _, expected := range expectedInstructions {
		if count := analysis.InstructionCounts[expected]; count == 0 {
			t.Errorf("Expected at least one %s instruction, got %d", expected, count)
		}
	}

	// Verify gas costs are properly calculated
	if analysis.TotalGasCost <= 0 {
		t.Error("Expected positive total gas cost")
	}

	// Verify average gas cost
	if analysis.AverageGasCost <= 0 {
		t.Error("Expected positive average gas cost")
	}
}

func TestDisassembleAtWithGas(t *testing.T) {
	// Function body: i32.const 1, i32.const 2, i32.add, drop
	body := []byte{
		0x41, 0x01, // i32.const 1
		0x41, 0x02, // i32.const 2
		0x6a,       // i32.add
		0x1a,       // drop
	}
	wasm := buildMinimalWasm(body)

	dis := NewDisassembler(wasm)
	if !dis.IsValidWasm() {
		t.Fatal("Invalid WASM module")
	}

	// Find the i32.add instruction offset
	instructions, err := dis.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	var addOffset uint64
	for _, inst := range instructions {
		if inst.Mnemonic == "i32.add" {
			addOffset = inst.Offset
			break
		}
	}

	// Test regular disassembly
	snippet, err := dis.DisassembleAt(addOffset, 2)
	if err != nil {
		t.Fatalf("DisassembleAt failed: %v", err)
	}

	regularOutput := snippet.Format()
	if !strings.Contains(regularOutput, "i32.add") {
		t.Error("Expected i32.add in regular output")
	}

	// Test disassembly with gas costs
	snippetWithGas, err := dis.DisassembleAtWithGas(addOffset, 2)
	if err != nil {
		t.Fatalf("DisassembleAtWithGas failed: %v", err)
	}

	gasOutput := snippetWithGas.Format()
	if !strings.Contains(gasOutput, "gas:") {
		t.Error("Expected gas information in gas output")
	}

	if !strings.Contains(gasOutput, "particles") {
		t.Error("Expected particle information in gas output")
	}

	if len(gasOutput) <= len(regularOutput) {
		t.Error("Expected gas output to be longer than regular output")
	}
}

func TestInstructionGasCost_Assignment(t *testing.T) {
	// Function body with various instruction types to test gas cost assignment
	body := []byte{
		0x41, 0x01, // i32.const 1 (cost: 1)
		0x41, 0x02, // i32.const 2 (cost: 1)
		0x6a,       // i32.add (cost: 1)
		0x6c,       // i32.mul (cost: 3)
		0x6d,       // i32.div_u (cost: 5)
		0x1a,       // drop (cost: 1)
	}
	wasm := buildMinimalWasm(body)

	dis := NewDisassembler(wasm)
	if !dis.IsValidWasm() {
		t.Fatal("Invalid WASM module")
	}

	instructions, err := dis.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	// Check that all instructions have gas costs assigned
	for i, inst := range instructions {
		if inst.GasCost == 0 {
			t.Errorf("Instruction %d (%s) has zero gas cost", i, inst.Mnemonic)
		}

		// Verify specific gas costs for known instructions
		switch inst.Mnemonic {
		case "i32.const", "i32.add", "drop":
			if inst.GasCost != 1 {
				t.Errorf("Instruction %d (%s) expected cost 1, got %d", i, inst.Mnemonic, inst.GasCost)
			}
		case "i32.mul":
			if inst.GasCost != 3 {
				t.Errorf("Instruction %d (%s) expected cost 3, got %d", i, inst.Mnemonic, inst.GasCost)
			}
		case "i32.div_u":
			if inst.GasCost != 5 {
				t.Errorf("Instruction %d (%s) expected cost 5, got %d", i, inst.Mnemonic, inst.GasCost)
			}
		}
	}
}

func TestSnippetFormatWithGas(t *testing.T) {
	snippet := &Snippet{
		Instructions: []Instruction{
			{Offset: 0x10, Mnemonic: "i32.const", Operands: "1", GasCost: 1},
			{Offset: 0x12, Mnemonic: "i32.const", Operands: "2", GasCost: 1},
			{Offset: 0x14, Mnemonic: "i32.add", GasCost: 1},
		},
		TargetOffset: 0x14,
		TargetIndex:  2,
		ShowGasCosts: true,
	}

	output := snippet.Format()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), output)
	}

	// Check that gas information is included
	for _, line := range lines {
		if !strings.Contains(line, "gas:") {
			t.Errorf("Expected gas information in line: %q", line)
		}
		if !strings.Contains(line, "particles") {
			t.Errorf("Expected particle information in line: %q", line)
		}
	}

	// Target line should have '>' marker and gas info
	targetLine := lines[2]
	if !strings.HasPrefix(targetLine, "> ") {
		t.Errorf("target line should start with '> ', got %q", targetLine)
	}

	if !strings.Contains(targetLine, "i32.add") {
		t.Errorf("target line should contain 'i32.add', got %q", targetLine)
	}
}

func TestGasCostAnalysis_EmptyModule(t *testing.T) {
	// Create a WASM module with an empty function body
	body := []byte{}
	wasm := buildMinimalWasm(body)

	dis := NewDisassembler(wasm)
	analysis, err := dis.AnalyzeGasCosts()
	if err != nil {
		t.Fatalf("AnalyzeGasCosts failed: %v", err)
	}

	// Empty function should still have the 'end' instruction
	if analysis.TotalInstructions == 0 {
		t.Error("Expected at least one instruction (end)")
	}

	// Format should still work
	output := analysis.Format()
	if output == "" {
		t.Error("Expected non-empty formatted output")
	}
}

func TestFormatGasAnalysis_Sorting(t *testing.T) {
	// Create a WASM module with instructions that have different gas costs
	body := []byte{
		0x41, 0x01, // i32.const 1 (cost: 1)
		0x6c,       // i32.mul (cost: 3)
		0x6d,       // i32.div_u (cost: 5)
		0x1a,       // drop (cost: 1)
	}
	wasm := buildMinimalWasm(body)

	dis := NewDisassembler(wasm)
	analysis, err := dis.AnalyzeGasCosts()
	if err != nil {
		t.Fatalf("AnalyzeGasCosts failed: %v", err)
	}

	output := analysis.Format()
	
	// Check that the output contains the expected sections
	if !strings.Contains(output, "Top Instructions by Total Gas Cost") {
		t.Error("Expected top instructions section in output")
	}

	if !strings.Contains(output, "Total Instructions:") {
		t.Error("Expected total instructions in output")
	}

	if !strings.Contains(output, "Total Gas Cost:") {
		t.Error("Expected total gas cost in output")
	}
}
