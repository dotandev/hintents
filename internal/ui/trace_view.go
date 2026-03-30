// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"sort"

	"github.com/dotandev/hintents/internal/debug"
	"github.com/dotandev/hintents/internal/visualizer"
)

// TraceView handles displaying the execution trace with highlighting
type TraceView struct {
	registry *debug.Registry
}

// NewTraceView creates a new trace view for the given registry
func NewTraceView(registry *debug.Registry) *TraceView {
	return &TraceView{registry: registry}
}

// DisplayTrace displays the trace tree, highlighting snapshots that match the search
func (tv *TraceView) DisplayTrace(highlightedTimestamps []int64) {
	if tv.registry == nil || len(tv.registry.Entries) == 0 {
		fmt.Println("No trace data available")
		return
	}

	// Sort highlighted timestamps for efficient lookup
	sort.Slice(highlightedTimestamps, func(i, j int) bool {
		return highlightedTimestamps[i] < highlightedTimestamps[j]
	})

	fmt.Println("Execution Trace:")
	fmt.Println("================")

	for i, entry := range tv.registry.Entries {
		timestamp := entry.Timestamp
		isHighlighted := contains(highlightedTimestamps, timestamp)

		if isHighlighted {
			fmt.Printf("%s [%d] Timestamp: %d - LEDGER KEY CHANGED\n", visualizer.Colorize("🔍", "yellow"), i, timestamp)
		} else {
			fmt.Printf("   [%d] Timestamp: %d\n", i, timestamp)
		}
	}
}

// DisplayTraceWithSearch displays the trace tree, highlighting snapshots that match the search query
func (tv *TraceView) DisplayTraceWithSearch(searchQuery string) error {
	filter, err := ParseSearchQuery(searchQuery)
	if err != nil {
		return err
	}

	var highlighted []int64
	if filter.Type == "changed-key" {
		highlighted = tv.registry.FindChangedKeySnapshots(filter.Key)
	}

	tv.DisplayTrace(highlighted)
	return nil
}

// contains checks if sorted timestamps include the given target.
func contains(sorted []int64, target int64) bool {
	idx := sort.Search(len(sorted), func(i int) bool {
		return sorted[i] >= target
	})
	return idx < len(sorted) && sorted[idx] == target
}