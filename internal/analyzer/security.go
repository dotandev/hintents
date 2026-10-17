// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

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

// DefaultStorageGrowthThresholdBytes is the default byte threshold past which
// a storage entry's size is flagged as potentially unbounded.
const DefaultStorageGrowthThresholdBytes = 1024

// StorageGrowthConfig configures parameters for storage growth diagnostics.
type StorageGrowthConfig struct {
	GrowthThresholdBytes int
}

type SecurityAnalyzer struct {
	violations []SecurityViolation
	config     StorageGrowthConfig
}

func NewSecurityAnalyzer() *SecurityAnalyzer {
	return &SecurityAnalyzer{
		violations: make([]SecurityViolation, 0),
		config: StorageGrowthConfig{
			GrowthThresholdBytes: DefaultStorageGrowthThresholdBytes,
		},
	}
}

func NewSecurityAnalyzerWithConfig(cfg StorageGrowthConfig) *SecurityAnalyzer {
	if cfg.GrowthThresholdBytes <= 0 {
		cfg.GrowthThresholdBytes = DefaultStorageGrowthThresholdBytes
	}
	return &SecurityAnalyzer{
		violations: make([]SecurityViolation, 0),
		config:     cfg,
	}
}

func (sa *SecurityAnalyzer) Analyze(resp *simulator.SimulationResponse) []SecurityViolation {
	sa.violations = make([]SecurityViolation, 0)

	if resp.Status != "success" {
		return sa.violations
	}

	sa.checkUnauthorizedStateModifications(resp.CategorizedEvents)
	sa.checkUnboundedStorageGrowth(resp)

	return sa.violations
}

func (sa *SecurityAnalyzer) checkUnboundedStorageGrowth(resp *simulator.SimulationResponse) {
	threshold := sa.config.GrowthThresholdBytes
	if threshold <= 0 {
		threshold = DefaultStorageGrowthThresholdBytes
	}

	// 1. Check storage writes in categorized events
	for i, event := range resp.CategorizedEvents {
		if event.EventType == "storage_write" {
			contractID := ""
			if event.ContractID != nil {
				contractID = *event.ContractID
			}

			// Check storage payload size growth against threshold
			dataSize := len(event.Data)
			if dataSize > threshold {
				sa.violations = append(sa.violations, SecurityViolation{
					Type:        "UnboundedStorageGrowth",
					Description: fmt.Sprintf("Storage entry in contract %s exceeds growth threshold (%d bytes > %d bytes)", contractID, dataSize, threshold),
					Severity:    "medium",
					Location:    fmt.Sprintf("event_index:%d", i),
				})
			}

			// Check if the storage key is keyed by caller/user input
			for _, topic := range event.Topics {
				topicLower := strings.ToLower(topic)
				if strings.Contains(topicLower, "user_input") ||
					strings.Contains(topicLower, "caller_supplied") ||
					strings.Contains(topicLower, "caller_input") ||
					strings.Contains(topicLower, "user_key") {
					sa.violations = append(sa.violations, SecurityViolation{
						Type:        "UserInputKeyedStorage",
						Description: fmt.Sprintf("Storage entry in contract %s is keyed by caller-supplied input", contractID),
						Severity:    "medium",
						Location:    fmt.Sprintf("event_index:%d", i),
					})
					break
				}
			}
		}
	}

	// 2. Check snapshot ledger entries if available
	if resp.OptimizationReport != nil && resp.OptimizationReport.Snapshots != nil {
		for snapshotName, snapshot := range resp.OptimizationReport.Snapshots.Inline {
			for idx, entry := range snapshot.LedgerEntries {
				entrySize := 0
				for _, part := range entry {
					entrySize += len(part)
				}
				if entrySize > threshold {
					sa.violations = append(sa.violations, SecurityViolation{
						Type:        "UnboundedStorageGrowth",
						Description: fmt.Sprintf("Snapshot %s ledger entry exceeds storage threshold (%d bytes > %d bytes)", snapshotName, entrySize, threshold),
						Severity:    "medium",
						Location:    fmt.Sprintf("snapshot:%s:entry:%d", snapshotName, idx),
					})
				}
			}
		}
	}
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
