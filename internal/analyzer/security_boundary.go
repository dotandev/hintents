// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"encoding/json"
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
		}
	}

	return violations, nil
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
