// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package dwarf provides DWARF debug information parsing for Soroban contract debugging.
// It extracts local variable information from WASM files with debug symbols to help
// reconstruct variable values at the point of a trap (e.g., memory-out-of-bounds).
package dwarf

import (
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrNoDebugInfo indicates the binary doesn't contain DWARF debug information
	ErrNoDebugInfo = errors.New("no DWARF debug information found")
	// ErrNoLocalVars indicates no local variables were found at the given address
	ErrNoLocalVars = errors.New("no local variables found at address")
	// ErrInvalidWASM indicates the file is not a valid WASM or ELF binary
	ErrInvalidWASM = errors.New("invalid WASM or ELF binary")
)

// LocalVar represents a local variable at a specific program location
type LocalVar struct {
	Name         string      // Variable name (may be mangled)
	DemangledName string    // Demangled name for display
	Type         string      // Type name
	Location     string      // DWARF location description
	Value        interface{} // Computed value (if available)
	Address      uint64      // Memory address (if applicable)
	StartLine    int         // Source line where variable is in scope
	EndLine      int         // Source line where variable goes out of scope
}

// SubprogramInfo represents a function/subprogram's debug information
type SubprogramInfo struct {
	Name           string
	DemangledName  string
	LowPC          uint64
	HighPC         uint64
	Line           int
	File           string
	LocalVariables []LocalVar
}

// SourceLocation represents a location in the source code
type SourceLocation struct {
	File   string
	Line   int
	Column int
}

// Frame represents a stack frame with local variable information
type Frame struct {
	Function     string
	SourceLoc    SourceLocation
	LocalVars    []LocalVar
	ReturnAddr   uint64
	FramePointer uint64
}

// Parser handles DWARF debug information extraction
type Parser struct {
	data       *dwarf.Data
	reader     *dwarf.Reader
	binaryType string // "wasm", "elf", "macho", "pe"
}

// NewParser creates a new DWARF parser from a binary
func NewParser(data []byte) (*Parser, error) {
	if len(data) < 4 {
		return nil, ErrInvalidWASM
	}

	// Detect binary type and try to parse DWARF info
	// Check for WASM (WebAssembly) magic number
	if data[0] == 0x00 && data[1] == 0x61 && data[2] == 0x73 && data[3] == 0x6d {
		return parseWASM(data)
	}

	// Try ELF
	if data[0] == 0x7f && data[1] == 0x45 && data[2] == 0x4c && data[3] == 0x46 {
		return parseELF(data)
	}

	// Try Mach-O
	if len(data) >= 4 {
		if binary.BigEndian.Uint32(data[0:4]) == 0xfeedfacf ||
			binary.LittleEndian.Uint32(data[0:4]) == 0xfeedfacf {
			return parseMacho(data)
		}
	}

	// Try PE
	if len(data) >= 2 {
		if binary.LittleEndian.Uint16(data[0:2]) == 0x5a4d {
			return parsePE(data)
		}
	}

	return nil, ErrInvalidWASM
}

// parseWASM parses DWARF info from a WASM binary
func parseWASM(data []byte) (*Parser, error) {
	sections := parseWASMSections(data)
	
	var dwarfData *dwarf.Data
	var err error

	infoSec := sections[".debug_info"]
	lineSec := sections[".debug_line"]
	strSec := sections[".debug_str"]
	abbrevSec := sections[".debug_abbrev"]
	rangesSec := sections[".debug_ranges"]

	infoSec, ok := sections[".debug_info"]
	if !ok || len(infoSec) == 0 {
		return nil, ErrNoDebugInfo
	}
	abbrevSec, _ := sections[".debug_abbrev"]
	lineSec, _ := sections[".debug_line"]
	rangesSec, _ := sections[".debug_ranges"]
	strSec, _ := sections[".debug_str"]

	dwarfData, err := dwarf.New(abbrevSec, nil, nil, infoSec, lineSec, nil, rangesSec, strSec)
	if dwarfData == nil || err != nil {
		// No DWARF info in WASM
		return nil, ErrNoDebugInfo
	}

	return &Parser{
		data:       dwarfData,
		binaryType: "wasm",
	}, nil
}

// parseWASMSections parses the section table of a WASM binary and returns a
// map of custom-section names to their content bytes.  Only custom sections
// (section ID 0) are collected; all other sections are skipped.
func parseWASMSections(data []byte) map[string][]byte {
	sections := make(map[string][]byte)
	
	if len(data) < 8 {
		return sections
	}
	
	// For now, return empty sections
	// A full WASM parser would be complex and is beyond scope
	// The important thing is we don't crash

	pos := 8 // skip 4-byte magic + 4-byte version
	for pos < len(data) {
		// Read section ID (1 byte).
		if pos >= len(data) {
			break
		}
		sectionID := data[pos]
		pos++

		// Read section size as an unsigned LEB128 varint.
		sectionSize, n := readULEB128(data, pos)
		if n == 0 {
			break
		}
		pos += n

		sectionEnd := pos + int(sectionSize)
		if sectionEnd > len(data) {
			break
		}

		if sectionID == 0 { // custom section
			// The first field inside the custom section is the name, also
			// length-prefixed with a LEB128 integer.
			nameLen, m := readULEB128(data, pos)
			if m == 0 || pos+m+int(nameLen) > sectionEnd {
				pos = sectionEnd
				continue
			}
			nameStart := pos + m
			name := string(data[nameStart : nameStart+int(nameLen)])
			content := data[nameStart+int(nameLen) : sectionEnd]
			sections[name] = content
		}

		pos = sectionEnd
	}

	return sections
}

// readULEB128 decodes an unsigned little-endian base-128 integer starting at
// data[pos].  It returns the value and the number of bytes consumed.  If the
// data is truncated or malformed it returns (0, 0).
func readULEB128(data []byte, pos int) (uint64, int) {
	var result uint64
	var shift uint
	for i := pos; i < len(data); i++ {
		b := data[i]
		result |= uint64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			return result, i - pos + 1
		}
		if shift >= 64 {
			return 0, 0 // overflow
		}
	}
	return 0, 0 // truncated
}

// parseELF parses DWARF info from an ELF binary
func parseELF(data []byte) (*Parser, error) {
	// Create a temporary file to use debug/elf package
	elfFile, err := elf.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}

	dwarfData, err := elfFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}

	return &Parser{
		data:       dwarfData,
		binaryType: "elf",
	}, nil
}

// parseMacho parses DWARF info from a Mach-O binary
func parseMacho(data []byte) (*Parser, error) {
	machoFile, err := macho.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}

	dwarfData, err := machoFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}

	return &Parser{
		data:       dwarfData,
		binaryType: "macho",
	}, nil
}

// parsePE parses DWARF info from a PE binary
func parsePE(data []byte) (*Parser, error) {
	peFile, err := pe.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}

	dwarfData, err := peFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}

	return &Parser{
		data:       dwarfData,
		binaryType: "pe",
	}, nil
}

// bytesToReader converts a byte slice to an io.ReaderAt
type bytesReader struct {
	data []byte
	off  int
}

func (r *bytesReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n = copy(p, r.data[int(off):])
	return n, nil
}

func bytesToReader(data []byte) io.ReaderAt {
	return &bytesReader{data: data}
}

// GetSubprograms returns all subprograms (functions) in the debug info
func (p *Parser) GetSubprograms() ([]SubprogramInfo, error) {
	if p.data == nil {
		return nil, ErrNoDebugInfo
	}

	var subprograms []SubprogramInfo

	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil {
			break
		}
		if entry == nil {
			break
		}

		if entry.Tag == dwarf.TagSubprogram {
			subprogram, err := p.extractSubprogram(entry)
			if err == nil {
				subprograms = append(subprograms, subprogram)
			}
		}
	}

	return subprograms, nil
}

// extractSubprogram extracts information about a function/subprogram
func (p *Parser) extractSubprogram(entry *dwarf.Entry) (SubprogramInfo, error) {
	info := SubprogramInfo{}

	// Extract name
	if name, ok := entry.Val(dwarf.AttrName).(string); ok {
		info.Name = name
	}

	// Extract demangled name (if available)
	if demangled, ok := entry.Val(dwarf.AttrLinkageName).(string); ok {
		info.DemangledName = demangled
	} else {
		info.DemangledName = nameDemangle(info.Name)
	}

	// Extract low PC
	if lowPC, ok := entry.Val(dwarf.AttrLowpc).(uint64); ok {
		info.LowPC = lowPC
	}

	// Extract high PC
	if highPC, ok := entry.Val(dwarf.AttrHighpc).(uint64); ok {
		info.HighPC = highPC
	}

	// Extract line number
	if line, ok := entry.Val(dwarf.AttrDeclLine).(int64); ok {
		info.Line = int(line)
	}

	// Extract file
	if file, ok := entry.Val(dwarf.AttrDeclFile).(string); ok {
		info.File = file
	}

	// Get local variables for this subprogram
	info.LocalVariables = p.getLocalVariables(entry)

	return info, nil
}

// getLocalVariables extracts local variables for a subprogram by reading the
// consecutive child entries that follow the subprogram entry in the DWARF tree.
func (p *Parser) getLocalVariables(subprog *dwarf.Entry) []LocalVar {
	var locals []LocalVar

	// Seek to just after the subprogram entry and read its children.
	reader := p.data.Reader()
	reader.Seek(subprog.Offset)

	// Skip the subprogram entry itself.
	if _, err := reader.Next(); err != nil {
		return locals
	}

	for {
		entry, err := reader.Next()
		if err != nil || entry == nil {
			break
		}
		// A tag of 0 marks the end of the subprogram's child list.
		if entry.Tag == 0 {
			break
		}

		if entry.Tag == dwarf.TagVariable || entry.Tag == dwarf.TagFormalParameter {
			local := p.extractLocalVar(entry)
			if local.Name != "" {
				locals = append(locals, local)
			}
		}
	}

	return locals
}

// extractLocalVar extracts information about a local variable
func (p *Parser) extractLocalVar(entry *dwarf.Entry) LocalVar {
	local := LocalVar{}

	// Get variable name
	if name, ok := entry.Val(dwarf.AttrName).(string); ok {
		local.Name = name
		local.DemangledName = nameDemangle(name)
	}

	// Get type
	if typ, ok := entry.Val(dwarf.AttrType).(dwarf.Offset); ok {
		local.Type = p.getTypeName(typ)
	}

	// Get location
	if loc, ok := entry.Val(dwarf.AttrLocation).([]byte); ok {
		local.Location = formatLocation(loc)
	}

	// Get line number
	if line, ok := entry.Val(dwarf.AttrDeclLine).(int64); ok {
		local.StartLine = int(line)
		local.EndLine = int(line)
	}

	return local
}

// getTypeName returns the name of a type given its offset
func (p *Parser) getTypeName(typeOffset dwarf.Offset) string {
	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil || entry == nil {
			break
		}

		if entry.Offset == typeOffset {
			switch entry.Tag {
			case dwarf.TagTypedef:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagBaseType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagStructType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagUnionType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagEnumerationType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return name
				}
			case dwarf.TagPointerType:
				if name, ok := entry.Val(dwarf.AttrName).(string); ok {
					return "*" + name
				}
			}
		}

		if entry.Tag == 0 {
			break
		}
	}

	return "unknown"
}

// FindSubprogramAt finds the subprogram containing the given address
func (p *Parser) FindSubprogramAt(addr uint64) (*SubprogramInfo, error) {
	subprograms, err := p.GetSubprograms()
	if err != nil {
		return nil, err
	}

	for i := range subprograms {
		s := &subprograms[i]
		if addr >= s.LowPC && addr < s.HighPC {
			return s, nil
		}
	}

	return nil, fmt.Errorf("no subprogram found at address 0x%x", addr)
}

// FindLocalVarsAt finds local variables visible at the given address
func (p *Parser) FindLocalVarsAt(addr uint64) ([]LocalVar, error) {
	subprogram, err := p.FindSubprogramAt(addr)
	if err != nil {
		return nil, err
	}

	// Filter variables that are in scope at this address
	var inScope []LocalVar
	for _, v := range subprogram.LocalVariables {
		if addr >= uint64(v.StartLine) {
			inScope = append(inScope, v)
		}
	}

	if len(inScope) == 0 {
		return nil, ErrNoLocalVars
	}

	return inScope, nil
}

// GetSourceLocation finds the source location for a given address
func (p *Parser) GetSourceLocation(addr uint64) (*SourceLocation, error) {
	if p.data == nil {
		return nil, ErrNoDebugInfo
	}

	// Iterate compile units and use LineReader to map addr -> source line.
	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil || entry == nil {
			break
		}

		if entry.Tag == dwarf.TagCompileUnit {
			// For now, return a basic location
			// In a full implementation, we would parse line tables
			return &SourceLocation{
				File:   "unknown",
				Line:   0,
				Column: 0,
			}, nil
			// Use LineReader (the real stdlib API) to walk line table entries.
			lr, err := p.data.LineReader(entry)
			if err != nil || lr == nil {
				reader.SkipChildren()
				continue
			}
			loc := p.findLineForAddr(lr, addr)
			if loc != nil {
				return loc, nil
			}
		}

		reader.SkipChildren()
	}

	return nil, fmt.Errorf("no source location found for address 0x%x", addr)
}

// findLineForAddr searches a LineReader for the entry that covers addr.
func (p *Parser) findLineForAddr(lr *dwarf.LineReader, addr uint64) *SourceLocation {
	var prev dwarf.LineEntry
	hasPrev := false

	for {
		var le dwarf.LineEntry
		if err := lr.Next(&le); err != nil {
			break
		}

		// If the previous entry's address range covers addr, use that entry.
		if hasPrev && prev.Address <= addr && addr < le.Address {
			if prev.File != nil {
				return &SourceLocation{
					File:   prev.File.Name,
					Line:   prev.Line,
					Column: prev.Column,
				}
			}
		}

		if le.EndSequence {
			hasPrev = false
			continue
		}
		prev = le
		hasPrev = true
	}

	return nil
}

// formatLocation formats a DWARF location expression byte sequence into a
// human-readable string.  Only a small subset of opcodes is handled; the rest
// fall through to a hex dump.
//
// DWARF location-expression opcodes used here:
//
//	0x03  DW_OP_addr          – followed by a target-address-sized literal
//	0x9f  DW_OP_stack_value   – the value is the top of the expression stack
//	0x00  (no-op / terminator in some older encodings)
// DWARF location expression opcodes (DW_OP_*) used in formatLocation.
// These are defined in the DWARF spec and are not exported by debug/dwarf.
const (
	dwOpAddr       = 0x03 // DW_OP_addr — constant address
	dwOpStackValue = 0x9f // DW_OP_stack_value — value is on the expression stack
	dwOpLit0       = 0x30 // DW_OP_lit0 — literal 0 (marks end-of-list in some contexts)
)

// formatLocation formats a DWARF location description
func formatLocation(loc []byte) string {
	if len(loc) == 0 {
		return ""
	}

	// DWARF location opcodes
	const (
		DW_OP_addr         = 0x03
		DW_OP_stack_value  = 0x9f
		DW_OP_implicit_value = 0x9e
	)

	switch loc[0] {
	case DW_OP_stack_value:
		return "immediate"
	case DW_OP_addr:
	case dwOpStackValue:
		return "immediate"
	case opAddr:
		if len(loc) >= 9 {
			addr := binary.LittleEndian.Uint64(loc[1:])
			return fmt.Sprintf("0x%x", addr)
		}
	case DW_OP_implicit_value:
		return "implicit"
	case dwOpLit0:
		return "end"
	}

	return fmt.Sprintf("location[0x%x]", loc[0])
}

// nameDemangle attempts to demangle a name (simplified version)
func nameDemangle(name string) string {
	// Basic Rust demangling: _RNv... -> original name
	if len(name) > 4 && name[:4] == "_RNv" {
		// For now, just return the original
		return name
	}
	return name
}

// HasDebugInfo returns true if the binary contains DWARF debug information
func (p *Parser) HasDebugInfo() bool {
	return p.data != nil
}

// BinaryType returns the type of binary being parsed
func (p *Parser) BinaryType() string {
	return p.binaryType
}

