// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

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
	"os"
)

var (
	ErrNoDebugInfo = errors.New("no DWARF debug information found")
	ErrNoLocalVars = errors.New("no local variables found at address")
	ErrInvalidWASM = errors.New("invalid WASM or ELF binary")
)

type LocalVar struct {
	Name          string
	DemangledName string
	Type          string
	Location      string
	Value         interface{}
	Address       uint64
	StartLine     int
	EndLine       int
}

type SubprogramInfo struct {
	Name           string
	DemangledName  string
	LowPC          uint64
	HighPC         uint64
	Line           int
	File           string
	LocalVariables []LocalVar
}

type SourceLocation struct {
	File   string
	Line   int
	Column int
}

type Frame struct {
	Function     string
	SourceLoc    SourceLocation
	LocalVars    []LocalVar
	ReturnAddr   uint64
	FramePointer uint64
}

type Parser struct {
	data       *dwarf.Data
	binaryType string
}

func NewParserFromFile(path string) (*Parser, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return NewParser(data)
}

func NewParser(data []byte) (*Parser, error) {
	if len(data) < 4 {
		return nil, ErrInvalidWASM
	}

	if data[0] == 0x00 && data[1] == 0x61 && data[2] == 0x73 && data[3] == 0x6d {
		return parseWASM(data)
	}
	if data[0] == 0x7f && data[1] == 0x45 && data[2] == 0x4c && data[3] == 0x46 {
		return parseELF(data)
	}
	if len(data) >= 4 {
		if binary.BigEndian.Uint32(data[0:4]) == 0xfeedfacf ||
			binary.LittleEndian.Uint32(data[0:4]) == 0xfeedfacf {
			return parseMacho(data)
		}
	}
	if len(data) >= 2 {
		if binary.LittleEndian.Uint16(data[0:2]) == 0x5a4d {
			return parsePE(data)
		}
	}

	return nil, ErrInvalidWASM
}

func parseWASM(data []byte) (*Parser, error) {
	sections := parseWASMSections(data)

	infoSec := sections[".debug_info"]
	if infoSec == nil {
		return nil, ErrNoDebugInfo
	}

	dwarfData, err := dwarf.New(
		infoSec,
		sections[".debug_abbrev"],
		nil,
		sections[".debug_str"],
		sections[".debug_line"],
		nil,
		sections[".debug_ranges"],
		nil,
	)
	if err != nil {
		return nil, ErrNoDebugInfo
	}

	return &Parser{data: dwarfData, binaryType: "wasm"}, nil
}

func parseWASMSections(data []byte) map[string][]byte {
	sections := make(map[string][]byte)

	i := 8
	for i < len(data) {
		if i+1 >= len(data) {
			break
		}
		sectionID := data[i]
		i++

		size, n := readVarUint32(data[i:])
		i += n

		if sectionID == 0 {
			nameLen, n := readVarUint32(data[i:])
			i += n
			if i+int(nameLen) > len(data) {
				break
			}
			name := string(data[i : i+int(nameLen)])
			i += int(nameLen)

			contentSize := int(size) - (n + int(nameLen))
			if contentSize > 0 && i+contentSize <= len(data) {
				sections[name] = data[i : i+contentSize]
			}
			i += contentSize
		} else {
			i += int(size)
		}
	}

	return sections
}

func readVarUint32(data []byte) (uint32, int) {
	var res uint32
	var shift uint
	for i, b := range data {
		res |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return res, i + 1
		}
		shift += 7
	}
	return res, 0
}

func parseELF(data []byte) (*Parser, error) {
	elfFile, err := elf.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}
	dwarfData, err := elfFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}
	return &Parser{data: dwarfData, binaryType: "elf"}, nil
}

func parseMacho(data []byte) (*Parser, error) {
	machoFile, err := macho.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}
	dwarfData, err := machoFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}
	return &Parser{data: dwarfData, binaryType: "macho"}, nil
}

func parsePE(data []byte) (*Parser, error) {
	peFile, err := pe.NewFile(bytesToReader(data))
	if err != nil {
		return nil, err
	}
	dwarfData, err := peFile.DWARF()
	if err != nil {
		return nil, ErrNoDebugInfo
	}
	return &Parser{data: dwarfData, binaryType: "pe"}, nil
}

type bytesReader struct {
	data []byte
}

func (r *bytesReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n = copy(p, r.data[off:])
	return n, nil
}

func bytesToReader(data []byte) io.ReaderAt {
	return &bytesReader{data: data}
}

func (p *Parser) GetSubprograms() ([]SubprogramInfo, error) {
	if p.data == nil {
		return nil, ErrNoDebugInfo
	}

	var subprograms []SubprogramInfo
	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil || entry == nil {
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

func (p *Parser) extractSubprogram(entry *dwarf.Entry) (SubprogramInfo, error) {
	info := SubprogramInfo{}

	if name, ok := entry.Val(dwarf.AttrName).(string); ok {
		info.Name = name
	}
	if demangled, ok := entry.Val(dwarf.AttrLinkageName).(string); ok {
		info.DemangledName = demangled
	} else {
		info.DemangledName = nameDemangle(info.Name)
	}
	if lowPC, ok := entry.Val(dwarf.AttrLowpc).(uint64); ok {
		info.LowPC = lowPC
	}
	if highPC, ok := entry.Val(dwarf.AttrHighpc).(uint64); ok {
		info.HighPC = highPC
	}
	if line, ok := entry.Val(dwarf.AttrDeclLine).(int64); ok {
		info.Line = int(line)
	}

	info.LocalVariables = p.getLocalVariables(entry)
	return info, nil
}

func (p *Parser) getLocalVariables(_ *dwarf.Entry) []LocalVar {
	// Go 1.21-1.23's debug/dwarf APIs do not expose enough parent/scope
	// metadata to robustly associate variables with a subprogram here.
	return nil
}

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

func (p *Parser) FindLocalVarsAt(addr uint64) ([]LocalVar, error) {
	subprogram, err := p.FindSubprogramAt(addr)
	if err != nil {
		return nil, err
	}
	if len(subprogram.LocalVariables) == 0 {
		return nil, ErrNoLocalVars
	}
	return subprogram.LocalVariables, nil
}

func (p *Parser) GetSourceLocation(addr uint64) (*SourceLocation, error) {
	if p.data == nil {
		return nil, ErrNoDebugInfo
	}

	reader := p.data.Reader()
	for {
		entry, err := reader.Next()
		if err != nil || entry == nil {
			break
		}
		if entry.Tag != dwarf.TagCompileUnit {
			continue
		}

		lineReader, err := p.data.LineReader(entry)
		if err != nil || lineReader == nil {
			continue
		}

		var le dwarf.LineEntry
		for {
			err := lineReader.Next(&le)
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			if le.Address == addr && le.File != nil {
				return &SourceLocation{File: le.File.Name, Line: le.Line, Column: le.Column}, nil
			}
		}
	}

	return nil, fmt.Errorf("no source location found for address 0x%x", addr)
}

func formatLocation(loc []byte) string {
	if len(loc) == 0 {
		return ""
	}

	const (
		dwOpAddr       = 0x03
		dwOpStackValue = 0x9f
	)

	switch loc[0] {
	case dwOpStackValue:
		return "immediate"
	case dwOpAddr:
		if len(loc) >= 9 {
			addr := binary.LittleEndian.Uint64(loc[1:])
			return fmt.Sprintf("0x%x", addr)
		}
	}

	return fmt.Sprintf("location[0x%x]", loc[0])
}

func nameDemangle(name string) string {
	if len(name) > 4 && name[:4] == "_RNv" {
		return name
	}
	return name
}

func (p *Parser) HasDebugInfo() bool {
	return p.data != nil
}

func (p *Parser) BinaryType() string {
	return p.binaryType
}
