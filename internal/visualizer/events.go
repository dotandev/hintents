// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/decoder"
)

// GenerateEventTree produces a structured ASCII tree view of contract events
// emitted during a transaction, grouped by call hierarchy.
func GenerateEventTree(root *decoder.CallNode) string {
	if root == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(Colorize("Event Trace Tree", "bold") + "\n")

	renderEventNode(root, "", true, &sb)

	return sb.String()
}

func renderEventNode(node *decoder.CallNode, indent string, isLast bool, sb *strings.Builder) {
	isRoot := node.Function == "TOP_LEVEL" && node.ContractID == "ROOT"

	var childIndent string

	if !isRoot {
		marker := "├── "
		if isLast {
			marker = "└── "
		}

		header := fmt.Sprintf("%s (%s)",
			Colorize(node.Function, "cyan"),
			Colorize(formatShortContractID(node.ContractID), "dim"),
		)

		sb.WriteString(indent + marker + header + "\n")

		if isLast {
			childIndent = indent + "    "
		} else {
			childIndent = indent + "│   "
		}
	} else {
		childIndent = indent
	}

	// Filter events (remove fn_call / fn_return)
	var events []decoder.DecodedEvent
	for _, e := range node.Events {
		if len(e.Topics) > 0 && (e.Topics[0] == "fn_call" || e.Topics[0] == "fn_return") {
			continue
		}
		events = append(events, e)
	}

	totalItems := len(events) + len(node.SubCalls)
	currentIndex := 0

	// Render events
	for _, event := range events {
		itemIsLast := currentIndex == totalItems-1

		marker := "├── "
		if itemIsLast {
			marker = "└── "
		}

		topics := formatTopics(event.Topics)
		data := truncateValue(event.Data)

		line := fmt.Sprintf("[%s] %s | data: %s",
			Colorize("EVENT", "yellow"),
			topics,
			data,
)

		sb.WriteString(childIndent + marker + line + "\n")
		currentIndex++
	}

	// Render subcalls
	for _, child := range node.SubCalls {
		itemIsLast := currentIndex == totalItems-1
		renderEventNode(child, childIndent, itemIsLast, sb)
		currentIndex++
	}
}

func formatTopics(topics []string) string {
	if len(topics) == 0 {
		return "no topics"
	}

	var parts []string
	for _, t := range topics {
		parts = append(parts, truncateValue(t))
	}

	return strings.Join(parts, ", ")
}

func truncateValue(s string) string {
	if len(s) <= 24 {
		return s
	}
	return s[:10] + "..." + s[len(s)-10:]
}

func formatShortContractID(id string) string {
	if len(id) > 12 {
		return id[:6] + "..." + id[len(id)-4:]
	}
	return id
}
