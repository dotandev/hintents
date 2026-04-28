// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"sort"
	"strings"
)

// Event represents a contract call event with its parameters and execution order.
type Event struct {
	Name     string
	Contract string
	Params   map[string]string
	Index    int
}

// kvPair represents a key-value pair for deterministic parameter rendering.
type kvPair struct {
	key   string
	value string
}

// RenderEventTree produces a structured ASCII tree view of contract events.
// It returns "No events emitted" for empty or nil input.
func RenderEventTree(events []Event) string {
	if len(events) == 0 {
		return "No events emitted"
	}

	// Work on a copy to avoid mutating the input slice.
	sorted := make([]Event, len(events))
	copy(sorted, events)
	sortEvents(sorted)

	var sb strings.Builder
	sb.WriteString("Events:\n")

	for i, event := range sorted {
		isLastEvent := i == len(sorted)-1
		renderEvent(&sb, event, isLastEvent)
	}

	return sb.String()
}

// renderEvent handles the ASCII tree rendering for a single event and its parameters.
func renderEvent(sb *strings.Builder, event Event, isLast bool) {
	name := safeName(event.Name)
	marker := "├── "
	if isLast {
		marker = "└── "
	}

	sb.WriteString(marker + name + "\n")

	params := formatParams(event.Params)
	if len(params) == 0 {
		return
	}

	maxKeyLen := 0
	for _, p := range params {
		if len(p.key) > maxKeyLen {
			maxKeyLen = len(p.key)
		}
	}

	// buildTreePrefix generates the indentation for the child parameters.
	prefix := buildTreePrefix(isLast, "")

	for j, p := range params {
		isLastParam := j == len(params)-1
		paramMarker := "├── "
		if isLastParam {
			paramMarker = "└── "
		}

		// Align values using padding based on maxKeyLen.
		padding := strings.Repeat(" ", maxKeyLen-len(p.key))
		val := truncateValue(p.value)

		fmt.Fprintf(sb, "%s%s%s: %s%s\n", prefix, paramMarker, p.key, padding, val)
	}
}

// sortEvents performs a stable sort of events based on their Index.
func sortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Index < events[j].Index
	})
}

// formatParams converts a map into a sorted slice of key-value pairs for determinism.
func formatParams(params map[string]string) []kvPair {
	if params == nil {
		return nil
	}

	pairs := make([]kvPair, 0, len(params))
	for k, v := range params {
		pairs = append(pairs, kvPair{key: k, value: v})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].key < pairs[j].key
	})

	return pairs
}

// buildTreePrefix constructs the indentation prefix for child nodes.
func buildTreePrefix(isLast bool, parentPrefix string) string {
	if isLast {
		return parentPrefix + "    "
	}
	return parentPrefix + "│   "
}

// truncateValue returns a truncated string if it exceeds 80 characters.
func truncateValue(s string) string {
	const maxLen = 80
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// safeName returns the event name or "UnknownEvent" if empty.
func safeName(name string) string {
	if name == "" {
		return "UnknownEvent"
	}
	return name
}
