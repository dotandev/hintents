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

package analyzer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
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
