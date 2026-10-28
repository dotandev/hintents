// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
)

const (
	// A single storage entry larger than this (bytes) is flagged.
	storageEntrySizeThreshold = 1024
	// A single entry growing by more than this (bytes) between writes is flagged.
	storageGrowthThreshold = 512
	// This many distinct keys under one topic prefix suggests caller-supplied keys.
	storageKeyFanoutThreshold = 10
)

type SecurityViolation struct {
	Type        string
	Description string
	Severity    string
	Location    string
}

type SecurityAnalyzer struct {
	violations []SecurityViolation
}

func NewSecurityAnalyzer() *SecurityAnalyzer {
	return &SecurityAnalyzer{
		violations: make([]SecurityViolation, 0),
	}
}

func (sa *SecurityAnalyzer) Analyze(resp *simulator.SimulationResponse) []SecurityViolation {
	sa.violations = make([]SecurityViolation, 0)

	if resp.Status != "success" {
		return sa.violations
	}

	sa.checkUnauthorizedStateModifications(resp.CategorizedEvents)
	sa.checkUnboundedStorageGrowth(resp.CategorizedEvents)

	return sa.violations
}

func (sa *SecurityAnalyzer) checkUnauthorizedStateModifications(events []simulator.CategorizedEvent) {
	type StateModification struct {
		index      int
		contractID string
	}

	authChecks := make(map[string][]int)
	stateWrites := make([]StateModification, 0)

	for i, event := range events {
		contractID := ""
		if event.ContractID != nil {
			contractID = *event.ContractID
		}

		switch event.EventType {
		case "require_auth":
			authChecks[contractID] = append(authChecks[contractID], i)
		case "storage_write":
			stateWrites = append(stateWrites, StateModification{
				index:      i,
				contractID: contractID,
			})
		}
	}

	for _, write := range stateWrites {
		if sa.isSACPattern(events, write.index) {
			continue
		}

		if !sa.hasAuthBeforeWrite(authChecks[write.contractID], write.index) {
			sa.violations = append(sa.violations, SecurityViolation{
				Type:        "UnauthorizedStateModification",
				Description: fmt.Sprintf("State modification without require_auth in contract %s", write.contractID),
				Severity:    "high",
				Location:    fmt.Sprintf("event_index:%d", write.index),
			})
		}
	}
}

// checkUnboundedStorageGrowth warns when a single storage entry becomes large or
// grows quickly during simulation, or when one contract writes many distinct keys
// under the same topic prefix (a sign the keys come from caller-supplied values).
func (sa *SecurityAnalyzer) checkUnboundedStorageGrowth(events []simulator.CategorizedEvent) {
	type entryState struct {
		lastSize int
		flagged  bool
	}

	entries := make(map[string]*entryState)
	fanout := make(map[string]map[string]struct{})
	fanoutFlagged := make(map[string]bool)

	for i, event := range events {
		if event.EventType != "storage_write" {
			continue
		}

		contractID := ""
		if event.ContractID != nil {
			contractID = *event.ContractID
		}

		key := strings.Join(event.Topics, "|")
		entryID := contractID + "/" + key
		size := len(event.Data)

		state, seen := entries[entryID]
		if !seen {
			state = &entryState{}
			entries[entryID] = state
		}

		growth := size - state.lastSize
		state.lastSize = size

		if !state.flagged && (size > storageEntrySizeThreshold || (seen && growth > storageGrowthThreshold)) {
			state.flagged = true
			sa.violations = append(sa.violations, SecurityViolation{
				Type:        "UnboundedStorageGrowth",
				Description: fmt.Sprintf("Storage entry in contract %s is large or growing fast (%d bytes)", contractID, size),
				Severity:    "medium",
				Location:    fmt.Sprintf("event_index:%d", i),
			})
		}

		if len(event.Topics) > 1 {
			prefix := contractID + "/" + event.Topics[0]
			if fanout[prefix] == nil {
				fanout[prefix] = make(map[string]struct{})
			}
			fanout[prefix][key] = struct{}{}

			if len(fanout[prefix]) >= storageKeyFanoutThreshold && !fanoutFlagged[prefix] {
				fanoutFlagged[prefix] = true
				sa.violations = append(sa.violations, SecurityViolation{
					Type: "UnboundedStorageGrowth",
					Description: fmt.Sprintf(
						"Contract %s writes %d distinct keys under %q; keys may come from caller-supplied values",
						contractID, len(fanout[prefix]), event.Topics[0]),
					Severity: "medium",
					Location: fmt.Sprintf("event_index:%d", i),
				})
			}
		}
	}
}

func (sa *SecurityAnalyzer) hasAuthBeforeWrite(authIndices []int, writeIndex int) bool {
	for _, authIdx := range authIndices {
		if authIdx < writeIndex {
			return true
		}
	}
	return false
}

func (sa *SecurityAnalyzer) isSACPattern(events []simulator.CategorizedEvent, writeIndex int) bool {
	if writeIndex >= len(events) {
		return false
	}

	event := events[writeIndex]

	for _, topic := range event.Topics {
		topicLower := strings.ToLower(topic)
		if strings.Contains(topicLower, "balance") ||
			strings.Contains(topicLower, "allowance") ||
			strings.Contains(topicLower, "admin") ||
			strings.Contains(topicLower, "metadata") {
			return true
		}
	}

	dataLower := strings.ToLower(event.Data)
	if strings.Contains(dataLower, "stellar_asset") ||
		strings.Contains(dataLower, "sac_") {
		return true
	}

	return false
}
