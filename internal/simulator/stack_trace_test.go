// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/json"
	"testing"
)

func TestWasmStackTraceDeserialization(t *testing.T) {
	jsonData := `{
		"trap_kind": "OutOfBoundsMemoryAccess",
		"raw_message": "wasm trap: out of bounds memory access\n  0: func[42] @ 0xa3c\n  1: func[7] @ 0xb20",
		"frames": [
			{
				"index": 0,
				"func_index": 42,
				"wasm_offset": 2620
			},
			{
				"index": 1,
				"func_index": 7,
				"func_name": "soroban_token::transfer",
				"wasm_offset": 2848,
				"module": "token"
			}
		],
		"soroban_wrapped": false
	}`

	var trace WasmStackTrace
	if err := json.Unmarshal([]byte(jsonData), &trace); err != nil {
		t.Fatalf("failed to unmarshal WasmStackTrace: %v", err)
	}

	if trace.RawMessage == "" {
		t.Error("expected non-empty RawMessage")
	}

	if len(trace.Frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(trace.Frames))
	}

	// Frame 0: func index only
	frame0 := trace.Frames[0]
	if frame0.Index != 0 {
		t.Errorf("frame 0 index = %d, want 0", frame0.Index)
	}
	if frame0.FuncIndex == nil || *frame0.FuncIndex != 42 {
		t.Errorf("frame 0 func_index = %v, want 42", frame0.FuncIndex)
	}
	if frame0.FuncName != nil {
		t.Errorf("frame 0 func_name should be nil, got %q", *frame0.FuncName)
	}

	// Frame 1: function name, index, offset, and module
	frame1 := trace.Frames[1]
	if frame1.Index != 1 {
		t.Errorf("frame 1 index = %d, want 1", frame1.Index)
	}
	if frame1.FuncName == nil || *frame1.FuncName != "soroban_token::transfer" {
		t.Errorf("frame 1 func_name = %v, want %q", frame1.FuncName, "soroban_token::transfer")
	}
	if frame1.Module == nil || *frame1.Module != "token" {
		t.Errorf("frame 1 module = %v, want %q", frame1.Module, "token")
	}
	if frame1.WasmOffset == nil || *frame1.WasmOffset != 2848 {
		t.Errorf("frame 1 wasm_offset = %v, want 2848", frame1.WasmOffset)
	}

	if trace.SorobanWrapped {
		t.Error("expected SorobanWrapped to be false")
	}
}

func TestWasmStackTraceSorobanWrapped(t *testing.T) {
	jsonData := `{
		"trap_kind": {"HostError": "ScError(WasmVm, MissingValue)"},
		"raw_message": "HostError: Error(WasmVm, MissingValue)",
		"frames": [
			{
				"index": 0,
				"func_index": 5,
				"wasm_offset": 66
			}
		],
		"soroban_wrapped": true
	}`

	var trace WasmStackTrace
	if err := json.Unmarshal([]byte(jsonData), &trace); err != nil {
		t.Fatalf("failed to unmarshal WasmStackTrace: %v", err)
	}

	if !trace.SorobanWrapped {
		t.Error("expected SorobanWrapped to be true")
	}

	if len(trace.Frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(trace.Frames))
	}
}

func TestWasmStackTraceEmptyFrames(t *testing.T) {
	jsonData := `{
		"trap_kind": {"Unknown": "assertion failed"},
		"raw_message": "assertion failed",
		"frames": [],
		"soroban_wrapped": false
	}`

	var trace WasmStackTrace
	if err := json.Unmarshal([]byte(jsonData), &trace); err != nil {
		t.Fatalf("failed to unmarshal WasmStackTrace: %v", err)
	}

	if len(trace.Frames) != 0 {
		t.Errorf("expected 0 frames, got %d", len(trace.Frames))
	}
}

func TestStackTraceInSimulationResponse(t *testing.T) {
	funcIdx := uint32(42)
	offset := uint64(0xa3c)
	funcName := "my_contract::transfer"

	resp := SimulationResponse{
		Status: "error",
		Error:  "contract trap",
		StackTrace: &WasmStackTrace{
			TrapKind:   "OutOfBoundsMemoryAccess",
			RawMessage: "wasm trap: out of bounds memory access",
			Frames: []StackFrame{
				{
					Index:      0,
					FuncIndex:  &funcIdx,
					WasmOffset: &offset,
				},
				{
					Index:    1,
					FuncName: &funcName,
				},
			},
			SorobanWrapped: false,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// Verify it can round-trip
	var decoded SimulationResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.StackTrace == nil {
		t.Fatal("expected non-nil StackTrace after round-trip")
	}
	if len(decoded.StackTrace.Frames) != 2 {
		t.Errorf("expected 2 frames, got %d", len(decoded.StackTrace.Frames))
	}
	if decoded.StackTrace.Frames[0].FuncIndex == nil || *decoded.StackTrace.Frames[0].FuncIndex != 42 {
		t.Error("frame 0 func_index mismatch after round-trip")
	}
}

func TestSimulationResponseWithoutStackTrace(t *testing.T) {
	resp := SimulationResponse{
		Status: "success",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	jsonStr := string(data)
	if containsField(jsonStr, "stack_trace") {
		t.Error("stack_trace should be omitted from JSON when nil")
	}
}

func containsField(jsonStr, field string) bool {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return false
	}
	_, ok := m[field]
	return ok
}

func TestStackFrameMinimalFields(t *testing.T) {
	frame := StackFrame{
		Index: 0,
	}

	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("failed to marshal minimal frame: %v", err)
	}

	var decoded StackFrame
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal minimal frame: %v", err)
	}

	if decoded.Index != 0 {
		t.Errorf("expected index 0, got %d", decoded.Index)
	}
	if decoded.FuncIndex != nil {
		t.Error("expected nil FuncIndex")
	}
	if decoded.FuncName != nil {
		t.Error("expected nil FuncName")
	}
	if decoded.WasmOffset != nil {
		t.Error("expected nil WasmOffset")
	}
	if decoded.Module != nil {
		t.Error("expected nil Module")
	}
}
