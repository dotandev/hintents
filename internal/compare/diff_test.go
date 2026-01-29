// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package compare

import (
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/require"
)

func TestCompareResults_Identical(t *testing.T) {
	onChain := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1", "event2"},
		Logs:   []string{"log1"},
	}
	local := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1", "event2"},
		Logs:   []string{"log1"},
	}

	diff := CompareResults(onChain, local)
	require.False(t, diff.StatusChanged)
	require.Equal(t, 2, len(diff.EventsDiff))
	require.Equal(t, "unchanged", diff.EventsDiff[0].Type)
	require.Equal(t, "unchanged", diff.EventsDiff[1].Type)
	require.Equal(t, "No differences found", diff.Summary)
}

func TestCompareResults_DifferentEvents(t *testing.T) {
	onChain := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1", "event2"},
		Logs:   []string{"log1"},
	}
	local := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1", "event3"}, // event2 -> event3
		Logs:   []string{"log1"},
	}

	diff := CompareResults(onChain, local)
	require.False(t, diff.StatusChanged)
	require.Equal(t, 2, len(diff.EventsDiff))
	require.Equal(t, "unchanged", diff.EventsDiff[0].Type)
	require.Equal(t, "modified", diff.EventsDiff[1].Type)
	require.Contains(t, diff.Summary, "event(s) differ")
}

func TestCompareResults_AddedEvent(t *testing.T) {
	onChain := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1"},
		Logs:   []string{},
	}
	local := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1", "event2"}, // added event2
		Logs:   []string{},
	}

	diff := CompareResults(onChain, local)
	require.Equal(t, 2, len(diff.EventsDiff))
	require.Equal(t, "unchanged", diff.EventsDiff[0].Type)
	require.Equal(t, "added", diff.EventsDiff[1].Type)
	require.Equal(t, "event2", diff.EventsDiff[1].Local)
}

func TestCompareResults_RemovedEvent(t *testing.T) {
	onChain := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1", "event2"},
		Logs:   []string{},
	}
	local := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{"event1"}, // removed event2
		Logs:   []string{},
	}

	diff := CompareResults(onChain, local)
	require.Equal(t, 2, len(diff.EventsDiff))
	require.Equal(t, "unchanged", diff.EventsDiff[0].Type)
	require.Equal(t, "removed", diff.EventsDiff[1].Type)
	require.Equal(t, "event2", diff.EventsDiff[1].OnChain)
}

func TestCompareResults_DifferentStatus(t *testing.T) {
	onChain := &simulator.SimulationResponse{
		Status: "success",
		Events: []string{},
		Logs:   []string{},
	}
	local := &simulator.SimulationResponse{
		Status: "error",
		Error:  "something broke",
		Events: []string{},
		Logs:   []string{},
	}

	diff := CompareResults(onChain, local)
	require.True(t, diff.StatusChanged)
	require.Contains(t, diff.Summary, "status changed")
}
