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
	ContractID       string          `json:"contract_id"`
	Topics           []string        `json:"topics"`
	Data             string          `json:"data"`
	CPU              uint64          `json:"cpu,omitempty"`
	Memory           uint64          `json:"mem,omitempty"`
	Error            *xdr.ScError    `json:"error,omitempty"`
	ContractFunction string          `json:"contract_function,omitempty"`
	ArgumentTypes    []xdr.ScValType `json:"argument_types,omitempty"`

	hasCPU      bool
	hasMemory   bool
	topicSymbol string
}

// DecodeEvents builds a call hierarchy from a list of base64-encoded XDR DiagnosticEvents.
// Deprecated: use DecodeDiagnosticEvents instead for gas-aware traces.
func DecodeEvents(eventsXdr []string, maxDepth int) (*CallNode, error) {
	var diagEvents []DecodedEvent
	for _, eventStr := range eventsXdr {
		var diag xdr.DiagnosticEvent
		data, err := base64.StdEncoding.DecodeString(eventStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 event: %w", err)
		}
		if err := xdr.SafeUnmarshal(data, &diag); err != nil {
			return nil, fmt.Errorf("failed to unmarshal XDR event: %w", err)
		}

		decoded := DecodedEvent{
			Topics: make([]string, 0, len(diag.Event.Body.V0.Topics)),
			Data:   fmt.Sprintf("%v", diag.Event.Body.V0.Data.Type),
		}
		if diag.Event.ContractId != nil {
			id := hex.EncodeToString(diag.Event.ContractId[:])
			decoded.ContractID = id
		}
		for _, topic := range diag.Event.Body.V0.Topics {
			if topic.Type == xdr.ScValTypeScvSymbol {
				decoded.Topics = append(decoded.Topics, string(*topic.Sym))
			} else {
				decoded.Topics = append(decoded.Topics, fmt.Sprintf("%v", topic.Type))
			}
		}
		populateDiagnosticFields(&decoded, diag.Event.Body.V0.Topics, &diag.Event.Body.V0.Data)
		diagEvents = append(diagEvents, decoded)
	}

	return buildCallTree(diagEvents, maxDepth)
}

// DecodeDiagnosticEvents builds a call hierarchy from a list of decoded simulator DiagnosticEvents
func DecodeDiagnosticEvents(events []simulator.DiagnosticEvent, maxDepth int) (*CallNode, error) {
	decodedEvents := make([]DecodedEvent, 0, len(events))
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
			decoded.hasCPU = true
		}
		if event.Memory != nil {
			decoded.Memory = *event.Memory
			decoded.hasMemory = true
		}
		// Legacy callers also pass display text; leave it available as fallback.
		topics, _ := event.ParseTopics()
		var data *xdr.ScVal
		if event.Data != "" {
			if value, err := event.ParseData(); err == nil {
				data = &value
			}
		}
		populateDiagnosticFields(&decoded, topics, data)
		decodedEvents = append(decodedEvents, decoded)
	}
	return buildCallTree(decodedEvents, maxDepth)
}

// populateDiagnosticFields retains typed XDR independently of display text.
func populateDiagnosticFields(event *DecodedEvent, topics []xdr.ScVal, data *xdr.ScVal) {
	if len(topics) > 0 && topics[0].Type == xdr.ScValTypeScvSymbol && topics[0].Sym != nil {
		event.topicSymbol = string(*topics[0].Sym)
	}
	for _, topic := range topics {
		if topic.Type == xdr.ScValTypeScvError && topic.Error != nil {
			event.Error = topic.Error
			break
		}
	}
	functionIndex := 1
	if event.topicSymbol == "fn_call" && len(topics) > 2 && topics[1].Type == xdr.ScValTypeScvBytes {
		// Host calls include the callee's contract ID before the function name.
		functionIndex = 2
		if topics[1].Bytes != nil {
			event.ContractID = hex.EncodeToString(*topics[1].Bytes)
		}
	}
	if (event.topicSymbol == "fn_call" || event.topicSymbol == "fn_return") && len(topics) > functionIndex && topics[functionIndex].Sym != nil {
		event.ContractFunction = string(*topics[functionIndex].Sym)
	}
	if data == nil {
		return
	}
	values := []xdr.ScVal{*data}
	if data.Type == xdr.ScValTypeScvVec && data.Vec != nil && *data.Vec != nil {
		values = **data.Vec
	}
	for _, value := range values {
		event.ArgumentTypes = append(event.ArgumentTypes, value.Type)
		if event.Error == nil && value.Type == xdr.ScValTypeScvError && value.Error != nil {
			event.Error = value.Error
		}
	}
}

func buildCallTree(events []DecodedEvent, maxDepth int) (*CallNode, error) {
	root := &CallNode{ContractID: "ROOT", Function: "TOP_LEVEL"}
	current := root
	currentDepth := 0
	skippedDepth := 0
	for _, decoded := range events {
		if decoded.ContractFunction == "" && current != root && !isFunctionCall(decoded) && !isFunctionReturn(decoded) {
			decoded.ContractFunction = current.Function
		}

		if isFunctionCall(decoded) {
			if skippedDepth > 0 || (maxDepth > 0 && currentDepth >= maxDepth) {
				skippedDepth++
				// Depth limit reached. Truncate this branch.
				// Add a warning event to the current node if not already present
				hasWarning := false
				for _, e := range current.Events {
					if len(e.Topics) > 0 && e.Topics[0] == "warning" && e.Data == "Max trace depth reached; branch truncated" {
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
			if decoded.hasCPU {
				child.CPUInstructions = decoded.CPU
			}
			if decoded.hasMemory {
				child.MemoryBytes = decoded.Memory
			}
			current.SubCalls = append(current.SubCalls, child)
			current = child
			currentDepth++
			current.Events = append(current.Events, decoded)
		} else if isFunctionReturn(decoded) {
			if skippedDepth > 0 {
				// We skipped the corresponding call, so we must skip the return.
				skippedDepth--
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
			if decoded.hasCPU && current.CPUInstructions > 0 {
				current.CPUInstructions = decoded.CPU - current.CPUInstructions
			}
			if decoded.hasMemory && current.MemoryBytes > 0 {
				current.MemoryBytes = decoded.Memory - current.MemoryBytes
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
	return e.topicSymbol == "fn_call" || (len(e.Topics) > 0 && e.Topics[0] == "fn_call")
}

func isFunctionReturn(e DecodedEvent) bool {
	return e.topicSymbol == "fn_return" || (len(e.Topics) > 0 && e.Topics[0] == "fn_return")
}

func extractFunctionName(e DecodedEvent) string {
	if e.ContractFunction != "" {
		return e.ContractFunction
	}
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
