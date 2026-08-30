// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/audit"
)

// FromDump converts a raw Dump into a Report for HTML/PDF rendering.
func FromDump(dump *audit.Dump) *Report {
	r := NewReport("Audit Report")

	ts, err := parseTimestamp(dump.Timestamp)
	if err == nil {
		r.GeneratedAt = ts
	}

	r.Metadata.DataSource = "audit-dump"

	r.Summary.TotalEvents = len(dump.Events)
	r.Summary.Status = "complete"

	steps := dumpToSteps(dump)
	r.Execution.Steps = steps

	r.Analytics.EventDistribution = buildEventDistribution(dump.Events)

	return r
}

// FromSignedDump converts a SignedDump into a Report, including integrity metadata.
func FromSignedDump(dump *audit.SignedDump) *Report {
	r := FromDump(&dump.Trace)
	r.Title = "Signed Audit Report"

	r.Metadata.DataSource = fmt.Sprintf("signed-audit-dump (signer: %s)", dump.Signer.Provider)
	r.Metadata.Tags = map[string]string{
		"algorithm":  dump.Algorithm,
		"hash":       dump.Hash,
		"signer":     dump.Signer.Provider,
		"public_key": truncate(dump.PublicKey, 64),
		"signature":  truncate(dump.Signature, 32) + "...",
	}

	return r
}

// ParseDump deserialises raw JSON into an Dump.
func ParseDump(data []byte) (*audit.Dump, error) {
	return audit.ParseDump(data)
}

// ParseSignedDump deserialises raw JSON into a SignedDump.
func ParseSignedDump(data []byte) (*audit.SignedDump, error) {
	return audit.ParseSignedDump(data)
}

// RenderDumpHTML is a convenience function: parse JSON bytes and render directly to HTML.
func RenderDumpHTML(data []byte) ([]byte, error) {
	// Try signed first (it has additional top-level fields).
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var report *Report
	if _, ok := probe["trace"]; ok {
		dump, err := ParseSignedDump(data)
		if err != nil {
			return nil, err
		}
		report = FromSignedDump(dump)
	} else {
		dump, err := ParseDump(data)
		if err != nil {
			return nil, err
		}
		report = FromDump(dump)
	}

	return NewHTMLRenderer().Render(report)
}

// dumpToSteps converts the input, state, and events into ExecutionSteps for the report.
func dumpToSteps(dump *audit.Dump) []ExecutionStep {
	var steps []ExecutionStep
	idx := 0

	if len(dump.Input) > 0 {
		steps = append(steps, ExecutionStep{
			Index:     idx,
			Operation: "input",
			Status:    "success",
			Details:   "Execution input parameters",
			Input:     dump.Input,
		})
		idx++
	}

	for i, ev := range dump.Events {
		var label string
		switch v := ev.(type) {
		case string:
			label = v
		default:
			b, _ := json.Marshal(v)
			label = string(b)
		}
		steps = append(steps, ExecutionStep{
			Index:     idx + i,
			Operation: "event",
			Status:    "success",
			Details:   label,
		})
	}

	if len(dump.State) > 0 {
		steps = append(steps, ExecutionStep{
			Index:     idx + len(dump.Events),
			Operation: "state-snapshot",
			Status:    "success",
			Details:   "Final state snapshot",
			Output:    dump.State,
		})
	}

	return steps
}

func buildEventDistribution(events []interface{}) map[string]int {
	dist := make(map[string]int)
	for _, ev := range events {
		key := "unknown"
		switch v := ev.(type) {
		case string:
			key = v
		case map[string]interface{}:
			if t, ok := v["type"].(string); ok {
				key = t
			}
		}
		dist[key]++
	}
	return dist
}

func parseTimestamp(ts string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, ts); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp: %q", ts)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}
