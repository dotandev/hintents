// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

type WasmStackTrace struct {
	TrapKind       interface{}  `json:"trap_kind"`
	RawMessage     string       `json:"raw_message"`
	Frames         []StackFrame `json:"frames"`
	SorobanWrapped bool         `json:"soroban_wrapped"`
}

type StackFrame struct {
	Index          int             `json:"index"`
	FuncIndex      *uint32         `json:"func_index,omitempty"`
	FuncName       *string         `json:"func_name,omitempty"`
	WasmOffset     *uint64         `json:"wasm_offset,omitempty"`
	Module         *string         `json:"module,omitempty"`
	SourceLocation *SourceLocation `json:"source_location,omitempty"`
}

type SourceLocation struct {
	File      string `json:"file"`
	Line      uint   `json:"line"`
	Column    uint   `json:"column"`
	ColumnEnd *uint  `json:"column_end,omitempty"`
}
