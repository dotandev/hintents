// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package dce

import (
	"testing"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test WASM module builder
// =============================================================================

// testModuleBuilder constructs synthetic WASM binaries for testing.
type testModuleBuilder struct {
	types    [][]byte // raw type entries
	imports  [][]byte // raw import entries
	funcIdxs []uint32 // type indices for local functions
	bodies   [][]byte // function bodies (local decls + code + end)
	exports  [][]byte // raw export entries
	start    *uint32
	elements [][]byte // raw element segment entries
	custom   [][]byte // raw custom section payloads (each preceded by name)
	tables   []byte   // raw table section payload
	memories []byte   // raw memory section payload
}

func newTestModule() *testModuleBuilder {
	return &testModuleBuilder{}
}

// addFuncType adds a () -> () function type.
func (b *testModuleBuilder) addFuncType() *testModuleBuilder {
	b.types = append(b.types, []byte{0x60, 0x00, 0x00})
	return b
}

// addFuncImport adds a function import.
func (b *testModuleBuilder) addFuncImport(module, name string, typeIdx uint32) *testModuleBuilder {
	var entry []byte
	entry = append(entry, encodeULEB128(uint32(len(module)))...)
	entry = append(entry, []byte(module)...)
	entry = append(entry, encodeULEB128(uint32(len(name)))...)
	entry = append(entry, []byte(name)...)
	entry = append(entry, importKindFunc)
	entry = append(entry, encodeULEB128(typeIdx)...)
	b.imports = append(b.imports, entry)
	return b
}

// addFunction adds a local function with the given body instructions.
// The body should NOT include local declarations or end byte.
func (b *testModuleBuilder) addFunction(typeIdx uint32, bodyInstructions []byte) *testModuleBuilder {
	b.funcIdxs = append(b.funcIdxs, typeIdx)
	// Body = 0 local declarations + instructions + end
	body := []byte{0x00} // 0 local declarations
	body = append(body, bodyInstructions...)
	body = append(body, 0x0b) // end
	b.bodies = append(b.bodies, body)
	return b
}

// addExport adds a function export.
func (b *testModuleBuilder) addExport(name string, funcIdx uint32) *testModuleBuilder {
	var entry []byte
	entry = append(entry, encodeULEB128(uint32(len(name)))...)
	entry = append(entry, []byte(name)...)
	entry = append(entry, exportKindFunc)
	entry = append(entry, encodeULEB128(funcIdx)...)
	b.exports = append(b.exports, entry)
	return b
}

// setStart sets the start function index.
func (b *testModuleBuilder) setStart(funcIdx uint32) *testModuleBuilder {
	b.start = &funcIdx
	return b
}

// addElementSegment adds an element segment with function indices.
func (b *testModuleBuilder) addElementSegment(funcIdxs []uint32) *testModuleBuilder {
	var entry []byte
	// table index 0
	entry = append(entry, encodeULEB128(0)...)
	// offset expr: i32.const 0, end
	entry = append(entry, 0x41, 0x00, 0x0b)
	// func indices
	entry = append(entry, encodeULEB128(uint32(len(funcIdxs)))...)
	for _, idx := range funcIdxs {
		entry = append(entry, encodeULEB128(idx)...)
	}
	b.elements = append(b.elements, entry)
	return b
}

// addCustomSection adds a custom section with the given name and payload.
func (b *testModuleBuilder) addCustomSection(name string, payload []byte) *testModuleBuilder {
	var sec []byte
	sec = append(sec, encodeULEB128(uint32(len(name)))...)
	sec = append(sec, []byte(name)...)
	sec = append(sec, payload...)
	b.custom = append(b.custom, sec)
	return b
}

// addTable adds a table section.
func (b *testModuleBuilder) addTable() *testModuleBuilder {
	// 1 table, funcref (0x70), limits: min=0, no max
	b.tables = []byte{0x01, 0x70, 0x00, 0x00}
	return b
}

// addMemory adds a memory section.
func (b *testModuleBuilder) addMemory() *testModuleBuilder {
	// 1 memory, limits: min=1, no max
	b.memories = []byte{0x01, 0x00, 0x01}
	return b
}

// build constructs the final WASM binary.
func (b *testModuleBuilder) build() []byte {
	out := make([]byte, 0, 256)
	// Header
	out = append(out, wasmMagic...)
	out = append(out, 0x01, 0x00, 0x00, 0x00)

	emitSection := func(id byte, payload []byte) {
		out = append(out, id)
		out = append(out, encodeULEB128(uint32(len(payload)))...)
		out = append(out, payload...)
	}

	// Type section
	if len(b.types) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(b.types)))...)
		for _, t := range b.types {
			payload = append(payload, t...)
		}
		emitSection(sectionType, payload)
	}

	// Import section
	if len(b.imports) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(b.imports)))...)
		for _, imp := range b.imports {
			payload = append(payload, imp...)
		}
		emitSection(sectionImport, payload)
	}

	// Function section
	if len(b.funcIdxs) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(b.funcIdxs)))...)
		for _, idx := range b.funcIdxs {
			payload = append(payload, encodeULEB128(idx)...)
		}
		emitSection(sectionFunction, payload)
	}

	// Table section
	if b.tables != nil {
		emitSection(sectionTable, b.tables)
	}

	// Memory section
	if b.memories != nil {
		emitSection(sectionMemory, b.memories)
	}

	// Export section
	if len(b.exports) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(b.exports)))...)
		for _, exp := range b.exports {
			payload = append(payload, exp...)
		}
		emitSection(sectionExport, payload)
	}

	// Start section
	if b.start != nil {
		emitSection(sectionStart, encodeULEB128(*b.start))
	}

	// Element section
	if len(b.elements) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(b.elements)))...)
		for _, elem := range b.elements {
			payload = append(payload, elem...)
		}
		emitSection(sectionElement, payload)
	}

	// Code section
	if len(b.bodies) > 0 {
		var payload []byte
		payload = append(payload, encodeULEB128(uint32(len(b.bodies)))...)
		for _, body := range b.bodies {
			payload = append(payload, encodeULEB128(uint32(len(body)))...)
			payload = append(payload, body...)
		}
		emitSection(sectionCode, payload)
	}

	// Custom sections
	for _, sec := range b.custom {
		emitSection(sectionCustom, sec)
	}

	return out
}

// =============================================================================
// Tests
// =============================================================================

func TestEliminate_NoDeadCode(t *testing.T) {
	// Two functions, both exported. No dead code.
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x01}).       // func 0: nop
		addFunction(0, []byte{0x10, 0x00}). // func 1: call func 0
		addExport("f0", 0).
		addExport("f1", 1).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.RemovedFunctions)
	assert.Equal(t, 2, stats.TotalFunctions)
	assert.Equal(t, wasm, out, "output should be identical when no dead code")
}

func TestEliminate_SingleDeadFunction(t *testing.T) {
	// Three functions: 0 (exported), 1 (called by 0), 2 (dead).
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x10, 0x01}). // func 0: call func 1
		addFunction(0, []byte{0x01}).        // func 1: nop
		addFunction(0, []byte{0x01}).        // func 2: nop (dead)
		addExport("main", 0).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.RemovedFunctions)
	assert.Equal(t, 3, stats.TotalFunctions)
	assert.Less(t, stats.OptimizedSize, stats.OriginalSize)

	// Verify output is valid by re-parsing.
	_, _, err = Eliminate(out)
	require.NoError(t, err)
}

func TestEliminate_ChainedCalls(t *testing.T) {
	// A -> B -> C chain. Only A is exported. B and C should survive.
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x10, 0x01}). // func 0: call func 1
		addFunction(0, []byte{0x10, 0x02}). // func 1: call func 2
		addFunction(0, []byte{0x01}).        // func 2: nop
		addFunction(0, []byte{0x01}).        // func 3: nop (dead)
		addExport("main", 0).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.RemovedFunctions)
	assert.Equal(t, 4, stats.TotalFunctions)

	// Verify the output parses and is idempotent.
	out2, stats2, err := Eliminate(out)
	require.NoError(t, err)
	assert.Equal(t, 0, stats2.RemovedFunctions)
	assert.Equal(t, out, out2)
}

func TestEliminate_AllDead(t *testing.T) {
	// No exports, no start, no elements. All locals should be removed.
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x01}). // func 0: nop (dead)
		addFunction(0, []byte{0x01}). // func 1: nop (dead)
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.RemovedFunctions)
	assert.Equal(t, 2, stats.TotalFunctions)
	assert.Less(t, stats.OptimizedSize, stats.OriginalSize)

	// Output should still be valid WASM.
	mod, err := parseModule(out)
	require.NoError(t, err)
	assert.Empty(t, mod.Bodies)
	assert.Empty(t, mod.FuncTypes)
}

func TestEliminate_IndirectCallPreservation(t *testing.T) {
	// Func 1 is referenced in element segment but not directly called or exported.
	wasm := newTestModule().
		addFuncType().
		addTable().
		addFunction(0, []byte{0x01}). // func 0: nop (dead - no export)
		addFunction(0, []byte{0x01}). // func 1: nop (kept via element)
		addFunction(0, []byte{0x01}). // func 2: nop (dead)
		addElementSegment([]uint32{1}).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.RemovedFunctions, "funcs 0 and 2 should be removed")

	// Verify func 1 survived.
	mod, err := parseModule(out)
	require.NoError(t, err)
	assert.Equal(t, 1, len(mod.Bodies))
}

func TestEliminate_StartFunctionPreservation(t *testing.T) {
	// Start function should be preserved even without exports.
	startIdx := uint32(1)
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x01}). // func 0: nop (dead)
		addFunction(0, []byte{0x01}). // func 1: nop (start)
		setStart(startIdx).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.RemovedFunctions)

	mod, err := parseModule(out)
	require.NoError(t, err)
	require.NotNil(t, mod.StartFunc)
	assert.Equal(t, uint32(0), *mod.StartFunc, "start should be reindexed to 0")
}

func TestEliminate_ImportedFunctionCalls(t *testing.T) {
	// Import func 0, local func 1 calls it, local func 2 is dead.
	wasm := newTestModule().
		addFuncType().
		addFuncImport("env", "log", 0).
		addFunction(0, []byte{0x10, 0x00}). // func 1 (local 0): call import func 0
		addFunction(0, []byte{0x01}).        // func 2 (local 1): nop (dead)
		addExport("main", 1).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.RemovedFunctions)
	assert.Equal(t, 3, stats.TotalFunctions) // 1 import + 2 local

	// Verify the import is still there.
	mod, err := parseModule(out)
	require.NoError(t, err)
	assert.Equal(t, 1, len(mod.Imports))
	assert.Equal(t, 1, len(mod.Bodies))
}

func TestEliminate_IndexRewriting(t *testing.T) {
	// func 0 (exported, calls func 2), func 1 (dead), func 2 (kept).
	// After removal of func 1, func 2 becomes func 1.
	// The call in func 0 should be rewritten from 2 to 1.
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x10, 0x02}). // func 0: call func 2
		addFunction(0, []byte{0x01}).        // func 1: nop (dead)
		addFunction(0, []byte{0x01}).        // func 2: nop (kept)
		addExport("main", 0).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.RemovedFunctions)

	// Parse the output and check that func 0's body calls func 1 (not 2).
	mod, err := parseModule(out)
	require.NoError(t, err)
	require.Equal(t, 2, len(mod.Bodies))

	targets := scanCalls(mod.Bodies[0].Raw)
	assert.True(t, targets[1], "func 0 should now call func 1")
	assert.False(t, targets[2], "func 0 should not reference old index 2")
}

func TestEliminate_CustomSectionsPreserved(t *testing.T) {
	customPayload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x01}).
		addExport("main", 0).
		addCustomSection("mydata", customPayload).
		build()

	out, _, err := Eliminate(wasm)
	require.NoError(t, err)

	// Parse output and check custom section is preserved.
	mod, err := parseModule(out)
	require.NoError(t, err)

	found := false
	for _, s := range mod.PassthroughSections {
		if s.ID == sectionCustom {
			found = true
			// Payload should contain our custom data.
			assert.Contains(t, string(s.Payload), string(customPayload))
		}
	}
	assert.True(t, found, "custom section should be preserved")
}

func TestEliminate_ExportReindexing(t *testing.T) {
	// func 0 (dead), func 1 (exported as "main"), func 2 (exported as "helper").
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x01}).        // func 0: nop (dead)
		addFunction(0, []byte{0x10, 0x02}). // func 1: call func 2
		addFunction(0, []byte{0x01}).        // func 2: nop
		addExport("main", 1).
		addExport("helper", 2).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.RemovedFunctions)

	mod, err := parseModule(out)
	require.NoError(t, err)

	// Exports should be reindexed: main=0, helper=1.
	for _, exp := range mod.Exports {
		switch exp.Name {
		case "main":
			assert.Equal(t, uint32(0), exp.Index)
		case "helper":
			assert.Equal(t, uint32(1), exp.Index)
		}
	}
}

func TestEliminate_InvalidWasm(t *testing.T) {
	_, _, err := Eliminate([]byte{0xFF, 0xFF})
	require.Error(t, err)
	assert.True(t, errors.Is(err, errors.ErrWasmInvalid))
}

func TestEliminate_EmptyModule(t *testing.T) {
	// Valid WASM with no functions at all.
	wasm := newTestModule().build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalFunctions)
	assert.Equal(t, 0, stats.RemovedFunctions)
	assert.Equal(t, wasm, out)
}

func TestEliminate_Idempotent(t *testing.T) {
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x10, 0x01}). // func 0: call func 1
		addFunction(0, []byte{0x01}).        // func 1: nop
		addFunction(0, []byte{0x01}).        // func 2: nop (dead)
		addExport("main", 0).
		build()

	out1, stats1, err := Eliminate(wasm)
	require.NoError(t, err)
	assert.Equal(t, 1, stats1.RemovedFunctions)

	out2, stats2, err := Eliminate(out1)
	require.NoError(t, err)
	assert.Equal(t, 0, stats2.RemovedFunctions)
	assert.Equal(t, out1, out2, "second pass should produce identical output")
}

func TestEliminate_Stats(t *testing.T) {
	wasm := newTestModule().
		addFuncType().
		addFuncImport("env", "log", 0).
		addFunction(0, []byte{0x01}). // func 1 (local 0): nop (kept)
		addFunction(0, []byte{0x01}). // func 2 (local 1): nop (dead)
		addFunction(0, []byte{0x01}). // func 3 (local 2): nop (dead)
		addExport("main", 1).
		build()

	out, stats, err := Eliminate(wasm)
	require.NoError(t, err)

	assert.Equal(t, 4, stats.TotalFunctions)   // 1 import + 3 local
	assert.Equal(t, 2, stats.RemovedFunctions)  // 2 dead locals
	assert.Equal(t, len(wasm), stats.OriginalSize)
	assert.Equal(t, len(out), stats.OptimizedSize)
	assert.Less(t, stats.OptimizedSize, stats.OriginalSize)
}

// =============================================================================
// Internal function tests
// =============================================================================

func TestScanCalls_MultipleCalls(t *testing.T) {
	// Body with calls to func 3 and func 7.
	body := []byte{
		0x00,       // 0 local declarations
		0x10, 0x03, // call 3
		0x10, 0x07, // call 7
		0x0b,       // end
	}
	targets := scanCalls(body)
	assert.True(t, targets[3])
	assert.True(t, targets[7])
	assert.Equal(t, 2, len(targets))
}

func TestScanCalls_NoCalls(t *testing.T) {
	body := []byte{
		0x00, // 0 local declarations
		0x01, // nop
		0x0b, // end
	}
	targets := scanCalls(body)
	assert.Empty(t, targets)
}

func TestRoundTrip_ParseAssemble(t *testing.T) {
	// Build a module, parse it, reassemble, and verify identical output.
	wasm := newTestModule().
		addFuncType().
		addFunction(0, []byte{0x01}).
		addExport("main", 0).
		build()

	mod, err := parseModule(wasm)
	require.NoError(t, err)

	out, err := assemble(mod)
	require.NoError(t, err)
	assert.Equal(t, wasm, out, "round-trip should produce identical binary")
}

func TestRoundTrip_WithImports(t *testing.T) {
	wasm := newTestModule().
		addFuncType().
		addFuncImport("env", "abort", 0).
		addFunction(0, []byte{0x10, 0x00}). // call imported func
		addExport("main", 1).
		build()

	mod, err := parseModule(wasm)
	require.NoError(t, err)

	out, err := assemble(mod)
	require.NoError(t, err)
	assert.Equal(t, wasm, out, "round-trip with imports should be identical")
}
