// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// CallNode represents a node in the execution call tree
type CallNode struct {
	ContractID      string         `json:"contract_id"`
	Function        string         `json:"function,omitempty"`
	Events          []DecodedEvent `json:"events,omitempty"`
	SubCalls        []*CallNode    `json:"sub_calls,omitempty"`
	CPUInstructions uint64         `json:"cpu,omitempty"`
	MemoryBytes     uint64         `json:"mem,omitempty"`

	// Internal for tree building
	parent *CallNode
}

// DecodedEvent is a human-friendly representation of a DiagnosticEvent
type DecodedEvent struct {
	ContractID string   `json:"contract_id"`
	Topics     []string `json:"topics"`
	Data       string   `json:"data"`
	CPU        uint64   `json:"cpu,omitempty"`
	Memory     uint64   `json:"mem,omitempty"`
}

// DecodeEvents builds a call hierarchy from a list of base64-encoded XDR DiagnosticEvents.
// Deprecated: use DecodeDiagnosticEvents instead for gas-aware traces.
func DecodeEvents(eventsXdr []string, maxDepth int) (*CallNode, error) {
	var diagEvents []simulator.DiagnosticEvent
	for _, eventStr := range eventsXdr {
		var diag xdr.DiagnosticEvent
		data, err := base64.StdEncoding.DecodeString(eventStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 event: %w", err)
		}
		if err := xdr.SafeUnmarshal(data, &diag); err != nil {
			return nil, fmt.Errorf("failed to unmarshal XDR event: %w", err)
		}

		decoded := simulator.DiagnosticEvent{
			EventType:                fmt.Sprintf("%v", diag.Event.Type),
			Topics:                   make([]string, 0, len(diag.Event.Body.V0.Topics)),
			Data:                     fmt.Sprintf("%v", diag.Event.Body.V0.Data.Type),
			InSuccessfulContractCall: diag.InSuccessfulContractCall,
		}
		if diag.Event.ContractId != nil {
			id := hex.EncodeToString(diag.Event.ContractId[:])
			decoded.ContractID = &id
		}
		for _, topic := range diag.Event.Body.V0.Topics {
			if topic.Type == xdr.ScValTypeScvSymbol {
				decoded.Topics = append(decoded.Topics, string(*topic.Sym))
			} else {
				decoded.Topics = append(decoded.Topics, fmt.Sprintf("%v", topic.Type))
			}
		}
		diagEvents = append(diagEvents, decoded)
	}

	return DecodeDiagnosticEvents(diagEvents, maxDepth)
}

// DecodeDiagnosticEvents builds a call hierarchy from a list of decoded simulator DiagnosticEvents
func DecodeDiagnosticEvents(events []simulator.DiagnosticEvent, maxDepth int) (*CallNode, error) {
	root := &CallNode{
		ContractID: "ROOT",
		Function:   "TOP_LEVEL",
	}
	current := root
	currentDepth := 0

	for _, event := range events {
		decoded := DecodedEvent{
			Topics: event.Topics,
			Data:   event.Data,
		}
		if event.ContractID != nil {
			decoded.ContractID = *event.ContractID
		}
		if event.CPU != nil {
			decoded.CPU = *event.CPU
		}
		if event.Memory != nil {
			decoded.Memory = *event.Memory
		}

		if isFunctionCall(decoded) {
			if maxDepth > 0 && currentDepth >= maxDepth {
				// Depth limit reached. Truncate this branch.
				// Add a warning event to the current node if not already present
				hasWarning := false
				for _, e := range current.Events {
					if e.Topics[0] == "warning" && e.Data == "Max trace depth reached; branch truncated" {
						hasWarning = true
						break
					}
				}
				if !hasWarning {
					current.Events = append(current.Events, DecodedEvent{
						ContractID: "SYSTEM",
						Topics:     []string{"warning"},
						Data:       "Max trace depth reached; branch truncated",
					})
				}
				continue
			}

			child := &CallNode{
				ContractID: decoded.ContractID,
				Function:   extractFunctionName(decoded),
				parent:     current,
			}
			if event.CPU != nil {
				child.CPUInstructions = *event.CPU
			}
			if event.Memory != nil {
				child.MemoryBytes = *event.Memory
			}
			current.SubCalls = append(current.SubCalls, child)
			current = child
			currentDepth++
			current.Events = append(current.Events, decoded)
		} else if isFunctionReturn(decoded) {
			if maxDepth > 0 && currentDepth >= maxDepth {
				// We skipped the corresponding call, so we must skip the return.
				continue
			}

			returnedFn := extractFunctionName(decoded)
			if current.Function != returnedFn && current.Function != "TOP_LEVEL" {
				iter := current.parent
				found := false
				tempDepth := currentDepth - 1
				for iter != nil {
					if iter.Function == returnedFn {
						found = true
						break
					}
					iter = iter.parent
					tempDepth--
				}
				if found {
					for current != iter {
						current = current.parent
						currentDepth--
					}
				}
			}

			// Calculate gas used by this node if both call and return have budget
			if event.CPU != nil && current.CPUInstructions > 0 {
				current.CPUInstructions = *event.CPU - current.CPUInstructions
			}
			if event.Memory != nil && current.MemoryBytes > 0 {
				current.MemoryBytes = *event.Memory - current.MemoryBytes
			}

			current.Events = append(current.Events, decoded)
			if current.parent != nil {
				current = current.parent
				currentDepth--
			}
		} else {
			current.Events = append(current.Events, decoded)
		}
	}

	return root, nil
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
