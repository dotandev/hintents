// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/decoder"
)

// GenerateEventTreeFromCallGraph produces a structured ASCII tree view of contract events
// emitted during a transaction, grouped by call hierarchy.
func GenerateEventTreeFromCallGraph(root *decoder.CallNode) string {
	if root == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(Colorize("Event Trace Tree", "bold") + "\n")

	// Start recursion
	renderEventNode(root, "", true, &sb)

	return sb.String()
}

func renderEventNode(node *decoder.CallNode, indent string, isLast bool, sb *strings.Builder) {
	// Root TOP_LEVEL node check
	isRoot := node.Function == "TOP_LEVEL" && node.ContractID == "ROOT"

	var childIndent string
	if !isRoot {
		marker := "├── "
		if isLast {
			marker = "└── "
		}

		// Call header with function and contract ID
		header := fmt.Sprintf("%s (%s)",
			Colorize(node.Function, "cyan"),
			Colorize(formatShortContractID(node.ContractID), "dim"),
		)
		sb.WriteString(indent + marker + header + "\n")

		// Update indent for children
		if isLast {
			childIndent = indent + "    "
		} else {
			childIndent = indent + "│   "
		}
	} else {
		childIndent = indent
	}

	// Prepare items to show: non-meta events and subcalls
	var events []decoder.DecodedEvent
	for _, e := range node.Events {
		if len(e.Topics) > 0 && (e.Topics[0] == "fn_call" || e.Topics[0] == "fn_return") {
			continue
		}
		events = append(events, e)
	}

	totalItems := len(events) + len(node.SubCalls)
	
	// 1. Render Events
	for i, event := range events {
		itemIsLast := (i == totalItems-1)
		itemMarker := "├── "
		if itemIsLast {
			itemMarker = "└── "
		}

		formattedTopics := formatIndexedTopics(event.Topics)
		data := truncateValue(event.Data)
		
		eventLine := fmt.Sprintf("[%s] %s | data: %s",
			Colorize("EVENT", "yellow"),
			Colorize(formattedTopics, "dim"),
			data,
		)
		sb.WriteString(childIndent + itemMarker + eventLine + "\n")
	}

	// 2. Render SubCalls
	for i, child := range node.SubCalls {
		itemIsLast := (i+len(events) == totalItems-1)
		renderEventNode(child, childIndent, itemIsLast, sb)
	}
}

func formatIndexedTopics(topics []string) string {
	var parts []string
	for i, t := range topics {
		parts = append(parts, fmt.Sprintf("topic[%d]: %s", i, truncateValue(t)))
	}
	if len(parts) == 0 {
		return "no topics"
	}
	return strings.Join(parts, ", ")
}

func truncateValue(s string) string {
	if len(s) <= 24 {
		return s
	}
	// Truncate long hex-like strings
	return s[:10] + "..." + s[len(s)-10:]
}

func formatShortContractID(id string) string {
	if len(id) > 12 {
		return id[:6] + "..." + id[len(id)-4:]
	}
	return id
}
