// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package wasmopt

import (
	"bytes"
	"fmt"
)

const (
	sectionImport   = 2
	sectionFunction = 3
	sectionExport   = 7
	sectionStart    = 8
	sectionElement  = 9
	sectionCode     = 10
)

var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

type section struct {
	id      byte
	payload []byte
}

// Report summarizes a DCE run.
type Report struct {
	OriginalDefinedFunctions int
	KeptDefinedFunctions     int
	RemovedDefinedFunctions  int
}

// EliminateDeadCode removes unreachable, non-imported functions from a WASM module.
// Reachability starts from exported functions, start function, and element segment function refs.
func EliminateDeadCode(module []byte) ([]byte, Report, error) {
	sections, err := parseSections(module)
	if err != nil {
		return nil, Report{}, err
	}

	importPayload, _ := findSection(sections, sectionImport)
	functionPayload, hasFunction := findSection(sections, sectionFunction)
	codePayload, hasCode := findSection(sections, sectionCode)
	if !hasFunction || !hasCode {
		return module, Report{}, nil
	}

	importedFuncCount, err := parseImportedFunctionCount(importPayload)
	if err != nil {
		return nil, Report{}, fmt.Errorf("parse import section: %w", err)
	}

	typeIdxs, err := parseFunctionSection(functionPayload)
	if err != nil {
		return nil, Report{}, fmt.Errorf("parse function section: %w", err)
	}

	codeBodies, err := parseCodeSection(codePayload)
	if err != nil {
		return nil, Report{}, fmt.Errorf("parse code section: %w", err)
	}
	if len(typeIdxs) != len(codeBodies) {
		return nil, Report{}, fmt.Errorf("function/code section length mismatch: %d vs %d", len(typeIdxs), len(codeBodies))
	}

	roots := make(map[uint32]struct{})
	exportPayload, hasExport := findSection(sections, sectionExport)
	if hasExport {
		rootsFromExport, err := parseExportFunctionRoots(exportPayload)
		if err != nil {
			return nil, Report{}, fmt.Errorf("parse export section: %w", err)
		}
		for k := range rootsFromExport {
			roots[k] = struct{}{}
		}
	}

	startPayload, hasStart := findSection(sections, sectionStart)
	if hasStart {
		startIdx, err := parseStartFunction(startPayload)
		if err != nil {
			return nil, Report{}, fmt.Errorf("parse start section: %w", err)
		}
		roots[startIdx] = struct{}{}
	}

	elementPayload, hasElement := findSection(sections, sectionElement)
	if hasElement {
		elementRoots, _, err := rewriteElementSection(elementPayload, identityMap, true)
		if err != nil {
			return nil, Report{}, fmt.Errorf("parse element section: %w", err)
		}
		for k := range elementRoots {
			roots[k] = struct{}{}
		}
	}

	totalFuncs := importedFuncCount + uint32(len(codeBodies))
	edges := make([][]uint32, len(codeBodies))
	for i, body := range codeBodies {
		_, calls, err := rewriteCodeBody(body, identityMap)
		if err != nil {
			return nil, Report{}, fmt.Errorf("parse code body %d: %w", i, err)
		}
		edges[i] = calls
	}

	reachable := make([]bool, totalFuncs)
	queue := make([]uint32, 0, len(roots))
	for r := range roots {
		if r < totalFuncs && !reachable[r] {
			reachable[r] = true
			queue = append(queue, r)
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur < importedFuncCount {
			continue
		}
		defIdx := cur - importedFuncCount
		if int(defIdx) >= len(edges) {
			continue
		}
		for _, callee := range edges[defIdx] {
			if callee < totalFuncs && !reachable[callee] {
				reachable[callee] = true
				queue = append(queue, callee)
			}
		}
	}

	keep := make([]bool, len(codeBodies))
	keptCount := 0
	for i := range codeBodies {
		idx := importedFuncCount + uint32(i)
		if reachable[idx] {
			keep[i] = true
			keptCount++
		}
	}

	report := Report{
		OriginalDefinedFunctions: len(codeBodies),
		KeptDefinedFunctions:     keptCount,
		RemovedDefinedFunctions:  len(codeBodies) - keptCount,
	}
	if report.RemovedDefinedFunctions == 0 {
		return module, report, nil
	}

	newIndex, ok := buildFunctionRemap(importedFuncCount, keep)
	if !ok {
		return nil, Report{}, fmt.Errorf("invalid function remap")
	}

	newTypeIdxs := make([]uint32, 0, keptCount)
	newBodies := make([][]byte, 0, keptCount)
	for i := range codeBodies {
		if !keep[i] {
			continue
		}
		newTypeIdxs = append(newTypeIdxs, typeIdxs[i])
		rebuiltBody, _, err := rewriteCodeBody(codeBodies[i], newIndex)
		if err != nil {
			return nil, Report{}, fmt.Errorf("rewrite code body %d: %w", i, err)
		}
		newBodies = append(newBodies, rebuiltBody)
	}

	replacements := make(map[byte][]byte)
	replacements[sectionFunction] = encodeFunctionSection(newTypeIdxs)
	replacements[sectionCode] = encodeCodeSection(newBodies)

	if hasExport {
		rewrittenExport, err := rewriteExportSection(exportPayload, newIndex)
		if err != nil {
			return nil, Report{}, fmt.Errorf("rewrite export section: %w", err)
		}
		replacements[sectionExport] = rewrittenExport
	}

	if hasStart {
		rewrittenStart, err := rewriteStartSection(startPayload, newIndex)
		if err != nil {
			return nil, Report{}, fmt.Errorf("rewrite start section: %w", err)
		}
		replacements[sectionStart] = rewrittenStart
	}

	if hasElement {
		_, rewrittenElement, err := rewriteElementSection(elementPayload, newIndex, false)
		if err != nil {
			return nil, Report{}, fmt.Errorf("rewrite element section: %w", err)
		}
		replacements[sectionElement] = rewrittenElement
	}

	optimized, err := rebuildModule(sections, replacements)
	if err != nil {
		return nil, Report{}, err
	}
	return optimized, report, nil
}

func parseSections(module []byte) ([]section, error) {
	if len(module) < len(wasmMagic) || !bytes.Equal(module[:len(wasmMagic)], wasmMagic) {
		return nil, fmt.Errorf("invalid wasm header")
	}
	pos := len(wasmMagic)
	var sections []section
	for pos < len(module) {
		id := module[pos]
		pos++
		size, n, err := readU32(module, pos)
		if err != nil {
			return nil, err
		}
		pos += n
		if pos+int(size) > len(module) {
			return nil, fmt.Errorf("section length out of bounds")
		}
		payload := make([]byte, int(size))
		copy(payload, module[pos:pos+int(size)])
		sections = append(sections, section{id: id, payload: payload})
		pos += int(size)
	}
	return sections, nil
}

func rebuildModule(sections []section, replacements map[byte][]byte) ([]byte, error) {
	var out bytes.Buffer
	out.Write(wasmMagic)
	used := make(map[byte]bool)
	for _, s := range sections {
		payload := s.payload
		if s.id != 0 && !used[s.id] {
			if repl, ok := replacements[s.id]; ok {
				payload = repl
			}
			used[s.id] = true
		}
		out.WriteByte(s.id)
		out.Write(encodeU32(uint32(len(payload))))
		out.Write(payload)
	}
	return out.Bytes(), nil
}

func findSection(sections []section, id byte) ([]byte, bool) {
	for _, s := range sections {
		if s.id == id {
			return s.payload, true
		}
	}
	return nil, false
}

func parseImportedFunctionCount(payload []byte) (uint32, error) {
	if len(payload) == 0 {
		return 0, nil
	}
	pos := 0
	count, n, err := readU32(payload, pos)
	if err != nil {
		return 0, err
	}
	pos += n
	var fnCount uint32
	for i := uint32(0); i < count; i++ {
		var err error
		pos, err = skipName(payload, pos)
		if err != nil {
			return 0, err
		}
		pos, err = skipName(payload, pos)
		if err != nil {
			return 0, err
		}
		if pos >= len(payload) {
			return 0, fmt.Errorf("import section truncated")
		}
		kind := payload[pos]
		pos++
		switch kind {
		case 0x00:
			_, n, err := readU32(payload, pos)
			if err != nil {
				return 0, err
			}
			pos += n
			fnCount++
		case 0x01:
			var err error
			pos, err = skipTableType(payload, pos)
			if err != nil {
				return 0, err
			}
		case 0x02:
			var err error
			pos, err = skipLimits(payload, pos)
			if err != nil {
				return 0, err
			}
		case 0x03:
			if pos+2 > len(payload) {
				return 0, fmt.Errorf("global import truncated")
			}
			pos += 2
		case 0x04:
			if pos >= len(payload) {
				return 0, fmt.Errorf("tag import truncated")
			}
			pos++
			_, n, err := readU32(payload, pos)
			if err != nil {
				return 0, err
			}
			pos += n
		default:
			return 0, fmt.Errorf("unsupported import kind %d", kind)
		}
	}
	if pos != len(payload) {
		return 0, fmt.Errorf("import section has trailing bytes")
	}
	return fnCount, nil
}

func skipTableType(data []byte, pos int) (int, error) {
	if pos >= len(data) {
		return 0, fmt.Errorf("table type truncated")
	}
	pos++
	return skipLimits(data, pos)
}

func skipLimits(data []byte, pos int) (int, error) {
	flags, n, err := readU32(data, pos)
	if err != nil {
		return 0, err
	}
	pos += n
	_, n, err = readU32(data, pos)
	if err != nil {
		return 0, err
	}
	pos += n
	if flags&0x01 != 0 {
		_, n, err = readU32(data, pos)
		if err != nil {
			return 0, err
		}
		pos += n
	}
	return pos, nil
}

func parseFunctionSection(payload []byte) ([]uint32, error) {
	pos := 0
	count, n, err := readU32(payload, pos)
	if err != nil {
		return nil, err
	}
	pos += n
	out := make([]uint32, 0, count)
	for i := uint32(0); i < count; i++ {
		v, n, err := readU32(payload, pos)
		if err != nil {
			return nil, err
		}
		pos += n
		out = append(out, v)
	}
	if pos != len(payload) {
		return nil, fmt.Errorf("function section has trailing bytes")
	}
	return out, nil
}

func encodeFunctionSection(typeIdxs []uint32) []byte {
	var b bytes.Buffer
	b.Write(encodeU32(uint32(len(typeIdxs))))
	for _, t := range typeIdxs {
		b.Write(encodeU32(t))
	}
	return b.Bytes()
}

func parseCodeSection(payload []byte) ([][]byte, error) {
	pos := 0
	count, n, err := readU32(payload, pos)
	if err != nil {
		return nil, err
	}
	pos += n
	out := make([][]byte, 0, count)
	for i := uint32(0); i < count; i++ {
		sz, n, err := readU32(payload, pos)
		if err != nil {
			return nil, err
		}
		pos += n
		if pos+int(sz) > len(payload) {
			return nil, fmt.Errorf("code body %d out of bounds", i)
		}
		body := make([]byte, int(sz))
		copy(body, payload[pos:pos+int(sz)])
		out = append(out, body)
		pos += int(sz)
	}
	if pos != len(payload) {
		return nil, fmt.Errorf("code section has trailing bytes")
	}
	return out, nil
}

func encodeCodeSection(bodies [][]byte) []byte {
	var b bytes.Buffer
	b.Write(encodeU32(uint32(len(bodies))))
	for _, body := range bodies {
		b.Write(encodeU32(uint32(len(body))))
		b.Write(body)
	}
	return b.Bytes()
}

func parseExportFunctionRoots(payload []byte) (map[uint32]struct{}, error) {
	pos := 0
	count, n, err := readU32(payload, pos)
	if err != nil {
		return nil, err
	}
	pos += n
	roots := make(map[uint32]struct{})
	for i := uint32(0); i < count; i++ {
		var err error
		pos, err = skipName(payload, pos)
		if err != nil {
			return nil, err
		}
		if pos+1 > len(payload) {
			return nil, fmt.Errorf("export entry truncated")
		}
		kind := payload[pos]
		pos++
		idx, n, err := readU32(payload, pos)
		if err != nil {
			return nil, err
		}
		pos += n
		if kind == 0x00 {
			roots[idx] = struct{}{}
		}
	}
	if pos != len(payload) {
		return nil, fmt.Errorf("export section has trailing bytes")
	}
	return roots, nil
}

func rewriteExportSection(payload []byte, remap func(uint32) (uint32, bool)) ([]byte, error) {
	pos := 0
	count, n, err := readU32(payload, pos)
	if err != nil {
		return nil, err
	}
	pos += n
	var out bytes.Buffer
	out.Write(encodeU32(count))
	for i := uint32(0); i < count; i++ {
		nameStart := pos
		pos, err = skipName(payload, pos)
		if err != nil {
			return nil, err
		}
		out.Write(payload[nameStart:pos])
		if pos+1 > len(payload) {
			return nil, fmt.Errorf("export entry truncated")
		}
		kind := payload[pos]
		pos++
		out.WriteByte(kind)
		idx, n, err := readU32(payload, pos)
		if err != nil {
			return nil, err
		}
		pos += n
		if kind == 0x00 {
			newIdx, ok := remap(idx)
			if !ok {
				return nil, fmt.Errorf("export points to removed function %d", idx)
			}
			out.Write(encodeU32(newIdx))
		} else {
			out.Write(encodeU32(idx))
		}
	}
	if pos != len(payload) {
		return nil, fmt.Errorf("export section has trailing bytes")
	}
	return out.Bytes(), nil
}

func parseStartFunction(payload []byte) (uint32, error) {
	idx, n, err := readU32(payload, 0)
	if err != nil {
		return 0, err
	}
	if n != len(payload) {
		return 0, fmt.Errorf("start section has trailing bytes")
	}
	return idx, nil
}

func rewriteStartSection(payload []byte, remap func(uint32) (uint32, bool)) ([]byte, error) {
	idx, err := parseStartFunction(payload)
	if err != nil {
		return nil, err
	}
	newIdx, ok := remap(idx)
	if !ok {
		return nil, fmt.Errorf("start function removed")
	}
	return encodeU32(newIdx), nil
}

func rewriteElementSection(payload []byte, remap func(uint32) (uint32, bool), collectRoots bool) (map[uint32]struct{}, []byte, error) {
	roots := make(map[uint32]struct{})
	pos := 0
	count, n, err := readU32(payload, pos)
	if err != nil {
		return nil, nil, err
	}
	pos += n
	var out bytes.Buffer
	out.Write(encodeU32(count))
	for i := uint32(0); i < count; i++ {
		flags, n, err := readU32(payload, pos)
		if err != nil {
			return nil, nil, err
		}
		pos += n
		out.Write(encodeU32(flags))

		switch flags {
		case 0:
			expr, np, err := readConstExpr(payload, pos)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			rewrittenExpr, refs, err := rewriteExpr(expr, remap)
			if err != nil {
				return nil, nil, err
			}
			if collectRoots {
				for _, r := range refs {
					roots[r] = struct{}{}
				}
			}
			out.Write(rewrittenExpr)
			vecBytes, np, err := rewriteFuncIdxVector(payload, pos, remap, roots, collectRoots)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			out.Write(vecBytes)
		case 1:
			if pos >= len(payload) {
				return nil, nil, fmt.Errorf("element kind truncated")
			}
			out.WriteByte(payload[pos])
			pos++
			vecBytes, np, err := rewriteFuncIdxVector(payload, pos, remap, roots, collectRoots)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			out.Write(vecBytes)
		case 2:
			tableIdx, n, err := readU32(payload, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			out.Write(encodeU32(tableIdx))
			expr, np, err := readConstExpr(payload, pos)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			rewrittenExpr, refs, err := rewriteExpr(expr, remap)
			if err != nil {
				return nil, nil, err
			}
			if collectRoots {
				for _, r := range refs {
					roots[r] = struct{}{}
				}
			}
			out.Write(rewrittenExpr)
			if pos >= len(payload) {
				return nil, nil, fmt.Errorf("element kind truncated")
			}
			out.WriteByte(payload[pos])
			pos++
			vecBytes, np, err := rewriteFuncIdxVector(payload, pos, remap, roots, collectRoots)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			out.Write(vecBytes)
		case 3:
			if pos >= len(payload) {
				return nil, nil, fmt.Errorf("element kind truncated")
			}
			out.WriteByte(payload[pos])
			pos++
			vecBytes, np, err := rewriteFuncIdxVector(payload, pos, remap, roots, collectRoots)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			out.Write(vecBytes)
		case 4:
			expr, np, err := readConstExpr(payload, pos)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			rewrittenExpr, refs, err := rewriteExpr(expr, remap)
			if err != nil {
				return nil, nil, err
			}
			if collectRoots {
				for _, r := range refs {
					roots[r] = struct{}{}
				}
			}
			out.Write(rewrittenExpr)
			vecBytes, np, refs2, err := rewriteExprVector(payload, pos, remap)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			if collectRoots {
				for _, r := range refs2 {
					roots[r] = struct{}{}
				}
			}
			out.Write(vecBytes)
		case 5:
			if pos >= len(payload) {
				return nil, nil, fmt.Errorf("element reftype truncated")
			}
			out.WriteByte(payload[pos])
			pos++
			vecBytes, np, refs2, err := rewriteExprVector(payload, pos, remap)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			if collectRoots {
				for _, r := range refs2 {
					roots[r] = struct{}{}
				}
			}
			out.Write(vecBytes)
		case 6:
			tableIdx, n, err := readU32(payload, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			out.Write(encodeU32(tableIdx))
			expr, np, err := readConstExpr(payload, pos)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			rewrittenExpr, refs, err := rewriteExpr(expr, remap)
			if err != nil {
				return nil, nil, err
			}
			if collectRoots {
				for _, r := range refs {
					roots[r] = struct{}{}
				}
			}
			out.Write(rewrittenExpr)
			if pos >= len(payload) {
				return nil, nil, fmt.Errorf("element reftype truncated")
			}
			out.WriteByte(payload[pos])
			pos++
			vecBytes, np, refs2, err := rewriteExprVector(payload, pos, remap)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			if collectRoots {
				for _, r := range refs2 {
					roots[r] = struct{}{}
				}
			}
			out.Write(vecBytes)
		case 7:
			if pos >= len(payload) {
				return nil, nil, fmt.Errorf("element reftype truncated")
			}
			out.WriteByte(payload[pos])
			pos++
			vecBytes, np, refs2, err := rewriteExprVector(payload, pos, remap)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			if collectRoots {
				for _, r := range refs2 {
					roots[r] = struct{}{}
				}
			}
			out.Write(vecBytes)
		default:
			return nil, nil, fmt.Errorf("unsupported element flags %d", flags)
		}
	}
	if pos != len(payload) {
		return nil, nil, fmt.Errorf("element section has trailing bytes")
	}
	return roots, out.Bytes(), nil
}

func rewriteFuncIdxVector(data []byte, pos int, remap func(uint32) (uint32, bool), roots map[uint32]struct{}, collectRoots bool) ([]byte, int, error) {
	count, n, err := readU32(data, pos)
	if err != nil {
		return nil, 0, err
	}
	pos += n
	mapped := make([]uint32, 0, count)
	for i := uint32(0); i < count; i++ {
		idx, n, err := readU32(data, pos)
		if err != nil {
			return nil, 0, err
		}
		pos += n
		if collectRoots {
			roots[idx] = struct{}{}
		}
		newIdx, ok := remap(idx)
		if ok {
			mapped = append(mapped, newIdx)
		}
	}
	var out bytes.Buffer
	out.Write(encodeU32(uint32(len(mapped))))
	for _, v := range mapped {
		out.Write(encodeU32(v))
	}
	return out.Bytes(), pos, nil
}

func rewriteExprVector(data []byte, pos int, remap func(uint32) (uint32, bool)) ([]byte, int, []uint32, error) {
	count, n, err := readU32(data, pos)
	if err != nil {
		return nil, 0, nil, err
	}
	pos += n
	var out bytes.Buffer
	out.Write(encodeU32(count))
	refs := make([]uint32, 0)
	for i := uint32(0); i < count; i++ {
		expr, np, err := readConstExpr(data, pos)
		if err != nil {
			return nil, 0, nil, err
		}
		pos = np
		rewritten, exprRefs, err := rewriteExpr(expr, remap)
		if err != nil {
			return nil, 0, nil, err
		}
		refs = append(refs, exprRefs...)
		out.Write(rewritten)
	}
	return out.Bytes(), pos, refs, nil
}

func readConstExpr(data []byte, pos int) ([]byte, int, error) {
	start := pos
	for pos < len(data) {
		if data[pos] == 0x0b {
			pos++
			expr := make([]byte, pos-start)
			copy(expr, data[start:pos])
			return expr, pos, nil
		}
		pos++
	}
	return nil, 0, fmt.Errorf("unterminated const expr")
}

func rewriteCodeBody(body []byte, remap func(uint32) (uint32, bool)) ([]byte, []uint32, error) {
	pos := 0
	localDeclCount, n, err := readU32(body, pos)
	if err != nil {
		return nil, nil, err
	}
	pos += n
	for i := uint32(0); i < localDeclCount; i++ {
		_, n, err := readU32(body, pos)
		if err != nil {
			return nil, nil, err
		}
		pos += n
		if pos >= len(body) {
			return nil, nil, fmt.Errorf("local decl truncated")
		}
		pos++
	}
	if pos > len(body) {
		return nil, nil, fmt.Errorf("invalid body local decls")
	}

	localsPrefix := body[:pos]
	expr := body[pos:]
	rewrittenExpr, calls, err := rewriteExpr(expr, remap)
	if err != nil {
		return nil, nil, err
	}
	newBody := make([]byte, 0, len(localsPrefix)+len(rewrittenExpr))
	newBody = append(newBody, localsPrefix...)
	newBody = append(newBody, rewrittenExpr...)
	return newBody, calls, nil
}

func rewriteExpr(expr []byte, remap func(uint32) (uint32, bool)) ([]byte, []uint32, error) {
	var out bytes.Buffer
	calls := make([]uint32, 0)
	pos := 0
	for pos < len(expr) {
		op := expr[pos]
		pos++
		out.WriteByte(op)
		switch op {
		case 0x02, 0x03, 0x04:
			start := pos
			np, err := skipBlockType(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos = np
			out.Write(expr[start:pos])
		case 0x0c, 0x0d, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26:
			start := pos
			_, n, err := readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			out.Write(expr[start:pos])
		case 0x0e:
			count, n, err := readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			start := pos
			pos += n
			for i := uint32(0); i < count+1; i++ {
				_, n, err := readU32(expr, pos)
				if err != nil {
					return nil, nil, err
				}
				pos += n
			}
			out.Write(expr[start:pos])
		case 0x10, 0x12, 0xd2:
			idx, n, err := readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			calls = append(calls, idx)
			newIdx, ok := remap(idx)
			if !ok {
				return nil, nil, fmt.Errorf("function index %d was removed", idx)
			}
			out.Write(encodeU32(newIdx))
		case 0x11, 0x13:
			start := pos
			_, n, err := readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			_, n, err = readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			out.Write(expr[start:pos])
		case 0x1c:
			start := pos
			c, n, err := readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			if pos+int(c) > len(expr) {
				return nil, nil, fmt.Errorf("select type vector out of bounds")
			}
			pos += int(c)
			out.Write(expr[start:pos])
		case 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
			0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37,
			0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e:
			start := pos
			_, n, err := readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			_, n, err = readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			out.Write(expr[start:pos])
		case 0x3f, 0x40:
			start := pos
			_, n, err := readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			out.Write(expr[start:pos])
		case 0x41:
			start := pos
			_, n, err := readSLEB32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			out.Write(expr[start:pos])
		case 0x42:
			start := pos
			_, n, err := readSLEB64(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			out.Write(expr[start:pos])
		case 0x43:
			if pos+4 > len(expr) {
				return nil, nil, fmt.Errorf("f32.const truncated")
			}
			out.Write(expr[pos : pos+4])
			pos += 4
		case 0x44:
			if pos+8 > len(expr) {
				return nil, nil, fmt.Errorf("f64.const truncated")
			}
			out.Write(expr[pos : pos+8])
			pos += 8
		case 0xd0:
			if pos >= len(expr) {
				return nil, nil, fmt.Errorf("ref.null truncated")
			}
			out.WriteByte(expr[pos])
			pos++
		case 0xfc:
			start := pos
			sub, n, err := readU32(expr, pos)
			if err != nil {
				return nil, nil, err
			}
			pos += n
			switch sub {
			case 0, 1, 2, 3, 4, 5, 6, 7:
			case 8, 10, 12, 14:
				_, n, err = readU32(expr, pos)
				if err != nil {
					return nil, nil, err
				}
				pos += n
				_, n, err = readU32(expr, pos)
				if err != nil {
					return nil, nil, err
				}
				pos += n
			case 9, 11, 13, 15, 16, 17:
				_, n, err = readU32(expr, pos)
				if err != nil {
					return nil, nil, err
				}
				pos += n
			default:
				return nil, nil, fmt.Errorf("unsupported 0xfc subopcode %d", sub)
			}
			out.Write(expr[start:pos])
		case 0xfd:
			return nil, nil, fmt.Errorf("unsupported SIMD opcode prefix 0xfd")
		case 0xfe:
			return nil, nil, fmt.Errorf("unsupported atomic opcode prefix 0xfe")
		default:
			if isNoImmediateOpcode(op) {
				continue
			}
			return nil, nil, fmt.Errorf("unsupported opcode 0x%02x", op)
		}
	}
	return out.Bytes(), calls, nil
}

func isNoImmediateOpcode(op byte) bool {
	switch op {
	case 0x00, 0x01, 0x05, 0x0b, 0x0f, 0x1a, 0x1b, 0x1d, 0x1e, 0x1f, 0xd1:
		return true
	}
	if op >= 0x45 && op <= 0xc4 {
		return true
	}
	return false
}

func buildFunctionRemap(importedFuncCount uint32, keep []bool) (func(uint32) (uint32, bool), bool) {
	newIdx := make([]uint32, len(keep))
	next := importedFuncCount
	for i, k := range keep {
		if k {
			newIdx[i] = next
			next++
		}
	}
	return func(old uint32) (uint32, bool) {
		if old < importedFuncCount {
			return old, true
		}
		def := old - importedFuncCount
		if int(def) >= len(keep) || !keep[def] {
			return 0, false
		}
		return newIdx[def], true
	}, true
}

func identityMap(v uint32) (uint32, bool) {
	return v, true
}

func skipName(data []byte, pos int) (int, error) {
	l, n, err := readU32(data, pos)
	if err != nil {
		return 0, err
	}
	pos += n
	if pos+int(l) > len(data) {
		return 0, fmt.Errorf("name out of bounds")
	}
	return pos + int(l), nil
}

func skipBlockType(data []byte, pos int) (int, error) {
	if pos >= len(data) {
		return 0, fmt.Errorf("blocktype truncated")
	}
	b := data[pos]
	switch b {
	case 0x40, 0x7f, 0x7e, 0x7d, 0x7c, 0x7b, 0x70, 0x6f:
		return pos + 1, nil
	default:
		_, n, err := readSLEB33(data, pos)
		if err != nil {
			return 0, err
		}
		return pos + n, nil
	}
}

func readU32(data []byte, pos int) (uint32, int, error) {
	var v uint32
	shift := uint(0)
	for i := 0; i < 5; i++ {
		if pos+i >= len(data) {
			return 0, 0, fmt.Errorf("uleb128 out of bounds")
		}
		b := data[pos+i]
		v |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return v, i + 1, nil
		}
		shift += 7
	}
	return 0, 0, fmt.Errorf("uleb128 overflow")
}

func readSLEB32(data []byte, pos int) (int32, int, error) {
	val, n, err := readSLEB(data, pos, 32)
	return int32(val), n, err
}

func readSLEB64(data []byte, pos int) (int64, int, error) {
	val, n, err := readSLEB(data, pos, 64)
	return val, n, err
}

func readSLEB33(data []byte, pos int) (int64, int, error) {
	return readSLEB(data, pos, 33)
}

func readSLEB(data []byte, pos int, bits uint) (int64, int, error) {
	var result int64
	shift := uint(0)
	var b byte
	for i := 0; i < 10; i++ {
		if pos+i >= len(data) {
			return 0, 0, fmt.Errorf("sleb128 out of bounds")
		}
		b = data[pos+i]
		result |= int64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			if shift < bits && (b&0x40) != 0 {
				result |= ^0 << shift
			}
			return result, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("sleb128 overflow")
}

func encodeU32(v uint32) []byte {
	var out [5]byte
	i := 0
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out[i] = b
		i++
		if v == 0 {
			break
		}
	}
	return out[:i]
}
