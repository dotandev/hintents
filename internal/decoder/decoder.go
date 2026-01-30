// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package decoder

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/stellar/go/xdr"
)

// CallNode represents a node in the execution call tree
type CallNode struct {
	ContractID string         `json:"contract_id"`
	Function   string         `json:"function,omitempty"`
	Events     []DecodedEvent `json:"events,omitempty"`
	SubCalls   []*CallNode    `json:"sub_calls,omitempty"`

	// Internal for tree building
	parent *CallNode
}

// DecodedEvent is a human-friendly representation of a DiagnosticEvent
type DecodedEvent struct {
	ContractID string   `json:"contract_id"`
	Topics     []string `json:"topics"`
	Data       string   `json:"data"`
}

// DecodeEvents builds a call hierarchy from a list of base64-encoded XDR DiagnosticEvents
func DecodeEvents(eventsXdr []string) (*CallNode, error) {
	root := &CallNode{
		ContractID: "ROOT",
		Function:   "TOP_LEVEL",
	}
	current := root

	for _, eventStr := range eventsXdr {
		var diag xdr.DiagnosticEvent
		data, err := base64.StdEncoding.DecodeString(eventStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 event: %w", err)
		}
		if err := xdr.SafeUnmarshal(data, &diag); err != nil {
			return nil, fmt.Errorf("failed to unmarshal XDR event: %w", err)
		}

		decoded := parseEvent(diag)

		// Check for call/return markers in topics
		// Convention: System events with topics ["fn_call", func_name, ...]
		// Note: This relies on the environment emitting these diagnostic events.
		if isFunctionCall(decoded) {
			child := &CallNode{
				ContractID: decoded.ContractID,
				Function:   extractFunctionName(decoded),
				parent:     current,
			}
			current.SubCalls = append(current.SubCalls, child)
			current = child

			// Add the call event itself to the child (optional, but good for context)
			current.Events = append(current.Events, decoded)
		} else if isFunctionReturn(decoded) {
			returnedFn := extractFunctionName(decoded)

			// Handle stack unwinding for failed/implicit returns
			// If current function doesn't match the return event, check up the stack
			if current.Function != returnedFn && current.Function != "TOP_LEVEL" {
				// Search for the matching function up the stack
				iter := current.parent
				found := false
				for iter != nil {
					if iter.Function == returnedFn {
						found = true
						break
					}
					iter = iter.parent
				}

				// If found, unwind everything below it (they failed/exited without event)
				if found {
					for current != iter {
						current = current.parent
					}
				}
			}

			// Add return event to current (which should now be the matching node)
			current.Events = append(current.Events, decoded)

			// Pop stack
			if current.parent != nil {
				current = current.parent
			}
		} else {
			// Regular event, add to current scope
			current.Events = append(current.Events, decoded)
		}
	}

	return root, nil
}

func parseEvent(diag xdr.DiagnosticEvent) DecodedEvent {
	var contractID string
	if diag.Event.ContractId != nil {
		contractID = hex.EncodeToString(diag.Event.ContractId[:])
	}

	topics := make([]string, 0)
	for _, topic := range diag.Event.Body.V0.Topics {
		// Attempt to convert to string if symbol, otherwise hex/debug
		if topic.Type == xdr.ScValTypeScvSymbol {
			topics = append(topics, string(*topic.Sym))
		} else {
			// Fallback for other types
			topics = append(topics, fmt.Sprintf("%v", topic.Type))
		}
	}

	// Simple data stringification
	data := fmt.Sprintf("%v", diag.Event.Body.V0.Data.Type)

	return DecodedEvent{
		ContractID: contractID,
		Topics:     topics,
		Data:       data,
	}
}

func isFunctionCall(e DecodedEvent) bool {
	return len(e.Topics) > 0 && e.Topics[0] == "fn_call"
}

func isFunctionReturn(e DecodedEvent) bool {
	return len(e.Topics) > 0 && e.Topics[0] == "fn_return"
}

func extractFunctionName(e DecodedEvent) string {
	if len(e.Topics) > 1 {
		return e.Topics[1]
	}
	return "unknown"
}

// DecodeEnvelope decodes a base64-encoded XDR transaction envelope
func DecodeEnvelope(envelopeXdr string) (*xdr.TransactionEnvelope, error) {
	if envelopeXdr == "" {
		return nil, fmt.Errorf("envelope XDR is empty")
	}

	// Decode base64
	xdrBytes, err := base64.StdEncoding.DecodeString(envelopeXdr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Decode XDR
	var envelope xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshal(xdrBytes, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XDR: %w", err)
	}

	return &envelope, nil
}
