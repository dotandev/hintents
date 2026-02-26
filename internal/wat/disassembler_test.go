// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package wat

import (
	"strings"
	"testing"
)

// =============================================================================
// Test WASM module builder
// =============================================================================

// buildMinimalWasm constructs a minimal valid WASM module with a code section.
// The code section contains a single function with the given body bytes.
func buildMinimalWasm(functionBody []byte) []byte {
	// WASM header: magic + version
	module := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	// Type section: one function type () -> ()
	typeSection := []byte{
		SectionType, // section id
		0x04,        // section size
		0x01,        // one type
		0x60,        // func type
		0x00,        // no params
		0x00,        // no results
	}
	module = append(module, typeSection...)

	// Function section: one function referencing type 0
	funcSection := []byte{
		SectionFunction, // section id
		0x02,            // section size
		0x01,            // one function
		0x00,            // type index 0
	}
	module = append(module, funcSection...)

	// Code section
	// Function body = local decl count (0) + body bytes
	funcBody := append([]byte{0x00}, functionBody...) // 0 locals
	funcBody = append(funcBody, 0x0b)                 // end opcode

	funcBodyLen := encodeULEB128(uint64(len(funcBody)))
	codeSectionPayload := append([]byte{0x01}, funcBodyLen...) // 1 function
	codeSectionPayload = append(codeSectionPayload, funcBody...)

	codeSectionLen := encodeULEB128(uint64(len(codeSectionPayload)))
	codeSection := append([]byte{SectionCode}, codeSectionLen...)
	codeSection = append(codeSection, codeSectionPayload...)

	module = append(module, codeSection...)

	return module
}

// encodeULEB128 encodes a uint64 as ULEB128.
func encodeULEB128(v uint64) []byte {
	if v == 0 {
		return []byte{0x00}
	}
	var result []byte
	for v > 0 {
		b := byte(v & 0x7f)
		v >>= 7
		if v > 0 {
			b |= 0x80
		}
		result = append(result, b)
	}
	return result
}

// =============================================================================
// IsValidWasm Tests
// =============================================================================

func TestIsValidWasm_ValidModule(t *testing.T) {
	wasm := buildMinimalWasm([]byte{0x01}) // nop
	d := NewDisassembler(wasm)
	if !d.IsValidWasm() {
		t.Error("expected valid WASM module")
	}
}

func TestIsValidWasm_TooShort(t *testing.T) {
	d := NewDisassembler([]byte{0x00, 0x61})
	if d.IsValidWasm() {
		t.Error("expected invalid for short data")
	}
}

func TestIsValidWasm_WrongMagic(t *testing.T) {
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x01, 0x00, 0x00, 0x00}
	d := NewDisassembler(data)
	if d.IsValidWasm() {
		t.Error("expected invalid for wrong magic")
	}
}

func TestIsValidWasm_WrongVersion(t *testing.T) {
	data := []byte{0x00, 0x61, 0x73, 0x6d, 0x02, 0x00, 0x00, 0x00}
	d := NewDisassembler(data)
	if d.IsValidWasm() {
		t.Error("expected invalid for wrong version")
	}
}

// =============================================================================
// Opcode Decoding Tests
// =============================================================================

func TestDecodeOpcode_Unreachable(t *testing.T) {
	m, op, n := decodeOpcode(0x00, nil)
	if m != "unreachable" || op != "" || n != 0 {
		t.Errorf("unreachable: got %q %q %d", m, op, n)
	}
}

func TestDecodeOpcode_Nop(t *testing.T) {
	m, _, _ := decodeOpcode(0x01, nil)
	if m != "nop" {
		t.Errorf("nop: got %q", m)
	}
}

func TestDecodeOpcode_Call(t *testing.T) {
	// call $func5 (index 5, encoded as single ULEB128 byte)
	m, op, n := decodeOpcode(0x10, []byte{0x05})
	if m != "call" {
		t.Errorf("expected 'call', got %q", m)
	}
	if op != "$func5" {
		t.Errorf("expected '$func5', got %q", op)
	}
	if n != 1 {
		t.Errorf("expected 1 byte consumed, got %d", n)
	}
}

func TestDecodeOpcode_LocalGet(t *testing.T) {
	m, op, n := decodeOpcode(0x20, []byte{0x03})
	if m != "local.get" || op != "3" || n != 1 {
		t.Errorf("local.get: got %q %q %d", m, op, n)
	}
}

func TestDecodeOpcode_I32Const(t *testing.T) {
	// i32.const 42 (42 in SLEB128 = 0x2a)
	m, op, n := decodeOpcode(0x41, []byte{0x2a})
	if m != "i32.const" || op != "42" || n != 1 {
		t.Errorf("i32.const 42: got %q %q %d", m, op, n)
	}
}

func TestDecodeOpcode_I32ConstNegative(t *testing.T) {
	// i32.const -1 in SLEB128 = 0x7f
	m, op, n := decodeOpcode(0x41, []byte{0x7f})
	if m != "i32.const" || op != "-1" || n != 1 {
		t.Errorf("i32.const -1: got %q %q %d", m, op, n)
	}
}

func TestDecodeOpcode_I32Add(t *testing.T) {
	m, _, _ := decodeOpcode(0x6a, nil)
	if m != "i32.add" {
		t.Errorf("expected 'i32.add', got %q", m)
	}
}

func TestDecodeOpcode_I32Load(t *testing.T) {
	// i32.load align=2 offset=0
	m, op, n := decodeOpcode(0x28, []byte{0x02, 0x00})
	if m != "i32.load" {
		t.Errorf("expected 'i32.load', got %q", m)
	}
	if !strings.Contains(op, "offset=0") || !strings.Contains(op, "align=2") {
		t.Errorf("i32.load operands = %q", op)
	}
	if n != 2 {
		t.Errorf("expected 2 bytes consumed, got %d", n)
	}
}

func TestDecodeOpcode_Block(t *testing.T) {
	// block (void)
	m, _, n := decodeOpcode(0x02, []byte{0x40})
	if m != "block" {
		t.Errorf("expected 'block', got %q", m)
	}
	if n != 1 {
		t.Errorf("expected 1 byte consumed, got %d", n)
	}
}

func TestDecodeOpcode_End(t *testing.T) {
	m, _, _ := decodeOpcode(0x0b, nil)
	if m != "end" {
		t.Errorf("expected 'end', got %q", m)
	}
}

func TestDecodeOpcode_Drop(t *testing.T) {
	m, _, _ := decodeOpcode(0x1a, nil)
	if m != "drop" {
		t.Errorf("expected 'drop', got %q", m)
	}
}

func TestDecodeOpcode_Return(t *testing.T) {
	m, _, _ := decodeOpcode(0x0f, nil)
	if m != "return" {
		t.Errorf("expected 'return', got %q", m)
	}
}

func TestDecodeOpcode_Unknown(t *testing.T) {
	m, _, _ := decodeOpcode(0xFE, nil)
	if !strings.HasPrefix(m, "unknown_") {
		t.Errorf("expected 'unknown_' prefix, got %q", m)
	}
}

// =============================================================================
// LEB128 Tests
// =============================================================================

func TestDecodeULEB128_Zero(t *testing.T) {
	val, n := decodeULEB128([]byte{0x00})
	if val != 0 || n != 1 {
		t.Errorf("ULEB128(0) = %d, %d bytes", val, n)
	}
}

func TestDecodeULEB128_SingleByte(t *testing.T) {
	val, n := decodeULEB128([]byte{0x7f})
	if val != 127 || n != 1 {
		t.Errorf("ULEB128(127) = %d, %d bytes", val, n)
	}
}

func TestDecodeULEB128_MultiByte(t *testing.T) {
	// 128 = 0x80 0x01
	val, n := decodeULEB128([]byte{0x80, 0x01})
	if val != 128 || n != 2 {
		t.Errorf("ULEB128(128) = %d, %d bytes", val, n)
	}
}

func TestDecodeULEB128_LargeValue(t *testing.T) {
	// 624485 = 0xe5 0x8e 0x26
	val, n := decodeULEB128([]byte{0xe5, 0x8e, 0x26})
	if val != 624485 || n != 3 {
		t.Errorf("ULEB128(624485) = %d, %d bytes", val, n)
	}
}

func TestDecodeSLEB128_Positive(t *testing.T) {
	val, n := decodeSLEB128([]byte{0x2a})
	if val != 42 || n != 1 {
		t.Errorf("SLEB128(42) = %d, %d bytes", val, n)
	}
}

func TestDecodeSLEB128_Negative(t *testing.T) {
	// -1 in SLEB128 = 0x7f
	val, n := decodeSLEB128([]byte{0x7f})
	if val != -1 || n != 1 {
		t.Errorf("SLEB128(-1) = %d, %d bytes", val, n)
	}
}

func TestDecodeSLEB128_NegativeLarge(t *testing.T) {
	// -128 in SLEB128 = 0x80, 0x7f
	val, n := decodeSLEB128([]byte{0x80, 0x7f})
	if val != -128 || n != 2 {
		t.Errorf("SLEB128(-128) = %d, %d bytes", val, n)
	}
}

// =============================================================================
// DisassembleAt Tests
// =============================================================================

func TestDisassembleAt_SimpleFunction(t *testing.T) {
	// Function body: i32.const 1, i32.const 2, i32.add, drop
	body := []byte{
		0x41, 0x01, // i32.const 1
		0x41, 0x02, // i32.const 2
		0x6a, // i32.add
		0x1a, // drop
	}
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)

	// The code section starts after header + type section + func section
	// Find the actual offset of i32.add by decoding
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	if len(instructions) < 5 { // i32.const, i32.const, i32.add, drop, end
		t.Fatalf("expected at least 5 instructions, got %d", len(instructions))
	}

	// Find i32.add instruction
	var addOffset uint64
	for _, inst := range instructions {
		if inst.Mnemonic == "i32.add" {
			addOffset = inst.Offset
			break
		}
	}

	snippet, err := d.DisassembleAt(addOffset, 2)
	if err != nil {
		t.Fatalf("DisassembleAt failed: %v", err)
	}

	if snippet.TargetIndex < 0 {
		t.Error("expected target to be found")
	}

	targetInst := snippet.Instructions[snippet.TargetIndex]
	if targetInst.Mnemonic != "i32.add" {
		t.Errorf("expected target instruction 'i32.add', got %q", targetInst.Mnemonic)
	}
}

func TestDisassembleAt_UnreachableInstruction(t *testing.T) {
	body := []byte{0x00} // unreachable
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	// Find unreachable
	var unreachableOffset uint64
	found := false
	for _, inst := range instructions {
		if inst.Mnemonic == "unreachable" {
			unreachableOffset = inst.Offset
			found = true
			break
		}
	}
	if !found {
		t.Fatal("unreachable instruction not found")
	}

	snippet, err := d.DisassembleAt(unreachableOffset, 1)
	if err != nil {
		t.Fatalf("DisassembleAt failed: %v", err)
	}

	if snippet.TargetIndex < 0 || snippet.TargetIndex >= len(snippet.Instructions) {
		t.Fatalf("invalid target index %d (len=%d)", snippet.TargetIndex, len(snippet.Instructions))
	}

	if snippet.Instructions[snippet.TargetIndex].Mnemonic != "unreachable" {
		t.Errorf("expected 'unreachable', got %q", snippet.Instructions[snippet.TargetIndex].Mnemonic)
	}
}

func TestDisassembleAt_InvalidWasm(t *testing.T) {
	d := NewDisassembler([]byte{0xFF, 0xFF})
	_, err := d.DisassembleAt(0, 5)
	if err == nil {
		t.Error("expected error for invalid WASM")
	}
}

// =============================================================================
// DecodeAll Tests
// =============================================================================

func TestDecodeAll_NopSequence(t *testing.T) {
	body := []byte{0x01, 0x01, 0x01} // 3 nops
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	nopCount := 0
	for _, inst := range instructions {
		if inst.Mnemonic == "nop" {
			nopCount++
		}
	}
	if nopCount != 3 {
		t.Errorf("expected 3 nops, found %d", nopCount)
	}
}

func TestDecodeAll_CallInstruction(t *testing.T) {
	body := []byte{0x10, 0x00} // call $func0
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	found := false
	for _, inst := range instructions {
		if inst.Mnemonic == "call" && inst.Operands == "$func0" {
			found = true
			break
		}
	}
	if !found {
		t.Error("call $func0 instruction not found")
	}
}

// =============================================================================
// Snippet Format Tests
// =============================================================================

func TestSnippetFormat_WithTarget(t *testing.T) {
	snippet := &Snippet{
		Instructions: []Instruction{
			{Offset: 0x10, Mnemonic: "i32.const", Operands: "1"},
			{Offset: 0x12, Mnemonic: "i32.const", Operands: "2"},
			{Offset: 0x14, Mnemonic: "i32.add"},
		},
		TargetOffset: 0x14,
		TargetIndex:  2,
	}

	output := snippet.Format()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), output)
	}

	// First two lines should not have marker
	if !strings.HasPrefix(lines[0], "  ") {
		t.Errorf("line 0 should start with '  ', got %q", lines[0])
	}

	// Target line should have '>' marker
	if !strings.HasPrefix(lines[2], "> ") {
		t.Errorf("target line should start with '> ', got %q", lines[2])
	}

	if !strings.Contains(lines[2], "i32.add") {
		t.Errorf("target line should contain 'i32.add', got %q", lines[2])
	}
}

func TestSnippetFormat_Empty(t *testing.T) {
	snippet := &Snippet{
		Instructions: nil,
		TargetIndex:  -1,
	}
	output := snippet.Format()
	if !strings.Contains(output, "no instructions") {
		t.Errorf("expected 'no instructions' message, got %q", output)
	}
}

// =============================================================================
// Instruction String Tests
// =============================================================================

func TestInstructionString_WithOperands(t *testing.T) {
	inst := &Instruction{Mnemonic: "i32.const", Operands: "42"}
	if inst.String() != "i32.const 42" {
		t.Errorf("expected 'i32.const 42', got %q", inst.String())
	}
}

func TestInstructionString_NoOperands(t *testing.T) {
	inst := &Instruction{Mnemonic: "i32.add"}
	if inst.String() != "i32.add" {
		t.Errorf("expected 'i32.add', got %q", inst.String())
	}
}

// =============================================================================
// FormatFallback Tests
// =============================================================================

func TestFormatFallback_ValidWasm(t *testing.T) {
	body := []byte{0x41, 0x01, 0x1a} // i32.const 1, drop
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}

	var dropOffset uint64
	for _, inst := range instructions {
		if inst.Mnemonic == "drop" {
			dropOffset = inst.Offset
			break
		}
	}

	output := FormatFallback(wasm, dropOffset, 3)
	if !strings.Contains(output, "WAT disassembly") {
		t.Errorf("expected 'WAT disassembly' header, got %q", output)
	}
	if !strings.Contains(output, "drop") {
		t.Errorf("expected 'drop' in output, got %q", output)
	}
}

func TestFormatFallback_InvalidWasm(t *testing.T) {
	output := FormatFallback([]byte{0xFF, 0xFF}, 0, 5)
	if !strings.Contains(output, "could not parse") {
		t.Errorf("expected parse error message, got %q", output)
	}
}

func TestFormatFallback_DefaultContext(t *testing.T) {
	body := []byte{0x01} // nop
	wasm := buildMinimalWasm(body)
	output := FormatFallback(wasm, 0, 0)
	// contextLines=0 should default to 5
	if !strings.Contains(output, "WAT disassembly") {
		t.Errorf("expected fallback output, got %q", output)
	}
}

// =============================================================================
// BlockType Tests
// =============================================================================

func TestDecodeBlockType_Void(t *testing.T) {
	bt, n := decodeBlockType([]byte{0x40})
	if bt != "" || n != 1 {
		t.Errorf("void block: got %q, %d", bt, n)
	}
}

func TestDecodeBlockType_I32(t *testing.T) {
	bt, n := decodeBlockType([]byte{0x7f})
	if bt != "(result i32)" || n != 1 {
		t.Errorf("i32 block: got %q, %d", bt, n)
	}
}

func TestDecodeBlockType_I64(t *testing.T) {
	bt, n := decodeBlockType([]byte{0x7e})
	if bt != "(result i64)" || n != 1 {
		t.Errorf("i64 block: got %q, %d", bt, n)
	}
}

func TestDecodeBlockType_Empty(t *testing.T) {
	bt, n := decodeBlockType([]byte{})
	if bt != "" || n != 0 {
		t.Errorf("empty block: got %q, %d", bt, n)
	}
}
