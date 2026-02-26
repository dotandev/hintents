// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package wasmopt

import (
	"bytes"
	"testing"
)

func TestEliminateDeadCode_RemovesUnreachableAndRewritesCalls(t *testing.T) {
	module := buildModule(
		buildTypeSectionSingleEmpty(),
		buildFunctionSection([]uint32{0, 0, 0, 0}),
		buildExportSection(map[string]uint32{"main": 0}),
		buildCodeSection([][]byte{
			buildBodyCall(2), // func 0 calls func 2
			buildBodyNoop(),  // func 1 dead
			buildBodyNoop(),  // func 2 live
			buildBodyNoop(),  // func 3 dead
		}),
	)

	optimized, report, err := EliminateDeadCode(module)
	if err != nil {
		t.Fatalf("EliminateDeadCode failed: %v", err)
	}

	if report.OriginalDefinedFunctions != 4 || report.KeptDefinedFunctions != 2 || report.RemovedDefinedFunctions != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}

	funcTypes, err := parseFunctionSectionPayloadFromModule(optimized)
	if err != nil {
		t.Fatalf("parse function section: %v", err)
	}
	if len(funcTypes) != 2 {
		t.Fatalf("expected 2 functions after DCE, got %d", len(funcTypes))
	}

	callIdx, err := firstCallIndexInFunction(optimized, 0)
	if err != nil {
		t.Fatalf("read call index: %v", err)
	}
	if callIdx != 1 {
		t.Fatalf("expected remapped call index 1, got %d", callIdx)
	}
}

func TestEliminateDeadCode_NoChangesWhenEverythingReachable(t *testing.T) {
	module := buildModule(
		buildTypeSectionSingleEmpty(),
		buildFunctionSection([]uint32{0, 0}),
		buildExportSection(map[string]uint32{"main": 0}),
		buildCodeSection([][]byte{
			buildBodyCall(1),
			buildBodyNoop(),
		}),
	)

	optimized, report, err := EliminateDeadCode(module)
	if err != nil {
		t.Fatalf("EliminateDeadCode failed: %v", err)
	}
	if report.RemovedDefinedFunctions != 0 {
		t.Fatalf("expected 0 removed functions, got %d", report.RemovedDefinedFunctions)
	}
	if !bytes.Equal(module, optimized) {
		t.Fatalf("expected identical module when no functions are removed")
	}
}

func buildModule(sections ...section) []byte {
	var out bytes.Buffer
	out.Write(wasmMagic)
	for _, s := range sections {
		out.WriteByte(s.id)
		out.Write(encodeU32(uint32(len(s.payload))))
		out.Write(s.payload)
	}
	return out.Bytes()
}

func buildTypeSectionSingleEmpty() section {
	// one type: () -> ()
	payload := []byte{
		0x01,       // vec len
		0x60, 0x00, // func type, no params
		0x00, // no results
	}
	return section{id: 1, payload: payload}
}

func buildFunctionSection(typeIndices []uint32) section {
	return section{id: sectionFunction, payload: encodeFunctionSection(typeIndices)}
}

func buildExportSection(exports map[string]uint32) section {
	var payload bytes.Buffer
	payload.Write(encodeU32(uint32(len(exports))))
	for name, fnIdx := range exports {
		payload.Write(encodeU32(uint32(len(name))))
		payload.WriteString(name)
		payload.WriteByte(0x00) // func export
		payload.Write(encodeU32(fnIdx))
	}
	return section{id: sectionExport, payload: payload.Bytes()}
}

func buildCodeSection(bodies [][]byte) section {
	return section{id: sectionCode, payload: encodeCodeSection(bodies)}
}

func buildBodyNoop() []byte {
	return []byte{
		0x00, // local decl count
		0x01, // nop
		0x0b, // end
	}
}

func buildBodyCall(idx uint32) []byte {
	var body bytes.Buffer
	body.WriteByte(0x00) // local decl count
	body.WriteByte(0x10) // call
	body.Write(encodeU32(idx))
	body.WriteByte(0x0b) // end
	return body.Bytes()
}

func parseFunctionSectionPayloadFromModule(module []byte) ([]uint32, error) {
	sections, err := parseSections(module)
	if err != nil {
		return nil, err
	}
	payload, ok := findSection(sections, sectionFunction)
	if !ok {
		return nil, nil
	}
	return parseFunctionSection(payload)
}

func firstCallIndexInFunction(module []byte, fn int) (uint32, error) {
	sections, err := parseSections(module)
	if err != nil {
		return 0, err
	}
	payload, ok := findSection(sections, sectionCode)
	if !ok {
		return 0, nil
	}
	bodies, err := parseCodeSection(payload)
	if err != nil {
		return 0, err
	}
	body := bodies[fn]
	// local decl count must be zero in this fixture
	pos := 1
	if body[pos] != 0x10 {
		return 0, nil
	}
	pos++
	idx, _, err := readU32(body, pos)
	return idx, err
}
