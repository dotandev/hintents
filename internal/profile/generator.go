// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package profile

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/dotandev/hintents/internal/trace"
)

//go:embed template.html
var flamegraphTemplate string

// FrameData holds per-frame metadata for the interactive flamegraph.
type FrameData struct {
	Name       string `json:"name"`
	Gas        int64  `json:"gas"`
	Step       int    `json:"step"`
	StateChurn int    `json:"state_churn"`
	SnapshotID int    `json:"snapshot_id"`
}

// SnapshotSummary is a lightweight record of a state snapshot for the flamegraph UI.
type SnapshotSummary struct {
	SnapshotID int `json:"snapshot_id"`
	Step       int `json:"step"`
	KeyCount   int `json:"key_count"`
}

// GenerateHTML writes an interactive flamegraph HTML page to w.
// Each frame carries State Churn (number of HostState keys modified at that step)
// and a Snapshot ID identifying the nearest snapshot, enabling click-to-jump navigation.
func GenerateHTML(execTrace *trace.ExecutionTrace, w io.Writer) error {
	if execTrace == nil {
		return fmt.Errorf("execution trace is nil")
	}

	frames := buildFrames(execTrace)
	summaries := buildSnapshotSummaries(execTrace)

	framesJSON, err := json.Marshal(frames)
	if err != nil {
		return fmt.Errorf("failed to marshal frames: %w", err)
	}
	summariesJSON, err := json.Marshal(summaries)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshots: %w", err)
	}

	tmpl, err := template.New("flamegraph").Parse(flamegraphTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse flamegraph template: %w", err)
	}

	data := map[string]interface{}{
		"Frames":    string(framesJSON),
		"Snapshots": string(summariesJSON),
		"TxHash":    execTrace.TransactionHash,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render flamegraph template: %w", err)
	}

	_, err = w.Write(buf.Bytes())
	return err
}

// buildFrames constructs a FrameData slice from the execution trace states.
func buildFrames(execTrace *trace.ExecutionTrace) []FrameData {
	frames := make([]FrameData, 0, len(execTrace.States))
	for i := range execTrace.States {
		state := &execTrace.States[i]
		name := functionName(state)
		if name == "" {
			name = state.Operation
		}
		if name == "" {
			name = fmt.Sprintf("step_%d", state.Step)
		}
		frames = append(frames, FrameData{
			Name:       name,
			Gas:        extractGasFromState(state),
			Step:       state.Step,
			StateChurn: len(state.HostState),
			SnapshotID: nearestSnapshotID(execTrace, state.Step),
		})
	}
	return frames
}

// buildSnapshotSummaries converts each snapshot into a SnapshotSummary for the UI.
func buildSnapshotSummaries(execTrace *trace.ExecutionTrace) []SnapshotSummary {
	summaries := make([]SnapshotSummary, 0, len(execTrace.Snapshots))
	for i := range execTrace.Snapshots {
		snap := &execTrace.Snapshots[i]
		summaries = append(summaries, SnapshotSummary{
			SnapshotID: i,
			Step:       snap.Step,
			KeyCount:   len(snap.HostState),
		})
	}
	return summaries
}

// nearestSnapshotID returns the index of the latest snapshot whose Step is <= step.
// Returns -1 when no snapshot exists at or before step.
func nearestSnapshotID(execTrace *trace.ExecutionTrace, step int) int {
	id := -1
	for i, snap := range execTrace.Snapshots {
		if snap.Step <= step {
			id = i
		} else {
			break
		}
	}
	return id
}
