// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"
)

func TestClassifyEventType_Explicit(t *testing.T) {
	tests := []struct {
		eventType string
		want      string
	}{
		{EventTypeTrap, EventTypeTrap},
		{EventTypeContractCall, EventTypeContractCall},
		{EventTypeHostFunction, EventTypeHostFunction},
		{EventTypeAuth, EventTypeAuth},
		{"", EventTypeOther},
	}
	for _, tt := range tests {
		state := &ExecutionState{EventType: tt.eventType, Operation: "custom"}
		got := ClassifyEventType(state)
		if tt.eventType == "" {
			if got != EventTypeOther {
				t.Errorf("ClassifyEventType(EventType=%q) = %q, want %q", tt.eventType, got, EventTypeOther)
			}
			continue
		}
		if got != tt.want {
			t.Errorf("ClassifyEventType(EventType=%q) = %q, want %q", tt.eventType, got, tt.want)
		}
	}
}

func TestClassifyEventType_Trap(t *testing.T) {
	state := &ExecutionState{Operation: "run", Error: "Wasm Trap: out of bounds"}
	if got := ClassifyEventType(state); got != EventTypeTrap {
		t.Errorf("ClassifyEventType(trap error) = %q, want %q", got, EventTypeTrap)
	}
	state = &ExecutionState{Operation: "trap", Error: "panic"}
	if got := ClassifyEventType(state); got != EventTypeTrap {
		t.Errorf("ClassifyEventType(trap op) = %q, want %q", got, EventTypeTrap)
	}
}

func TestClassifyEventType_ContractCall(t *testing.T) {
	state := &ExecutionState{Operation: "contract_call", ContractID: "C123", Function: "transfer"}
	if got := ClassifyEventType(state); got != EventTypeContractCall {
		t.Errorf("ClassifyEventType(contract_call) = %q, want %q", got, EventTypeContractCall)
	}
	state = &ExecutionState{Operation: "contract_init", ContractID: "C456"}
	if got := ClassifyEventType(state); got != EventTypeContractCall {
		t.Errorf("ClassifyEventType(contract_init) = %q, want %q", got, EventTypeContractCall)
	}
}

func TestClassifyEventType_HostFunction(t *testing.T) {
	state := &ExecutionState{Operation: "host_fn", Function: "get_ledger_entry"}
	if got := ClassifyEventType(state); got != EventTypeHostFunction {
		t.Errorf("ClassifyEventType(host_fn) = %q, want %q", got, EventTypeHostFunction)
	}
	state = &ExecutionState{Operation: "invoke", Function: "put_ledger_entry"}
	if got := ClassifyEventType(state); got != EventTypeHostFunction {
		t.Errorf("ClassifyEventType(host function) = %q, want %q", got, EventTypeHostFunction)
	}
}

func TestClassifyEventType_Auth(t *testing.T) {
	state := &ExecutionState{Operation: "call", Function: "require_auth"}
	if got := ClassifyEventType(state); got != EventTypeAuth {
		t.Errorf("ClassifyEventType(require_auth) = %q, want %q", got, EventTypeAuth)
	}
	state = &ExecutionState{Operation: "auth", Function: "check"}
	if got := ClassifyEventType(state); got != EventTypeAuth {
		t.Errorf("ClassifyEventType(auth op) = %q, want %q", got, EventTypeAuth)
	}
}

func TestAllFilterableEventTypes(t *testing.T) {
	types := AllFilterableEventTypes()
	want := []string{EventTypeTrap, EventTypeContractCall, EventTypeHostFunction, EventTypeAuth}
	if len(types) != len(want) {
		t.Fatalf("AllFilterableEventTypes() length = %d, want %d", len(types), len(want))
	}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("AllFilterableEventTypes()[%d] = %q, want %q", i, types[i], want[i])
		}
	}
}
