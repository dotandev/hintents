// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

type DiagnosticEvent struct {
	EventType                string   `json:"event_type"`
	ContractID               *string  `json:"contract_id,omitempty"`
	Topics                   []string `json:"topics"`
	Data                     string   `json:"data"`
	InSuccessfulContractCall bool     `json:"in_successful_contract_call"`
	WasmInstruction          *string  `json:"wasm_instruction,omitempty"`
	CPU                      *uint64  `json:"cpu,omitempty"`
	Memory                   *uint64  `json:"mem,omitempty"`
}

// ParseData decodes the base64-encoded XDR Data into an xdr.ScVal
func (e *DiagnosticEvent) ParseData() (xdr.ScVal, error) {
	var val xdr.ScVal
	if e.Data == "" {
		return val, nil
	}
	raw, err := base64.StdEncoding.DecodeString(e.Data)
	if err != nil {
		return val, fmt.Errorf("decode data base64: %w", err)
	}
	if err := xdr.SafeUnmarshal(raw, &val); err != nil {
		return val, fmt.Errorf("unmarshal data xdr: %w", err)
	}
	return val, nil
}

// ParseTopics decodes the base64-encoded XDR Topics into a slice of xdr.ScVal
func (e *DiagnosticEvent) ParseTopics() ([]xdr.ScVal, error) {
	var vals []xdr.ScVal
	for i, t := range e.Topics {
		var val xdr.ScVal
		raw, err := base64.StdEncoding.DecodeString(t)
		if err != nil {
			return nil, fmt.Errorf("decode topic[%d] base64: %w", i, err)
		}
		if err := xdr.SafeUnmarshal(raw, &val); err != nil {
			return nil, fmt.Errorf("unmarshal topic[%d] xdr: %w", i, err)
		}
		vals = append(vals, val)
	}
	return vals, nil
}

// CategorizedEvent mirrors the simulator IPC definition (simulator/src/types.rs):
// a category label plus the nested DiagnosticEvent payload.
type CategorizedEvent struct {
	Category string          `json:"category"`
	Event    DiagnosticEvent `json:"event"`
}

// EventType returns the nested event's type (contract/system/diagnostic).
func (e *CategorizedEvent) EventType() string { return e.Event.EventType }

// ContractID returns the nested event's contract ID pointer.
func (e *CategorizedEvent) ContractID() *string { return e.Event.ContractID }

// Topics returns the nested event's topics.
func (e *CategorizedEvent) Topics() []string { return e.Event.Topics }

// Data returns the nested event's base64 XDR data.
func (e *CategorizedEvent) Data() string { return e.Event.Data }

// InSuccessfulContractCall reports whether the nested event succeeded.
func (e *CategorizedEvent) InSuccessfulContractCall() bool { return e.Event.InSuccessfulContractCall }

// WasmInstruction returns the nested event's WASM instruction, if any.
func (e *CategorizedEvent) WasmInstruction() *string { return e.Event.WasmInstruction }

// CPU returns the nested event's CPU instruction count, if recorded.
func (e *CategorizedEvent) CPU() *uint64 { return e.Event.CPU }

// Memory returns the nested event's memory bytes, if recorded.
func (e *CategorizedEvent) Memory() *uint64 { return e.Event.Memory }
