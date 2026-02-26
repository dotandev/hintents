// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package dce implements dead code elimination for WASM binaries.
// It traverses the call graph from exported functions, removes
// unreferenced functions, and updates section offsets.
package dce

import (
	"encoding/binary"
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
)

// Stats holds metrics about the dead code elimination pass.
type Stats struct {
	TotalFunctions   int
	RemovedFunctions int
	OriginalSize     int
	OptimizedSize    int
}

// Eliminate strips unreachable functions from a WASM binary.
// It parses the module, builds a call graph, marks reachable functions,
// removes dead code, rewrites call indices, and reassembles the binary.
func Eliminate(wasm []byte) ([]byte, Stats, error) {
	mod, err := parseModule(wasm)
	if err != nil {
		return nil, Stats{}, err
	}

	totalLocal := len(mod.Bodies)

	roots := buildRootSet(mod)
	reachable := markReachable(mod, roots)

	reindexMap, deadSet := buildReindexMap(mod, reachable)

	if len(deadSet) == 0 {
		stats := Stats{
			TotalFunctions:   int(mod.NumImportedFuncs) + totalLocal,
			RemovedFunctions: 0,
			OriginalSize:     len(wasm),
			OptimizedSize:    len(wasm),
		}
		return wasm, stats, nil
	}

	rewrite(mod, reindexMap, deadSet)

	out, err := assemble(mod)
	if err != nil {
		return nil, Stats{}, fmt.Errorf("reassembling module: %w", err)
	}

	stats := Stats{
		TotalFunctions:   int(mod.NumImportedFuncs) + totalLocal,
		RemovedFunctions: len(deadSet),
		OriginalSize:     len(wasm),
		OptimizedSize:    len(out),
	}

	return out, stats, nil
}

// =============================================================================
// WASM constants
// =============================================================================

var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d}

const wasmVersion = 1

const (
	sectionCustom   byte = 0
	sectionType     byte = 1
	sectionImport   byte = 2
	sectionFunction byte = 3
	sectionTable    byte = 4
	sectionMemory   byte = 5
	sectionGlobal   byte = 6
	sectionExport   byte = 7
	sectionStart    byte = 8
	sectionElement  byte = 9
	sectionCode     byte = 10
	sectionData     byte = 11
)

// Export kind constants.
const exportKindFunc byte = 0

// Import kind constants.
const importKindFunc byte = 0

// =============================================================================
// Data structures
// =============================================================================

type rawSection struct {
	ID      byte
	Payload []byte
}

type importEntry struct {
	Module     string
	Name       string
	Kind       byte
	TypeIndex  uint32
	Descriptor []byte // raw descriptor bytes for non-func imports
}

type exportEntry struct {
	Name  string
	Kind  byte
	Index uint32
}

type elemSegment struct {
	TableIdx   uint32
	OffsetExpr []byte
	FuncIdxs   []uint32
}

type funcBody struct {
	Raw []byte // full body bytes including local declarations, code, and end byte
}

type wasmModule struct {
	// Sections we parse and rewrite.
	Types     [][]byte // raw bytes of each type entry
	Imports   []importEntry
	FuncTypes []uint32 // type index for each local function
	Exports   []exportEntry
	StartFunc *uint32
	Elements  []elemSegment
	Bodies    []funcBody

	// Sections we pass through unchanged, in original order.
	PassthroughSections []rawSection

	NumImportedFuncs uint32
}

// =============================================================================
// LEB128 encoding/decoding
// =============================================================================

func decodeULEB128(data []byte) (uint32, int) {
	var result uint32
	var shift uint
	for i := 0; i < len(data) && i < 5; i++ {
		b := data[i]
		result |= uint32(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			return result, i + 1
		}
	}
	return result, len(data)
}

func decodeSLEB128(data []byte) (int32, int) {
	var result int64
	var shift uint
	var b byte
	var i int
	for i = 0; i < len(data) && i < 5; i++ {
		b = data[i]
		result |= int64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	if shift < 32 && b&0x40 != 0 {
		result |= -(1 << shift)
	}
	return int32(result), i + 1
}

func decodeSLEB128_64(data []byte) (int64, int) {
	var result int64
	var shift uint
	var b byte
	var i int
	for i = 0; i < len(data) && i < 10; i++ {
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

func encodeULEB128(v uint32) []byte {
	if v == 0 {
		return []byte{0x00}
	}
	var buf []byte
	for v > 0 {
		b := byte(v & 0x7f)
		v >>= 7
		if v > 0 {
			b |= 0x80
		}
		buf = append(buf, b)
	}
	return buf
}

func encodeSLEB128(v int32) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			buf = append(buf, b)
			break
		}
		buf = append(buf, b|0x80)
	}
	return buf
}

func encodeSLEB128_64(v int64) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			buf = append(buf, b)
			break
		}
		buf = append(buf, b|0x80)
	}
	return buf
}

// =============================================================================
// Module parsing
// =============================================================================

func parseModule(wasm []byte) (*wasmModule, error) {
	if len(wasm) < 8 {
		return nil, errors.WrapWasmInvalid("file too short")
	}
	for i := 0; i < 4; i++ {
		if wasm[i] != wasmMagic[i] {
			return nil, errors.WrapWasmInvalid("bad magic bytes")
		}
	}
	version := binary.LittleEndian.Uint32(wasm[4:8])
	if version != wasmVersion {
		return nil, errors.WrapWasmInvalid("unsupported version")
	}

	mod := &wasmModule{}
	pos := 8

	for pos < len(wasm) {
		if pos >= len(wasm) {
			break
		}
		secID := wasm[pos]
		pos++

		secSize, n := decodeULEB128(wasm[pos:])
		pos += n
		if pos+int(secSize) > len(wasm) {
			return nil, errors.WrapWasmInvalid("section extends past end of file")
		}
		payload := wasm[pos : pos+int(secSize)]

		switch secID {
		case sectionType:
			types, err := parseTypeSection(payload)
			if err != nil {
				return nil, err
			}
			mod.Types = types
		case sectionImport:
			imports, numFuncs, err := parseImportSection(payload)
			if err != nil {
				return nil, err
			}
			mod.Imports = imports
			mod.NumImportedFuncs = numFuncs
		case sectionFunction:
			funcTypes, err := parseFunctionSection(payload)
			if err != nil {
				return nil, err
			}
			mod.FuncTypes = funcTypes
		case sectionExport:
			exports, err := parseExportSection(payload)
			if err != nil {
				return nil, err
			}
			mod.Exports = exports
		case sectionStart:
			if len(payload) == 0 {
				return nil, errors.WrapWasmInvalid("empty start section")
			}
			idx, _ := decodeULEB128(payload)
			mod.StartFunc = &idx
		case sectionElement:
			elems, err := parseElementSection(payload)
			if err != nil {
				return nil, err
			}
			mod.Elements = elems
		case sectionCode:
			bodies, err := parseCodeSection(payload)
			if err != nil {
				return nil, err
			}
			mod.Bodies = bodies
		default:
			// Passthrough: Table, Memory, Global, Data, Custom
			raw := make([]byte, len(payload))
			copy(raw, payload)
			mod.PassthroughSections = append(mod.PassthroughSections, rawSection{
				ID:      secID,
				Payload: raw,
			})
		}

		pos += int(secSize)
	}

	return mod, nil
}

func parseTypeSection(data []byte) ([][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	count, n := decodeULEB128(data)
	pos := n
	types := make([][]byte, 0, count)
	for i := uint32(0); i < count; i++ {
		start := pos
		if pos >= len(data) {
			return nil, errors.WrapWasmInvalid("type section truncated")
		}
		// func type marker (0x60)
		pos++
		// param count + params
		paramCount, pn := decodeULEB128(data[pos:])
		pos += pn + int(paramCount)
		// result count + results
		resultCount, rn := decodeULEB128(data[pos:])
		pos += rn + int(resultCount)
		raw := make([]byte, pos-start)
		copy(raw, data[start:pos])
		types = append(types, raw)
	}
	return types, nil
}

func parseImportSection(data []byte) ([]importEntry, uint32, error) {
	if len(data) == 0 {
		return nil, 0, nil
	}
	count, n := decodeULEB128(data)
	pos := n
	var imports []importEntry
	var numFuncs uint32
	for i := uint32(0); i < count; i++ {
		modLen, mn := decodeULEB128(data[pos:])
		pos += mn
		modName := string(data[pos : pos+int(modLen)])
		pos += int(modLen)

		nameLen, nn := decodeULEB128(data[pos:])
		pos += nn
		name := string(data[pos : pos+int(nameLen)])
		pos += int(nameLen)

		kind := data[pos]
		pos++

		entry := importEntry{Module: modName, Name: name, Kind: kind}
		if kind == importKindFunc {
			typeIdx, tn := decodeULEB128(data[pos:])
			pos += tn
			entry.TypeIndex = typeIdx
			numFuncs++
		} else {
			// For non-func imports, capture raw descriptor.
			descStart := pos
			switch kind {
			case 1: // table
				pos++ // elem type
				limits, ln := parseLimits(data[pos:])
				_ = limits
				pos += ln
			case 2: // memory
				_, ln := parseLimits(data[pos:])
				pos += ln
			case 3: // global
				pos += 2 // valtype + mutability
			}
			desc := make([]byte, pos-descStart)
			copy(desc, data[descStart:pos])
			entry.Descriptor = desc
		}

		imports = append(imports, entry)
	}
	return imports, numFuncs, nil
}

func parseLimits(data []byte) ([]byte, int) {
	if len(data) == 0 {
		return nil, 0
	}
	flag := data[0]
	pos := 1
	_, n := decodeULEB128(data[pos:]) // min
	pos += n
	if flag == 1 {
		_, n = decodeULEB128(data[pos:]) // max
		pos += n
	}
	return data[:pos], pos
}

func parseFunctionSection(data []byte) ([]uint32, error) {
	if len(data) == 0 {
		return nil, nil
	}
	count, n := decodeULEB128(data)
	pos := n
	types := make([]uint32, 0, count)
	for i := uint32(0); i < count; i++ {
		idx, tn := decodeULEB128(data[pos:])
		pos += tn
		types = append(types, idx)
	}
	return types, nil
}

func parseExportSection(data []byte) ([]exportEntry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	count, n := decodeULEB128(data)
	pos := n
	exports := make([]exportEntry, 0, count)
	for i := uint32(0); i < count; i++ {
		nameLen, nn := decodeULEB128(data[pos:])
		pos += nn
		name := string(data[pos : pos+int(nameLen)])
		pos += int(nameLen)
		kind := data[pos]
		pos++
		idx, in_ := decodeULEB128(data[pos:])
		pos += in_
		exports = append(exports, exportEntry{Name: name, Kind: kind, Index: idx})
	}
	return exports, nil
}

func parseElementSection(data []byte) ([]elemSegment, error) {
	if len(data) == 0 {
		return nil, nil
	}
	count, n := decodeULEB128(data)
	pos := n
	var elems []elemSegment
	for i := uint32(0); i < count; i++ {
		tableIdx, tn := decodeULEB128(data[pos:])
		pos += tn

		// Offset expression: read until 0x0b (end)
		exprStart := pos
		for pos < len(data) && data[pos] != 0x0b {
			pos++
		}
		if pos < len(data) {
			pos++ // consume end byte
		}
		offsetExpr := make([]byte, pos-exprStart)
		copy(offsetExpr, data[exprStart:pos])

		funcCount, fn := decodeULEB128(data[pos:])
		pos += fn

		idxs := make([]uint32, funcCount)
		for j := uint32(0); j < funcCount; j++ {
			idx, jn := decodeULEB128(data[pos:])
			pos += jn
			idxs[j] = idx
		}
		elems = append(elems, elemSegment{
			TableIdx:   tableIdx,
			OffsetExpr: offsetExpr,
			FuncIdxs:   idxs,
		})
	}
	return elems, nil
}

func parseCodeSection(data []byte) ([]funcBody, error) {
	if len(data) == 0 {
		return nil, nil
	}
	count, n := decodeULEB128(data)
	pos := n
	bodies := make([]funcBody, 0, count)
	for i := uint32(0); i < count; i++ {
		bodySize, bn := decodeULEB128(data[pos:])
		pos += bn
		if pos+int(bodySize) > len(data) {
			return nil, errors.WrapWasmInvalid("code body extends past section")
		}
		raw := make([]byte, bodySize)
		copy(raw, data[pos:pos+int(bodySize)])
		bodies = append(bodies, funcBody{Raw: raw})
		pos += int(bodySize)
	}
	return bodies, nil
}

// =============================================================================
// Module assembly
// =============================================================================

func assemble(mod *wasmModule) ([]byte, error) {
	out := make([]byte, 0, 1024)
	// Header
	out = append(out, wasmMagic...)
	out = append(out, 0x01, 0x00, 0x00, 0x00)

	// Helper to emit a section.
	emitSection := func(id byte, payload []byte) {
		out = append(out, id)
		out = append(out, encodeULEB128(uint32(len(payload)))...)
		out = append(out, payload...)
	}

	// Sections must be emitted in canonical order: 1-11, with custom (0) interspersed.
	// We emit known sections in order, and passthrough sections in their original position.

	// Collect passthrough sections by ID for ordering.
	passthroughByID := map[byte][]rawSection{}
	for _, s := range mod.PassthroughSections {
		passthroughByID[s.ID] = append(passthroughByID[s.ID], s)
	}

	emitPassthrough := func(id byte) {
		for _, s := range passthroughByID[id] {
			emitSection(s.ID, s.Payload)
		}
	}

	// Custom sections that appeared before any known section go first.
	emitPassthrough(sectionCustom)

	// Type section (1)
	if len(mod.Types) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(mod.Types)))...)
		for _, t := range mod.Types {
			payload = append(payload, t...)
		}
		emitSection(sectionType, payload)
	}

	// Import section (2)
	if len(mod.Imports) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(mod.Imports)))...)
		for _, imp := range mod.Imports {
			payload = append(payload, encodeULEB128(uint32(len(imp.Module)))...)
			payload = append(payload, []byte(imp.Module)...)
			payload = append(payload, encodeULEB128(uint32(len(imp.Name)))...)
			payload = append(payload, []byte(imp.Name)...)
			payload = append(payload, imp.Kind)
			if imp.Kind == importKindFunc {
				payload = append(payload, encodeULEB128(imp.TypeIndex)...)
			} else {
				payload = append(payload, imp.Descriptor...)
			}
		}
		emitSection(sectionImport, payload)
	}

	// Function section (3)
	if len(mod.FuncTypes) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(mod.FuncTypes)))...)
		for _, t := range mod.FuncTypes {
			payload = append(payload, encodeULEB128(t)...)
		}
		emitSection(sectionFunction, payload)
	}

	// Table (4)
	emitPassthrough(sectionTable)

	// Memory (5)
	emitPassthrough(sectionMemory)

	// Global (6)
	emitPassthrough(sectionGlobal)

	// Export section (7)
	if len(mod.Exports) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(mod.Exports)))...)
		for _, exp := range mod.Exports {
			payload = append(payload, encodeULEB128(uint32(len(exp.Name)))...)
			payload = append(payload, []byte(exp.Name)...)
			payload = append(payload, exp.Kind)
			payload = append(payload, encodeULEB128(exp.Index)...)
		}
		emitSection(sectionExport, payload)
	}

	// Start section (8)
	if mod.StartFunc != nil {
		emitSection(sectionStart, encodeULEB128(*mod.StartFunc))
	}

	// Element section (9)
	if len(mod.Elements) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(mod.Elements)))...)
		for _, elem := range mod.Elements {
			payload = append(payload, encodeULEB128(elem.TableIdx)...)
			payload = append(payload, elem.OffsetExpr...)
			payload = append(payload, encodeULEB128(uint32(len(elem.FuncIdxs)))...)
			for _, idx := range elem.FuncIdxs {
				payload = append(payload, encodeULEB128(idx)...)
			}
		}
		emitSection(sectionElement, payload)
	}

	// Code section (10)
	if len(mod.Bodies) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(mod.Bodies)))...)
		for _, body := range mod.Bodies {
			payload = append(payload, encodeULEB128(uint32(len(body.Raw)))...)
			payload = append(payload, body.Raw...)
		}
		emitSection(sectionCode, payload)
	}

	// Data (11)
	emitPassthrough(sectionData)

	return out, nil
}

// =============================================================================
// Call graph analysis
// =============================================================================

// scanCalls scans a function body for call instructions and returns the
// set of target function indices.
func scanCalls(body []byte) map[uint32]bool {
	targets := map[uint32]bool{}

	// Skip local declarations.
	pos := 0
	if pos >= len(body) {
		return targets
	}
	localDeclCount, n := decodeULEB128(body[pos:])
	pos += n
	for i := uint32(0); i < localDeclCount; i++ {
		_, cn := decodeULEB128(body[pos:]) // count
		pos += cn
		pos++ // valtype
	}

	// Scan instructions.
	for pos < len(body) {
		opcode := body[pos]
		pos++

		switch opcode {
		// 0 extra bytes
		case 0x00, 0x01, 0x05, 0x0b, 0x0f, 0x1a, 0x1b:
			// unreachable, nop, else, end, return, drop, select
		// block/loop/if: 1 blocktype byte
		case 0x02, 0x03, 0x04:
			if pos < len(body) {
				bt := body[pos]
				if bt == 0x40 || bt == 0x7f || bt == 0x7e || bt == 0x7d || bt == 0x7c {
					pos++
				} else {
					_, sn := decodeSLEB128(body[pos:])
					pos += sn
				}
			}
		// 1 ULEB128 (non-call)
		case 0x0c, 0x0d: // br, br_if
			_, n := decodeULEB128(body[pos:])
			pos += n
		case 0x10: // call
			idx, n := decodeULEB128(body[pos:])
			pos += n
			targets[idx] = true
		case 0x20, 0x21, 0x22, 0x23, 0x24: // local.get/set/tee, global.get/set
			_, n := decodeULEB128(body[pos:])
			pos += n
		case 0x3f, 0x40: // memory.size, memory.grow
			_, n := decodeULEB128(body[pos:])
			pos += n
		// 2 ULEB128
		case 0x11: // call_indirect
			_, n1 := decodeULEB128(body[pos:])
			pos += n1
			_, n2 := decodeULEB128(body[pos:])
			pos += n2
		// memory load/store: 2 ULEB128 (align + offset)
		case 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
			0x30, 0x31, 0x32, 0x33, 0x34, 0x35,
			0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e:
			_, n1 := decodeULEB128(body[pos:])
			pos += n1
			_, n2 := decodeULEB128(body[pos:])
			pos += n2
		// br_table
		case 0x0e:
			count, cn := decodeULEB128(body[pos:])
			pos += cn
			for j := uint32(0); j <= count; j++ {
				_, ln := decodeULEB128(body[pos:])
				pos += ln
			}
		// i32.const (SLEB128)
		case 0x41:
			_, sn := decodeSLEB128(body[pos:])
			pos += sn
		// i64.const (SLEB128-64)
		case 0x42:
			_, sn := decodeSLEB128_64(body[pos:])
			pos += sn
		// f32.const (4 bytes)
		case 0x43:
			pos += 4
		// f64.const (8 bytes)
		case 0x44:
			pos += 8
		default:
			// All other MVP opcodes (0x45-0xC4) take 0 extra bytes.
			// This covers comparison, arithmetic, and conversion ops.
		}
	}

	return targets
}

// buildRootSet returns the set of function indices that serve as GC roots.
func buildRootSet(mod *wasmModule) map[uint32]bool {
	roots := map[uint32]bool{}

	// Exported functions.
	for _, exp := range mod.Exports {
		if exp.Kind == exportKindFunc {
			roots[exp.Index] = true
		}
	}

	// Start function.
	if mod.StartFunc != nil {
		roots[*mod.StartFunc] = true
	}

	// Element segment entries (indirect call targets).
	for _, elem := range mod.Elements {
		for _, idx := range elem.FuncIdxs {
			roots[idx] = true
		}
	}

	return roots
}

// markReachable performs BFS from the root set, scanning function bodies
// for call targets, and returns the full set of reachable function indices.
func markReachable(mod *wasmModule, roots map[uint32]bool) map[uint32]bool {
	reachable := map[uint32]bool{}
	worklist := make([]uint32, 0, len(roots))

	for idx := range roots {
		if !reachable[idx] {
			reachable[idx] = true
			worklist = append(worklist, idx)
		}
	}

	for len(worklist) > 0 {
		idx := worklist[0]
		worklist = worklist[1:]

		// Imported functions have no body to scan.
		if idx < mod.NumImportedFuncs {
			continue
		}

		localIdx := idx - mod.NumImportedFuncs
		if int(localIdx) >= len(mod.Bodies) {
			continue
		}

		callTargets := scanCalls(mod.Bodies[localIdx].Raw)
		for target := range callTargets {
			if !reachable[target] {
				reachable[target] = true
				worklist = append(worklist, target)
			}
		}
	}

	return reachable
}

// buildReindexMap builds a mapping from old function index to new function index.
// It also returns the set of dead local function indices (in local index space).
func buildReindexMap(mod *wasmModule, reachable map[uint32]bool) (map[uint32]uint32, map[int]bool) {
	reindexMap := map[uint32]uint32{}
	deadLocalIndices := map[int]bool{}

	// Imported functions keep their indices.
	for i := uint32(0); i < mod.NumImportedFuncs; i++ {
		reindexMap[i] = i
	}

	newIdx := mod.NumImportedFuncs
	for i := 0; i < len(mod.Bodies); i++ {
		oldIdx := mod.NumImportedFuncs + uint32(i)
		if reachable[oldIdx] {
			reindexMap[oldIdx] = newIdx
			newIdx++
		} else {
			deadLocalIndices[i] = true
		}
	}

	return reindexMap, deadLocalIndices
}

// =============================================================================
// Rewriting
// =============================================================================

// rewriteBody rewrites call operands in a function body using the reindex map.
// Returns the new body bytes.
func rewriteBody(body []byte, reindexMap map[uint32]uint32) []byte {
	var out []byte

	// Copy local declarations unchanged.
	pos := 0
	localDeclCount, n := decodeULEB128(body[pos:])
	out = append(out, body[pos:pos+n]...)
	pos += n
	for i := uint32(0); i < localDeclCount; i++ {
		_, cn := decodeULEB128(body[pos:])
		out = append(out, body[pos:pos+cn]...)
		pos += cn
		out = append(out, body[pos]) // valtype
		pos++
	}

	// Rewrite instructions.
	for pos < len(body) {
		opcode := body[pos]
		out = append(out, opcode)
		pos++

		switch opcode {
		case 0x00, 0x01, 0x05, 0x0b, 0x0f, 0x1a, 0x1b:
			// 0 extra bytes
		case 0x02, 0x03, 0x04:
			if pos < len(body) {
				bt := body[pos]
				if bt == 0x40 || bt == 0x7f || bt == 0x7e || bt == 0x7d || bt == 0x7c {
					out = append(out, bt)
					pos++
				} else {
					val, sn := decodeSLEB128(body[pos:])
					out = append(out, encodeSLEB128(val)...)
					pos += sn
				}
			}
		case 0x0c, 0x0d: // br, br_if
			val, n := decodeULEB128(body[pos:])
			out = append(out, encodeULEB128(val)...)
			pos += n
		case 0x10: // call — rewrite target index
			oldIdx, n := decodeULEB128(body[pos:])
			pos += n
			newIdx := oldIdx
			if mapped, ok := reindexMap[oldIdx]; ok {
				newIdx = mapped
			}
			out = append(out, encodeULEB128(newIdx)...)
		case 0x20, 0x21, 0x22, 0x23, 0x24:
			val, n := decodeULEB128(body[pos:])
			out = append(out, encodeULEB128(val)...)
			pos += n
		case 0x3f, 0x40:
			val, n := decodeULEB128(body[pos:])
			out = append(out, encodeULEB128(val)...)
			pos += n
		case 0x11: // call_indirect
			typeIdx, n1 := decodeULEB128(body[pos:])
			pos += n1
			tableIdx, n2 := decodeULEB128(body[pos:])
			pos += n2
			out = append(out, encodeULEB128(typeIdx)...)
			out = append(out, encodeULEB128(tableIdx)...)
		case 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
			0x30, 0x31, 0x32, 0x33, 0x34, 0x35,
			0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e:
			align, n1 := decodeULEB128(body[pos:])
			pos += n1
			offset, n2 := decodeULEB128(body[pos:])
			pos += n2
			out = append(out, encodeULEB128(align)...)
			out = append(out, encodeULEB128(offset)...)
		case 0x0e: // br_table
			count, cn := decodeULEB128(body[pos:])
			pos += cn
			out = append(out, encodeULEB128(count)...)
			for j := uint32(0); j <= count; j++ {
				val, ln := decodeULEB128(body[pos:])
				pos += ln
				out = append(out, encodeULEB128(val)...)
			}
		case 0x41:
			val, sn := decodeSLEB128(body[pos:])
			pos += sn
			out = append(out, encodeSLEB128(val)...)
		case 0x42:
			val, sn := decodeSLEB128_64(body[pos:])
			pos += sn
			out = append(out, encodeSLEB128_64(val)...)
		case 0x43:
			out = append(out, body[pos:pos+4]...)
			pos += 4
		case 0x44:
			out = append(out, body[pos:pos+8]...)
			pos += 8
		default:
			// 0 extra bytes for all other MVP opcodes (0x45-0xC4).
		}
	}

	return out
}

// rewrite mutates the module in place: removes dead functions and rewrites indices.
func rewrite(mod *wasmModule, reindexMap map[uint32]uint32, deadLocalIndices map[int]bool) {
	// Remove dead function type entries and bodies.
	newFuncTypes := make([]uint32, 0, len(mod.FuncTypes)-len(deadLocalIndices))
	newBodies := make([]funcBody, 0, len(mod.Bodies)-len(deadLocalIndices))
	for i := 0; i < len(mod.Bodies); i++ {
		if deadLocalIndices[i] {
			continue
		}
		newFuncTypes = append(newFuncTypes, mod.FuncTypes[i])
		newBodies = append(newBodies, mod.Bodies[i])
	}
	mod.FuncTypes = newFuncTypes
	mod.Bodies = newBodies

	// Rewrite call operands in surviving bodies.
	for i := range mod.Bodies {
		mod.Bodies[i].Raw = rewriteBody(mod.Bodies[i].Raw, reindexMap)
	}

	// Rewrite export indices.
	for i := range mod.Exports {
		if mod.Exports[i].Kind == exportKindFunc {
			if newIdx, ok := reindexMap[mod.Exports[i].Index]; ok {
				mod.Exports[i].Index = newIdx
			}
		}
	}

	// Rewrite start function index.
	if mod.StartFunc != nil {
		if newIdx, ok := reindexMap[*mod.StartFunc]; ok {
			mod.StartFunc = &newIdx
		}
	}

	// Rewrite element segment function indices.
	for i := range mod.Elements {
		for j := range mod.Elements[i].FuncIdxs {
			if newIdx, ok := reindexMap[mod.Elements[i].FuncIdxs[j]]; ok {
				mod.Elements[i].FuncIdxs[j] = newIdx
			}
		}
	}
}
