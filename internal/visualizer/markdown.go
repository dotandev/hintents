// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"sort"
	"strings"
)

// TraceStep models one callgraph row for markdown export.
type TraceStep struct {
	Step     int
	Contract string
	Function string
	Caller   string
}

// Trace is a deterministic markdown-exportable execution trace.
type Trace struct {
	Steps []TraceStep
}

// ExportMarkdown renders a trace as a GitHub-friendly markdown table.
func ExportMarkdown(trace Trace) string {
	if len(trace.Steps) == 0 {
		return "| Step | Contract | Function | Caller |\n|------|----------|----------|--------|\n"
	}

	steps := make([]TraceStep, len(trace.Steps))
	copy(steps, trace.Steps)
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].Step != steps[j].Step {
			return steps[i].Step < steps[j].Step
		}
		if steps[i].Contract != steps[j].Contract {
			return steps[i].Contract < steps[j].Contract
		}
		if steps[i].Function != steps[j].Function {
			return steps[i].Function < steps[j].Function
		}
		return steps[i].Caller < steps[j].Caller
	})

	var b strings.Builder
	b.WriteString("| Step | Contract | Function | Caller |\n")
	b.WriteString("|------|----------|----------|--------|\n")
	for _, step := range steps {
		fmt.Fprintf(
			&b,
			"| %d | %s | %s | %s |\n",
			step.Step,
			escapeMarkdownCell(step.Contract),
			escapeMarkdownCell(step.Function),
			escapeMarkdownCell(step.Caller),
		)
	}

	return b.String()
}

func escapeMarkdownCell(v string) string {
	escaped := strings.ReplaceAll(v, "|", "\\|")
	return strings.TrimSpace(escaped)
}
