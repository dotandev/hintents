// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package wat

import (
	"strconv"
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

// =============================================================================
// CrossReferenceEvents Tests
// =============================================================================

// testEvent is a minimal DiagnosticEventSource for testing.
type testEvent struct{ wasmInstruction *string }

func (e *testEvent) GetWasmInstruction() *string { return e.wasmInstruction }

func strPtr(s string) *string { return &s }

func TestCrossReferenceEvents_Basic(t *testing.T) {
	// Build a WASM with: i32.const 1, i32.add, drop
	body := []byte{0x41, 0x01, 0x6a, 0x1a}
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}

	// Find the i32.add offset.
	var addOffset uint64
	for _, inst := range instructions {
		if inst.Mnemonic == "i32.add" {
			addOffset = inst.Offset
			break
		}
	}

	events := []DiagnosticEventSource{
		&testEvent{wasmInstruction: strPtr(strconv.FormatUint(addOffset, 10))},
	}

	refs, err := CrossReferenceEvents(wasm, events)
	if err != nil {
		t.Fatalf("CrossReferenceEvents: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Instruction == nil {
		t.Fatal("expected instruction to be resolved")
	}
	if refs[0].Instruction.Mnemonic != "i32.add" {
		t.Errorf("expected 'i32.add', got %q", refs[0].Instruction.Mnemonic)
	}
	if refs[0].EventIndex != 0 {
		t.Errorf("expected EventIndex 0, got %d", refs[0].EventIndex)
	}
}

func TestCrossReferenceEvents_SkipsNilInstruction(t *testing.T) {
	wasm := buildMinimalWasm([]byte{0x01}) // nop
	events := []DiagnosticEventSource{
		&testEvent{wasmInstruction: nil},
		&testEvent{wasmInstruction: strPtr("")},
	}
	refs, err := CrossReferenceEvents(wasm, events)
	if err != nil {
		t.Fatalf("CrossReferenceEvents: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for nil/empty instructions, got %d", len(refs))
	}
}

func TestCrossReferenceEvents_UnresolvableOffset(t *testing.T) {
	wasm := buildMinimalWasm([]byte{0x01}) // nop
	// Offset 9999 won't exist in this tiny module.
	events := []DiagnosticEventSource{
		&testEvent{wasmInstruction: strPtr("9999")},
	}
	refs, err := CrossReferenceEvents(wasm, events)
	if err != nil {
		t.Fatalf("CrossReferenceEvents: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Instruction != nil {
		t.Error("expected nil instruction for unresolvable offset")
	}
	if refs[0].Offset != 9999 {
		t.Errorf("expected offset 9999, got %d", refs[0].Offset)
	}
}

func TestCrossReferenceEvents_InvalidWasm(t *testing.T) {
	_, err := CrossReferenceEvents([]byte{0xFF, 0xFF}, []DiagnosticEventSource{})
	if err == nil {
		t.Error("expected error for invalid WASM")
	}
}

func TestCrossReferenceEvents_MultipleEvents(t *testing.T) {
	body := []byte{0x41, 0x01, 0x6a, 0x1a} // i32.const 1, i32.add, drop
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}

	// Collect offsets for i32.add and drop.
	offsets := map[string]uint64{}
	for _, inst := range instructions {
		if inst.Mnemonic == "i32.add" || inst.Mnemonic == "drop" {
			offsets[inst.Mnemonic] = inst.Offset
		}
	}

	events := []DiagnosticEventSource{
		&testEvent{wasmInstruction: nil}, // skipped
		&testEvent{wasmInstruction: strPtr(strconv.FormatUint(offsets["i32.add"], 10))},
		&testEvent{wasmInstruction: strPtr(strconv.FormatUint(offsets["drop"], 10))},
	}

	refs, err := CrossReferenceEvents(wasm, events)
	if err != nil {
		t.Fatalf("CrossReferenceEvents: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].EventIndex != 1 {
		t.Errorf("expected EventIndex 1, got %d", refs[0].EventIndex)
	}
	if refs[0].Instruction == nil || refs[0].Instruction.Mnemonic != "i32.add" {
		t.Errorf("expected 'i32.add' for first ref")
	}
	if refs[1].EventIndex != 2 {
		t.Errorf("expected EventIndex 2, got %d", refs[1].EventIndex)
	}
	if refs[1].Instruction == nil || refs[1].Instruction.Mnemonic != "drop" {
		t.Errorf("expected 'drop' for second ref")
	}
}

func TestCrossReferenceEvents_UnparsableOffset(t *testing.T) {
	wasm := buildMinimalWasm([]byte{0x01})
	events := []DiagnosticEventSource{
		&testEvent{wasmInstruction: strPtr("not-a-number")},
	}
	refs, err := CrossReferenceEvents(wasm, events)
	if err != nil {
		t.Fatalf("CrossReferenceEvents: %v", err)
	}
	// Should still produce a ref with zero offset and nil instruction.
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Instruction != nil {
		t.Error("expected nil instruction for unparsable offset")
	}
}

// =============================================================================
// Basic Block Analysis Tests
// =============================================================================

func TestDisassembleAtWithBasicBlocks_SimpleFunction(t *testing.T) {
	// Function body: i32.const 1, i32.const 2, i32.add, drop
	body := []byte{
		0x41, 0x01, // i32.const 1
		0x41, 0x02, // i32.const 2
		0x6a, // i32.add
		0x1a, // drop
	}
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	// Find the i32.add instruction
	var addOffset uint64
	for _, inst := range instructions {
		if inst.Mnemonic == "i32.add" {
			addOffset = inst.Offset
			break
		}
	}

	snippet, err := d.DisassembleAtWithBasicBlocks(addOffset, 2)
	if err != nil {
		t.Fatalf("DisassembleAtWithBasicBlocks failed: %v", err)
	}

	if snippet.BasicBlocks == nil {
		t.Fatal("expected basic blocks analysis to be populated")
	}

	if len(snippet.BasicBlocks.Blocks) == 0 {
		t.Error("expected at least one basic block")
	}

	// Should have a single block for this simple linear function
	if len(snippet.BasicBlocks.Blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(snippet.BasicBlocks.Blocks))
	}

	block := snippet.BasicBlocks.Blocks[0]
	if block.BlockType != "normal" {
		t.Errorf("expected block type 'normal', got %q", block.BlockType)
	}

	if len(block.Instructions) < 4 {
		t.Errorf("expected at least 4 instructions in block, got %d", len(block.Instructions))
	}
}

func TestDisassembleAtWithBasicBlocks_BranchFunction(t *testing.T) {
	// Function body with branch: i32.const 1, br_if 0, i32.const 2, drop
	body := []byte{
		0x41, 0x01, // i32.const 1
		0x0d, 0x00, // br_if 0
		0x41, 0x02, // i32.const 2
		0x1a, // drop
	}
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	// Find the br_if instruction
	var brIfOffset uint64
	for _, inst := range instructions {
		if inst.Mnemonic == "br_if" {
			brIfOffset = inst.Offset
			break
		}
	}

	snippet, err := d.DisassembleAtWithBasicBlocks(brIfOffset, 3)
	if err != nil {
		t.Fatalf("DisassembleAtWithBasicBlocks failed: %v", err)
	}

	if snippet.BasicBlocks == nil {
		t.Fatal("expected basic blocks analysis to be populated")
	}

	// Should have multiple blocks due to the branch
	if len(snippet.BasicBlocks.Blocks) < 2 {
		t.Errorf("expected at least 2 blocks due to branch, got %d", len(snippet.BasicBlocks.Blocks))
	}

	// Check that we identified jump targets
	if len(snippet.BasicBlocks.JumpTargets) == 0 {
		t.Error("expected to find jump targets")
	}

	// Check block types
	foundConditional := false
	for _, block := range snippet.BasicBlocks.Blocks {
		if block.BlockType == "conditional" {
			foundConditional = true
			break
		}
	}
	if !foundConditional {
		t.Error("expected to find a conditional block")
	}
}

func TestDisassembleAtWithBasicBlocks_LoopFunction(t *testing.T) {
	// Function body with loop: loop, i32.const 1, br 0, end
	body := []byte{
		0x03, 0x40, // loop (void)
		0x41, 0x01, // i32.const 1
		0x0c, 0x00, // br 0
		0x0b, // end
	}
	wasm := buildMinimalWasm(body)

	d := NewDisassembler(wasm)
	instructions, err := d.DecodeAll()
	if err != nil {
		t.Fatalf("DecodeAll failed: %v", err)
	}

	// Find the loop instruction
	var loopOffset uint64
	for _, inst := range instructions {
		if inst.Mnemonic == "loop" {
			loopOffset = inst.Offset
			break
		}
	}

	snippet, err := d.DisassembleAtWithBasicBlocks(loopOffset, 3)
	if err != nil {
		t.Fatalf("DisassembleAtWithBasicBlocks failed: %v", err)
	}

	if snippet.BasicBlocks == nil {
		t.Fatal("expected basic blocks analysis to be populated")
	}

	// Check block types
	foundLoop := false
	for _, block := range snippet.BasicBlocks.Blocks {
		if block.BlockType == "loop" {
			foundLoop = true
			break
		}
	}
	if !foundLoop {
		t.Error("expected to find a loop block")
	}
}

func TestAnalyzeBasicBlocks_EmptyInstructions(t *testing.T) {
	d := NewDisassembler([]byte{})
	analysis := d.AnalyzeBasicBlocks([]Instruction{})

	if analysis == nil {
		t.Fatal("expected analysis to be returned")
	}

	if len(analysis.Blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(analysis.Blocks))
	}

	if len(analysis.OffsetToBlock) != 0 {
		t.Errorf("expected 0 offset mappings, got %d", len(analysis.OffsetToBlock))
	}

	if len(analysis.JumpTargets) != 0 {
		t.Errorf("expected 0 jump targets, got %d", len(analysis.JumpTargets))
	}
}

func TestIsJumpInstruction(t *testing.T) {
	d := NewDisassembler([]byte{})

	jumpInstructions := []string{"br", "br_if", "br_table", "return", "call", "call_indirect"}
	for _, mnemonic := range jumpInstructions {
		if !d.isJumpInstruction(mnemonic) {
			t.Errorf("expected %q to be a jump instruction", mnemonic)
		}
	}

	nonJumpInstructions := []string{"i32.add", "i32.const", "local.get", "drop", "nop"}
	for _, mnemonic := range nonJumpInstructions {
		if d.isJumpInstruction(mnemonic) {
			t.Errorf("expected %q not to be a jump instruction", mnemonic)
		}
	}
}

func TestDetermineBlockType(t *testing.T) {
	d := NewDisassembler([]byte{})

	// Test normal block
	normalBlock := []Instruction{
		{Mnemonic: "i32.const"},
		{Mnemonic: "i32.add"},
		{Mnemonic: "drop"},
	}
	if blockType := d.determineBlockType(normalBlock); blockType != "normal" {
		t.Errorf("expected 'normal', got %q", blockType)
	}

	// Test conditional block
	conditionalBlock := []Instruction{
		{Mnemonic: "i32.const"},
		{Mnemonic: "br_if"},
		{Mnemonic: "drop"},
	}
	if blockType := d.determineBlockType(conditionalBlock); blockType != "conditional" {
		t.Errorf("expected 'conditional', got %q", blockType)
	}

	// Test loop block
	loopBlock := []Instruction{
		{Mnemonic: "loop"},
		{Mnemonic: "i32.const"},
		{Mnemonic: "br"},
	}
	if blockType := d.determineBlockType(loopBlock); blockType != "loop" {
		t.Errorf("expected 'loop', got %q", blockType)
	}

	// Test branch block
	branchBlock := []Instruction{
		{Mnemonic: "i32.const"},
		{Mnemonic: "i32.add"},
		{Mnemonic: "br"},
	}
	if blockType := d.determineBlockType(branchBlock); blockType != "branch" {
		t.Errorf("expected 'branch', got %q", blockType)
	}

	// Test empty block
	emptyBlock := []Instruction{}
	if blockType := d.determineBlockType(emptyBlock); blockType != "empty" {
		t.Errorf("expected 'empty', got %q", blockType)
	}
}

func TestFormatWithBasicBlocks_WithJumpTargets(t *testing.T) {
	// Create a snippet with basic block analysis
	snippet := &Snippet{
		Instructions: []Instruction{
			{Offset: 0x10, Mnemonic: "i32.const", Operands: "1"},
			{Offset: 0x12, Mnemonic: "br_if", Operands: "0"},
			{Offset: 0x14, Mnemonic: "i32.const", Operands: "2"},
			{Offset: 0x16, Mnemonic: "drop"},
		},
		TargetOffset: 0x12,
		TargetIndex:  1,
		BasicBlocks: &BasicBlockAnalysis{
			Blocks: []BasicBlock{
				{
					StartOffset: 0x10,
					EndOffset:   0x12,
					BlockType:   "conditional",
					IsJumpTarget: false,
				},
				{
					StartOffset: 0x14,
					EndOffset:   0x16,
					BlockType:   "normal",
					IsJumpTarget: true,
					JumpSources: []uint64{0x12},
				},
			},
			OffsetToBlock: map[uint64]*BasicBlock{
				0x10: &BasicBlock{StartOffset: 0x10, EndOffset: 0x12, BlockType: "conditional"},
				0x12: &BasicBlock{StartOffset: 0x10, EndOffset: 0x12, BlockType: "conditional"},
				0x14: &BasicBlock{StartOffset: 0x14, EndOffset: 0x16, BlockType: "normal", IsJumpTarget: true},
				0x16: &BasicBlock{StartOffset: 0x14, EndOffset: 0x16, BlockType: "normal", IsJumpTarget: true},
			},
			JumpTargets: map[uint64]uint64{
				0x12: 0x14,
			},
		},
	}

	output := snippet.Format()

	// Should contain block boundaries
	if !strings.Contains(output, "block start") {
		t.Error("expected output to contain block start markers")
	}

	if !strings.Contains(output, "end of block") {
		t.Error("expected output to contain block end markers")
	}

	// Should contain jump target information
	if !strings.Contains(output, "JUMP TARGET") {
		t.Error("expected output to contain jump target marker")
	}

	// Should contain jump arrow
	if !strings.Contains(output, "-> 0x") {
		t.Error("expected output to contain jump target arrow")
	}

	// Should contain block type
	if !strings.Contains(output, "[conditional]") {
		t.Error("expected output to contain conditional block type")
	}
}
