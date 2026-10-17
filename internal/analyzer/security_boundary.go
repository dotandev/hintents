// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Event struct {
	Type      string `json:"type"`
	Contract  string `json:"contract,omitempty"`
	Address   string `json:"address,omitempty"`
	EventType string `json:"event_type,omitempty"`
}

type Violation struct {
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Contract    string                 `json:"contract"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

type SecurityBoundaryChecker struct{}

type contractInvocationState struct {
	HasAuth     bool
	AuthChecked map[string]bool
}

func NewSecurityBoundaryChecker() *SecurityBoundaryChecker {
	return &SecurityBoundaryChecker{}
}

func (c *SecurityBoundaryChecker) Analyze(events []string) ([]Violation, error) {
	var violations []Violation

	contractStates := make(map[string]*contractInvocationState)
	var callStack []string

	for _, eventStr := range events {
		var event Event
		if err := json.Unmarshal([]byte(eventStr), &event); err != nil {
			continue
		}

		if event.Contract == "" || event.Contract == "unknown" {
			continue
		}

		if _, exists := contractStates[event.Contract]; !exists {
			contractStates[event.Contract] = &contractInvocationState{
				AuthChecked: make(map[string]bool),
			}
		}

		state := contractStates[event.Contract]

		switch event.Type {
		case "contract_call", "call", "call_start", "invocation_start":
			callStack = append(callStack, event.Contract)

		case "call_end", "call_return", "invocation_end":
			if len(callStack) > 0 {
				if callStack[len(callStack)-1] == event.Contract {
					callStack = callStack[:len(callStack)-1]
				} else {
					for i := len(callStack) - 1; i >= 0; i-- {
						if callStack[i] == event.Contract {
							callStack = append(callStack[:i], callStack[i+1:]...)
							break
						}
					}
				}
			}

		case "auth":
			state.AuthChecked[event.Address] = true
			state.HasAuth = true

		case "storage_write":
			if !state.HasAuth {
				if !isSACPattern(event) {
					violations = append(violations, Violation{
						Type:        "unauthorized_state_modification",
						Severity:    "high",
						Description: "Storage write operation without prior require_auth check",
						Contract:    event.Contract,
						Details: map[string]interface{}{
							"operation": "storage_write",
						},
					})
				}
			}

			// Cross-contract reentrancy check:
			// If the contract appears more than once on the active call stack and writes storage:
			count := 0
			for _, cInStack := range callStack {
				if cInStack == event.Contract {
					count++
				}
			}
			if count > 1 {
				violations = append(violations, Violation{
					Type:        "cross_contract_reentrancy",
					Severity:    "critical",
					Description: fmt.Sprintf("Reentrant call to contract '%s' modifies state via storage write", event.Contract),
					Contract:    event.Contract,
					Details: map[string]interface{}{
						"operation":  "storage_write",
						"call_depth": len(callStack),
						"call_stack": append([]string(nil), callStack...),
					},
				})
			}
		}
	}

	return violations, nil
}

// InvocationNode represents a node in the contract invocation tree.
type InvocationNode struct {
	ContractID string           `json:"contract_id"`
	Function   string           `json:"function,omitempty"`
	Calls      []InvocationNode `json:"calls,omitempty"`
	HasWrite   bool             `json:"has_write,omitempty"`
}

// CheckInvocationTreeReentrancy walks the invocation tree and flags any contract that
// appears more than once on the active call stack and writes storage on the reentrant path.
func CheckInvocationTreeReentrancy(root InvocationNode) []Violation {
	var violations []Violation
	var walk func(node InvocationNode, stack []string)

	walk = func(node InvocationNode, stack []string) {
		currentStack := append(stack, node.ContractID)

		count := 0
		for _, c := range currentStack {
			if c == node.ContractID {
				count++
			}
		}

		if count > 1 && node.HasWrite {
			violations = append(violations, Violation{
				Type:        "cross_contract_reentrancy",
				Severity:    "critical",
				Description: fmt.Sprintf("Reentrant call to contract '%s' modifies state via storage write", node.ContractID),
				Contract:    node.ContractID,
				Details: map[string]interface{}{
					"operation":  "storage_write",
					"call_depth": len(currentStack),
					"call_stack": append([]string(nil), currentStack...),
				},
			})
		}

		for _, child := range node.Calls {
			walk(child, currentStack)
		}
	}

	walk(root, nil)
	return violations
}

func isSACPattern(event Event) bool {
	contract := event.Contract

	if contract == "" || contract == "unknown" {
		return false
	}

	sacPatterns := []string{
		"stellar_asset",
		"SAC",
		"token",
	}

	contractLower := strings.ToLower(contract)
	for _, pattern := range sacPatterns {
		if strings.Contains(contractLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
