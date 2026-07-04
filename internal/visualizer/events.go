// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"strings"
)

// Event represents an emitted contract event during a contract call.
type Event struct {
	Type     string
	Metadata string
	Children []Event
}

// RenderEventTree renders a list of events as a structured ASCII tree.
func RenderEventTree(events []Event) string {
	if len(events) == 0 {
		return "No events recorded."
	}

	var sb strings.Builder
	sb.WriteString("Events\n")

	renderNodes(&sb, events, "")

	return strings.TrimSuffix(sb.String(), "\n")
}

// renderNodes is a recursive helper that builds the ASCII tree structure.
func renderNodes(sb *strings.Builder, events []Event, prefix string) {
	for i, event := range events {
		isLast := i == len(events)-1

		// Write the current level's prefix and branch character
		sb.WriteString(prefix)
		if isLast {
			sb.WriteString("└── ")
		} else {
			sb.WriteString("├── ")
		}

		// Write the event type
		sb.WriteString(event.Type)

		// Optionally write metadata if present
		if event.Metadata != "" {
			sb.WriteString(" (")
			sb.WriteString(event.Metadata)
			sb.WriteString(")")
		}

		sb.WriteByte('\n')

		// Recursively render children with updated prefix
		if len(event.Children) > 0 {
			nextPrefix := prefix
			if isLast {
				nextPrefix += "    "
			} else {
				nextPrefix += "│   "
			}
			renderNodes(sb, event.Children, nextPrefix)
		}
	}
}
