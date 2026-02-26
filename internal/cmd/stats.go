// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dotandev/hintents/internal/session"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

const (
	statsTopN = 5

	// Ledger resource cost weights for estimating "expensive" calls.
	costWeightStorageWrite = 3
	costWeightAuth         = 2
	costWeightDefault      = 1
)

var statsSessionFlag string

type contractStat struct {
	contractID    string
	eventCount    int
	storageWrites int
	authChecks    int
	estimatedCost uint64
	callDepth     int
	seenTypes     map[string]bool
}

var statsCmd = &cobra.Command{
	Use:     "stats",
	GroupID: "utility",
	Short:   "Summarize budget usage and call depth for the top contract calls",
	Long: `Returns a non-interactive table of the top 5 most expensive contract calls.

Cost is estimated based on weighted operations:
  - Storage writes: weight 3
  - Auth checks:    weight 2
  - Other events:   weight 1

Call depth counts the number of distinct event types observed per contract.`,
	Args: cobra.NoArgs,
	RunE: runStats,
}

func runStats(cmd *cobra.Command, args []string) error {
	simResp, err := loadSimulationResponse(cmd, statsSessionFlag)
	if err != nil {
		return err
	}

	stats := buildContractStats(simResp)
	if len(stats) == 0 {
		fmt.Println("No contract call data found in the session.")
		return nil
	}

	printStatsTable(stats)
	return nil
}

func loadSimulationResponse(cmd *cobra.Command, id string) (*simulator.SimulationResponse, error) {
	if id != "" {
		store, err := session.NewStore()
		if err != nil {
			return nil, fmt.Errorf("failed to open session store: %w", err)
		}
		defer store.Close()

		data, err := resolveSessionInput(cmd.Context(), store, id)
		if err != nil {
			return nil, err
		}
		return data.ToSimulationResponse()
	}

	data := GetCurrentSession()
	if data == nil {
		return nil, fmt.Errorf("no active session. Run 'erst debug <tx-hash>' first")
	}
	return data.ToSimulationResponse()
}

func buildContractStats(resp *simulator.SimulationResponse) []contractStat {
	index := make(map[string]*contractStat)

	process := func(contractID *string, eventType string) {
		if contractID == nil || *contractID == "" {
			return
		}
		id := *contractID
		if _, ok := index[id]; !ok {
			index[id] = &contractStat{contractID: id, seenTypes: make(map[string]bool)}
		}

		s := index[id]
		s.eventCount++
		s.estimatedCost += eventCost(eventType)

		lowerType := strings.ToLower(eventType)
		if lowerType == "storage_write" {
			s.storageWrites++
		} else if lowerType == "require_auth" || lowerType == "auth" {
			s.authChecks++
		}

		if !s.seenTypes[lowerType] {
			s.seenTypes[lowerType] = true
			s.callDepth++
		}
	}

	for _, e := range resp.CategorizedEvents {
		process(e.ContractID, e.EventType)
	}

	if len(index) == 0 {
		for _, e := range resp.DiagnosticEvents {
			process(e.ContractID, e.EventType)
		}
	}

	result := make([]contractStat, 0, len(index))
	for _, s := range index {
		result = append(result, *s)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].estimatedCost != result[j].estimatedCost {
			return result[i].estimatedCost > result[j].estimatedCost
		}
		return result[i].contractID < result[j].contractID
	})

	if len(result) > statsTopN {
		result = result[:statsTopN]
	}
	return result
}

func eventCost(eventType string) uint64 {
	switch strings.ToLower(eventType) {
	case "storage_write":
		return uint64(costWeightStorageWrite)
	case "require_auth", "auth":
		return uint64(costWeightAuth)
	default:
		return uint64(costWeightDefault)
	}
}

func printStatsTable(stats []contractStat) {
	const colContract = 44
	fmt.Printf("Top %d most expensive contract calls\n\n", statsTopN)
	fmt.Printf("%-44s | %-12s | %-7s\n", "Contract ID", "Est. Cost", "Depth")
	fmt.Println(strings.Repeat("-", colContract+23))

	for i, s := range stats {
		displayID := s.contractID
		if len(displayID) > colContract {
			displayID = displayID[:colContract-3] + "..."
		}
		fmt.Printf("%d. %-41s | %-12d | %-7d\n", i+1, displayID, s.estimatedCost, s.callDepth)
	}
}

func init() {
	statsCmd.Flags().StringVar(&statsSessionFlag, "session", "", "Load a saved session by ID")
	rootCmd.AddCommand(statsCmd)
}
