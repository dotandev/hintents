// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package wat provides WebAssembly Text format (WAT) decompilation for
// WASM bytecode. When source mapping is unavailable (no DWARF debug
// symbols), this package decodes raw WASM instructions and renders them
// in the WAT text format so that the exact failing instruction can be
// shown to the user.
//
// This is a fallback mechanism: if the WASM was compiled without debug
// info, or source mapping fails, the user still gets a readable view
// of what instruction trapped.
package wat

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// =============================================================================
// WASM constants
// =============================================================================

// WASM magic number and version.
var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d}

const wasmVersion = 1

// WASM section IDs.
const (
	SectionCustom   byte = 0
	SectionType     byte = 1
	SectionImport   byte = 2
	SectionFunction byte = 3
	SectionTable    byte = 4
	SectionMemory   byte = 5
	SectionGlobal   byte = 6
	SectionExport   byte = 7
	SectionStart    byte = 8
	SectionElement  byte = 9
	SectionCode     byte = 10
	SectionData     byte = 11
)

// =============================================================================
// Instruction representation
// =============================================================================

// Instruction represents a single decoded WASM instruction.
type Instruction struct {
	// Offset is the byte offset of this instruction within the WASM module.
	Offset uint64
	// Opcode is the raw opcode byte.
	Opcode byte
	// Mnemonic is the WAT mnemonic (e.g. "i32.add", "call", "unreachable").
	Mnemonic string
	// Operands is the human-readable operand string, if any.
	Operands string
	// Size is the number of bytes this instruction occupies.
	Size int
}

// String formats the instruction in WAT style.
func (inst *Instruction) String() string {
	if inst.Operands != "" {
		return fmt.Sprintf("%s %s", inst.Mnemonic, inst.Operands)
	}
	return inst.Mnemonic
}

// =============================================================================
// Snippet represents a window of decoded instructions around a target offset.
// =============================================================================

// Snippet is a range of decoded instructions around a failing offset.
type Snippet struct {
	// Instructions is the ordered list of decoded instructions.
	Instructions []Instruction
	// TargetOffset is the byte offset of the failing instruction.
	TargetOffset uint64
	// TargetIndex is the index within Instructions that corresponds to the target.
	TargetIndex int
	// FuncIndex is the function index this snippet belongs to, if known.
	FuncIndex int
}

// Format renders the snippet as a human-readable WAT text block with an
// arrow marker on the failing instruction.
func (s *Snippet) Format() string {
	if len(s.Instructions) == 0 {
		return "  <no instructions decoded>"
	}

	var b strings.Builder
	for i, inst := range s.Instructions {
		marker := "  "
		if i == s.TargetIndex {
			marker = "> "
		}
		b.WriteString(fmt.Sprintf("%s0x%04x: %s\n", marker, inst.Offset, inst.String()))
	}
	return b.String()
}

// =============================================================================
// Disassembler
// =============================================================================

// Disassembler decodes WASM bytecode into WAT instructions.
type Disassembler struct {
	data []byte
}

// NewDisassembler creates a disassembler for the given WASM module bytes.
func NewDisassembler(wasmBytes []byte) *Disassembler {
	return &Disassembler{data: wasmBytes}
}

// IsValidWasm checks whether the data starts with the WASM magic number.
func (d *Disassembler) IsValidWasm() bool {
	if len(d.data) < 8 {
		return false
	}
	for i := 0; i < 4; i++ {
		if d.data[i] != wasmMagic[i] {
			return false
		}
	}
	version := binary.LittleEndian.Uint32(d.data[4:8])
	return version == wasmVersion
}

// DisassembleAt decodes instructions around the given byte offset,
// returning a Snippet with `contextLines` instructions before and after
// the target instruction.
func (d *Disassembler) DisassembleAt(targetOffset uint64, contextLines int) (*Snippet, error) {
	if !d.IsValidWasm() {
		return nil, fmt.Errorf("not a valid WASM module")
	}

	// Find the code section
	codeStart, codeEnd, err := d.findCodeSection()
	if err != nil {
		return nil, fmt.Errorf("failed to locate code section: %w", err)
	}

	// Decode instructions in the code section
	instructions, err := d.decodeInstructions(codeStart, codeEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to decode instructions: %w", err)
	}

	if len(instructions) == 0 {
		return &Snippet{TargetOffset: targetOffset, TargetIndex: -1}, nil
	}

	// Find the target instruction
	targetIdx := -1
	for i, inst := range instructions {
		if inst.Offset == targetOffset {
			targetIdx = i
			break
		}
		// If exact match isn't found, find the closest instruction at or before the offset
		if inst.Offset <= targetOffset && (i+1 >= len(instructions) || instructions[i+1].Offset > targetOffset) {
			targetIdx = i
			break
		}
	}

	if targetIdx < 0 {
		targetIdx = 0
	}

	// Extract context window
	start := targetIdx - contextLines
	if start < 0 {
		start = 0
	}
	end := targetIdx + contextLines + 1
	if end > len(instructions) {
		end = len(instructions)
	}

	return &Snippet{
		Instructions: instructions[start:end],
		TargetOffset: targetOffset,
		TargetIndex:  targetIdx - start,
	}, nil
}

// DecodeAll decodes all instructions in the code section.
func (d *Disassembler) DecodeAll() ([]Instruction, error) {
	if !d.IsValidWasm() {
		return nil, fmt.Errorf("not a valid WASM module")
	}

	codeStart, codeEnd, err := d.findCodeSection()
	if err != nil {
		return nil, fmt.Errorf("failed to locate code section: %w", err)
	}

	return d.decodeInstructions(codeStart, codeEnd)
}

// findCodeSection locates the code section in the WASM module and returns
// the start and end byte offsets of the section payload.
func (d *Disassembler) findCodeSection() (int, int, error) {
	pos := 8 // Skip magic + version

	for pos < len(d.data) {
		if pos >= len(d.data) {
			break
		}

		sectionID := d.data[pos]
		pos++

		sectionSize, n := decodeULEB128(d.data[pos:])
		pos += n

		if sectionID == SectionCode {
			return pos, pos + int(sectionSize), nil
		}

		pos += int(sectionSize)
	}

	return 0, 0, fmt.Errorf("code section not found")
}

// decodeInstructions decodes WASM instructions from the given byte range.
func (d *Disassembler) decodeInstructions(start, end int) ([]Instruction, error) {
	if start >= len(d.data) || end > len(d.data) || start >= end {
		return nil, fmt.Errorf("invalid byte range [%d, %d)", start, end)
	}

	// Skip the function count at the beginning of the code section
	pos := start
	_, n := decodeULEB128(d.data[pos:])
	pos += n

	var instructions []Instruction

	for pos < end {
		instOffset := uint64(pos)
		opcode := d.data[pos]
		pos++

		mnemonic, operands, consumed := decodeOpcode(opcode, d.data[pos:])
		pos += consumed

		instructions = append(instructions, Instruction{
			Offset:   instOffset,
			Opcode:   opcode,
			Mnemonic: mnemonic,
			Operands: operands,
			Size:     1 + consumed,
		})
	}

	return instructions, nil
}

// =============================================================================
// Fallback formatting
// =============================================================================

// FormatFallback produces a user-facing fallback message when source mapping
// is unavailable. It disassembles the WASM around the failing offset and
// displays the WAT snippet.
func FormatFallback(wasmBytes []byte, failingOffset uint64, contextLines int) string {
	if contextLines <= 0 {
		contextLines = 5
	}

	dis := NewDisassembler(wasmBytes)
	if !dis.IsValidWasm() {
		return fmt.Sprintf("  Source mapping unavailable. WASM offset: 0x%x\n  (could not parse WASM module)", failingOffset)
	}

	snippet, err := dis.DisassembleAt(failingOffset, contextLines)
	if err != nil {
		return fmt.Sprintf("  Source mapping unavailable. WASM offset: 0x%x\n  Disassembly error: %v", failingOffset, err)
	}

	var b strings.Builder
	b.WriteString("Source mapping unavailable. Showing WAT disassembly:\n\n")
	b.WriteString(snippet.Format())
	b.WriteString(fmt.Sprintf("\nFailing instruction at offset 0x%x\n", failingOffset))

	return b.String()
}

// =============================================================================
// WASM opcode decoding
// =============================================================================

// decodeOpcode returns the WAT mnemonic, operand string, and number of
// additional bytes consumed for operands.
func decodeOpcode(opcode byte, rest []byte) (string, string, int) {
	switch opcode {
	// Control flow
	case 0x00:
		return "unreachable", "", 0
	case 0x01:
		return "nop", "", 0
	case 0x02:
		bt, n := decodeBlockType(rest)
		return "block", bt, n
	case 0x03:
		bt, n := decodeBlockType(rest)
		return "loop", bt, n
	case 0x04:
		bt, n := decodeBlockType(rest)
		return "if", bt, n
	case 0x05:
		return "else", "", 0
	case 0x0b:
		return "end", "", 0
	case 0x0c:
		idx, n := decodeULEB128(rest)
		return "br", fmt.Sprintf("%d", idx), n
	case 0x0d:
		idx, n := decodeULEB128(rest)
		return "br_if", fmt.Sprintf("%d", idx), n
	case 0x0e:
		// br_table: count + indices + default
		count, n := decodeULEB128(rest)
		consumed := n
		for i := uint64(0); i <= count; i++ {
			_, m := decodeULEB128(rest[consumed:])
			consumed += m
		}
		return "br_table", fmt.Sprintf("(count=%d)", count), consumed
	case 0x0f:
		return "return", "", 0
	case 0x10:
		idx, n := decodeULEB128(rest)
		return "call", fmt.Sprintf("$func%d", idx), n
	case 0x11:
		typeIdx, n := decodeULEB128(rest)
		_, m := decodeULEB128(rest[n:])
		return "call_indirect", fmt.Sprintf("(type %d)", typeIdx), n + m

	// Variable access
	case 0x20:
		idx, n := decodeULEB128(rest)
		return "local.get", fmt.Sprintf("%d", idx), n
	case 0x21:
		idx, n := decodeULEB128(rest)
		return "local.set", fmt.Sprintf("%d", idx), n
	case 0x22:
		idx, n := decodeULEB128(rest)
		return "local.tee", fmt.Sprintf("%d", idx), n
	case 0x23:
		idx, n := decodeULEB128(rest)
		return "global.get", fmt.Sprintf("%d", idx), n
	case 0x24:
		idx, n := decodeULEB128(rest)
		return "global.set", fmt.Sprintf("%d", idx), n

	// Memory
	case 0x28:
		align, n1 := decodeULEB128(rest)
		offset, n2 := decodeULEB128(rest[n1:])
		return "i32.load", fmt.Sprintf("offset=%d align=%d", offset, align), n1 + n2
	case 0x29:
		align, n1 := decodeULEB128(rest)
		offset, n2 := decodeULEB128(rest[n1:])
		return "i64.load", fmt.Sprintf("offset=%d align=%d", offset, align), n1 + n2
	case 0x2a:
		align, n1 := decodeULEB128(rest)
		offset, n2 := decodeULEB128(rest[n1:])
		return "f32.load", fmt.Sprintf("offset=%d align=%d", offset, align), n1 + n2
	case 0x2b:
		align, n1 := decodeULEB128(rest)
		offset, n2 := decodeULEB128(rest[n1:])
		return "f64.load", fmt.Sprintf("offset=%d align=%d", offset, align), n1 + n2
	case 0x36:
		align, n1 := decodeULEB128(rest)
		offset, n2 := decodeULEB128(rest[n1:])
		return "i32.store", fmt.Sprintf("offset=%d align=%d", offset, align), n1 + n2
	case 0x37:
		align, n1 := decodeULEB128(rest)
		offset, n2 := decodeULEB128(rest[n1:])
		return "i64.store", fmt.Sprintf("offset=%d align=%d", offset, align), n1 + n2
	case 0x3f:
		_, n := decodeULEB128(rest)
		return "memory.size", "", n
	case 0x40:
		_, n := decodeULEB128(rest)
		return "memory.grow", "", n

	// Constants
	case 0x41:
		val, n := decodeSLEB128(rest)
		return "i32.const", fmt.Sprintf("%d", val), n
	case 0x42:
		val, n := decodeSLEB128_64(rest)
		return "i64.const", fmt.Sprintf("%d", val), n
	case 0x43:
		if len(rest) < 4 {
			return "f32.const", "?", 0
		}
		bits := binary.LittleEndian.Uint32(rest[:4])
		return "f32.const", fmt.Sprintf("%g", math.Float32frombits(bits)), 4
	case 0x44:
		if len(rest) < 8 {
			return "f64.const", "?", 0
		}
		bits := binary.LittleEndian.Uint64(rest[:8])
		return "f64.const", fmt.Sprintf("%g", math.Float64frombits(bits)), 8

	// i32 comparison
	case 0x45:
		return "i32.eqz", "", 0
	case 0x46:
		return "i32.eq", "", 0
	case 0x47:
		return "i32.ne", "", 0
	case 0x48:
		return "i32.lt_s", "", 0
	case 0x49:
		return "i32.lt_u", "", 0
	case 0x4a:
		return "i32.gt_s", "", 0
	case 0x4b:
		return "i32.gt_u", "", 0
	case 0x4c:
		return "i32.le_s", "", 0
	case 0x4d:
		return "i32.le_u", "", 0
	case 0x4e:
		return "i32.ge_s", "", 0
	case 0x4f:
		return "i32.ge_u", "", 0

	// i64 comparison
	case 0x50:
		return "i64.eqz", "", 0
	case 0x51:
		return "i64.eq", "", 0
	case 0x52:
		return "i64.ne", "", 0

	// i32 arithmetic
	case 0x67:
		return "i32.clz", "", 0
	case 0x68:
		return "i32.ctz", "", 0
	case 0x69:
		return "i32.popcnt", "", 0
	case 0x6a:
		return "i32.add", "", 0
	case 0x6b:
		return "i32.sub", "", 0
	case 0x6c:
		return "i32.mul", "", 0
	case 0x6d:
		return "i32.div_s", "", 0
	case 0x6e:
		return "i32.div_u", "", 0
	case 0x6f:
		return "i32.rem_s", "", 0
	case 0x70:
		return "i32.rem_u", "", 0
	case 0x71:
		return "i32.and", "", 0
	case 0x72:
		return "i32.or", "", 0
	case 0x73:
		return "i32.xor", "", 0
	case 0x74:
		return "i32.shl", "", 0
	case 0x75:
		return "i32.shr_s", "", 0
	case 0x76:
		return "i32.shr_u", "", 0
	case 0x77:
		return "i32.rotl", "", 0
	case 0x78:
		return "i32.rotr", "", 0

	// i64 arithmetic
	case 0x79:
		return "i64.clz", "", 0
	case 0x7a:
		return "i64.ctz", "", 0
	case 0x7c:
		return "i64.add", "", 0
	case 0x7d:
		return "i64.sub", "", 0
	case 0x7e:
		return "i64.mul", "", 0

	// Conversions
	case 0xa7:
		return "i32.wrap_i64", "", 0
	case 0xac:
		return "i64.extend_i32_s", "", 0
	case 0xad:
		return "i64.extend_i32_u", "", 0

	// drop / select
	case 0x1a:
		return "drop", "", 0
	case 0x1b:
		return "select", "", 0

	default:
		return fmt.Sprintf("unknown_0x%02x", opcode), "", 0
	}
}

// decodeBlockType decodes a block type byte and returns the WAT representation.
func decodeBlockType(data []byte) (string, int) {
	if len(data) == 0 {
		return "", 0
	}
	switch data[0] {
	case 0x40:
		return "", 1 // void block
	case 0x7f:
		return "(result i32)", 1
	case 0x7e:
		return "(result i64)", 1
	case 0x7d:
		return "(result f32)", 1
	case 0x7c:
		return "(result f64)", 1
	default:
		// Could be a type index (signed LEB128)
		_, n := decodeSLEB128(data)
		return "(type)", n
	}
}

// decodeULEB128 decodes an unsigned LEB128 integer from the given bytes.
// Returns the decoded value and the number of bytes consumed.
func decodeULEB128(data []byte) (uint64, int) {
	var result uint64
	var shift uint
	for i := 0; i < len(data); i++ {
		b := data[i]
		result |= uint64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			return result, i + 1
		}
	}
	return result, len(data)
}

// decodeSLEB128 decodes a signed LEB128 integer (32-bit).
func decodeSLEB128(data []byte) (int32, int) {
	var result int64
	var shift uint
	var b byte
	var i int
	for i = 0; i < len(data); i++ {
		b = data[i]
		result |= int64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	// Sign extend
	if shift < 32 && b&0x40 != 0 {
		result |= -(1 << shift)
	}
	return int32(result), i + 1
}

// decodeSLEB128_64 decodes a signed LEB128 integer (64-bit).
func decodeSLEB128_64(data []byte) (int64, int) {
	var result int64
	var shift uint
	var b byte
	var i int
	for i = 0; i < len(data); i++ {
		b = data[i]
		result |= int64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	if shift < 64 && b&0x40 != 0 {
		result |= -(1 << shift)
	}
	return result, i + 1
}
