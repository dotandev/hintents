// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
)

type GasInefficiency struct {
	Type           string
	Description    string
	Severity       string
	Location       string
	Recommendation string
}

type GasAnalyzer struct {
	inefficiencies []GasInefficiency
}

func NewGasAnalyzer() *GasAnalyzer {
	return &GasAnalyzer{
		inefficiencies: make([]GasInefficiency, 0),
	}
}

func (ga *GasAnalyzer) Analyze(resp *simulator.SimulationResponse) []GasInefficiency {
	ga.inefficiencies = make([]GasInefficiency, 0)

	if resp == nil || resp.Status != "success" {
		return ga.inefficiencies
	}

	ga.analyzeStoragePatterns(resp.CategorizedEvents)
	ga.analyzeMainnetFeeTrends(resp.CategorizedEvents)

	return ga.inefficiencies
}

func (ga *GasAnalyzer) analyzeStoragePatterns(events []simulator.CategorizedEvent) {
	contractWrites := make(map[string]int)
	contractReads := make(map[string]int)

	for i, event := range events {
		contractID := "unknown"
		if event.ContractID != nil && *event.ContractID != "" {
			contractID = *event.ContractID
		}

		// Detect state modifications and reads based on topics and types
		isWrite := event.EventType == "storage_write"
		isRead := event.EventType == "storage_read"
		
		// Some simulation traces might categorize events differently
		if !isWrite && !isRead {
			categoryLower := strings.ToLower(event.Category)
			typeLower := strings.ToLower(event.EventType)
			if strings.Contains(categoryLower, "storage") || strings.Contains(typeLower, "storage") {
				if strings.Contains(typeLower, "write") || strings.Contains(typeLower, "update") || strings.Contains(typeLower, "set") {
					isWrite = true
				} else if strings.Contains(typeLower, "read") || strings.Contains(typeLower, "get") {
					isRead = true
				}
			}
		}

		if isWrite {
			contractWrites[contractID]++
			if contractWrites[contractID] > 2 {
				ga.inefficiencies = append(ga.inefficiencies, GasInefficiency{
					Type:           "ExcessiveStorageWrites",
					Description:    fmt.Sprintf("Multiple sequential storage writes detected in contract %s. Mainnet fees heavily penalize multiple small footprint writes.", contractID),
					Severity:       "high",
					Location:       fmt.Sprintf("event_index:%d", i),
					Recommendation: "Combine related state into a single struct/enum to reduce Write fees and optimize footprint usage.",
				})
				contractWrites[contractID] = 0 // reset to avoid spamming
			}
		}

		if isRead {
			contractReads[contractID]++
			if contractReads[contractID] > 4 {
				ga.inefficiencies = append(ga.inefficiencies, GasInefficiency{
					Type:           "ExcessiveStorageReads",
					Description:    fmt.Sprintf("High number of storage reads detected in contract %s.", contractID),
					Severity:       "medium",
					Location:       fmt.Sprintf("event_index:%d", i),
					Recommendation: "Cache read values in memory to reduce Rent fees and redundant read operations.",
				})
				contractReads[contractID] = 0 // reset
			}
		}
	}
}

func (ga *GasAnalyzer) analyzeMainnetFeeTrends(events []simulator.CategorizedEvent) {
	// Simulated check for ledger operations that could be combined
	// based on mainnet fee trends (e.g. rent vs footprint size)
	for i, event := range events {
		if event.EventType == "contract_call" {
			// In a real scenario, this would check footprint size vs rent fee limits
			for _, topic := range event.Topics {
				if strings.Contains(strings.ToLower(topic), "extend_ttl") || strings.Contains(strings.ToLower(topic), "restore_footprint") {
					ga.inefficiencies = append(ga.inefficiencies, GasInefficiency{
						Type:           "SuboptimalRentManagement",
						Description:    "Frequent TTL extensions or footprint restorations detected.",
						Severity:       "medium",
						Location:       fmt.Sprintf("event_index:%d", i),
						Recommendation: "Batch TTL extensions together or use instance storage for data that shares the same lifecycle to save on Rent fees.",
					})
					break // Only report once per contract_call event for this pattern
				}
			}
		}
	}
}
