// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"fmt"
	"time"

	"github.com/dotandev/hintents/internal/simulator"
)

// ReportData contains the debugging report information
type ReportData struct {
	TraceID          string
	TxHash           string
	Network          string
	Status           string
	Error            string
	Timestamp        time.Time
	AuditLogURL      string
	DiagnosticEvents []simulator.DiagnosticEvent
	Logs             []string
}

// SlackMessage represents Slack webhook payload
type SlackMessage struct {
	Blocks []interface{} `json:"blocks"`
	Text   string        `json:"text"`
}

// DiscordMessage represents Discord webhook payload
type DiscordMessage struct {
	Username string         `json:"username"`
	Content  string         `json:"content"`
	Embeds   []DiscordEmbed `json:"embeds"`
}

// DiscordEmbed represents a Discord embed
type DiscordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Fields      []DiscordEmbedField `json:"fields"`
	Timestamp   string              `json:"timestamp"`
	Footer      DiscordEmbedFooter  `json:"footer"`
}

// DiscordEmbedField represents a field in Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// DiscordEmbedFooter represents footer in Discord embed
type DiscordEmbedFooter struct {
	Text string `json:"text"`
}

// FormatSlackMessage creates a formatted Slack webhook message
func FormatSlackMessage(report ReportData) SlackMessage {
	headerSection := map[string]interface{}{
		"type": "header",
		"text": map[string]interface{}{
			"type": "plain_text",
			"text": "🔍 ERST Debugging Report",
		},
	}

	statusEmoji := "❌"
	if report.Status == "success" {
		statusEmoji = "✅"
	}

	summaryBlock := map[string]interface{}{
		"type": "section",
		"text": map[string]interface{}{
			"type": "mrkdwn",
			"text": fmt.Sprintf(
				"%s *Status:* %s\n*Network:* %s\n*Timestamp:* %s",
				statusEmoji,
				report.Status,
				report.Network,
				report.Timestamp.Format("2006-01-02 15:04:05 MST"),
			),
		},
	}

	blocks := []interface{}{headerSection, summaryBlock}

	// Add transaction info
	txBlock := map[string]interface{}{
		"type": "section",
		"fields": []interface{}{
			map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*TX Hash:*\n`%s`", report.TxHash),
			},
			map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Trace ID:*\n`%s`", report.TraceID),
			},
		},
	}
	blocks = append(blocks, txBlock)

	// Add error details if present
	if report.Error != "" {
		errorBlock := map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Error:*\n```%s```", truncateString(report.Error, 500)),
			},
		}
		blocks = append(blocks, errorBlock)
	}

	// Add diagnostic events summary
	if len(report.DiagnosticEvents) > 0 {
		eventsText := fmt.Sprintf("*Events:* %d diagnostic events recorded", len(report.DiagnosticEvents))
		if len(report.DiagnosticEvents) > 0 && len(report.DiagnosticEvents) <= 3 {
			eventsText += "\n"
			for i, evt := range report.DiagnosticEvents {
				eventsText += fmt.Sprintf("• %s: %s\n", evt.EventType, truncateString(evt.Data, 100))
				if i >= 2 {
					break
				}
			}
		}
		eventsBlock := map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": eventsText,
			},
		}
		blocks = append(blocks, eventsBlock)
	}

	// Add action buttons
	elements := []interface{}{}
	if report.AuditLogURL != "" {
		elements = append(elements, map[string]interface{}{
			"type": "button",
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": "View Audit Log",
			},
			"url":   report.AuditLogURL,
			"style": "primary",
		})
	}

	if len(elements) > 0 {
		actionBlock := map[string]interface{}{
			"type":     "actions",
			"elements": elements,
		}
		blocks = append(blocks, actionBlock)
	}

	// Add divider
	blocks = append(blocks, map[string]interface{}{
		"type": "divider",
	})

	return SlackMessage{
		Blocks: blocks,
		Text:   fmt.Sprintf("ERST Debugging Report - %s", report.Status),
	}
}

// FormatDiscordMessage creates a formatted Discord webhook message
func FormatDiscordMessage(report ReportData) DiscordMessage {
	color := colorForStatus(report.Status)
	colorInt := hexToDecimal(color)

	statusTitle := "❌ Simulation Failed"
	if report.Status == "success" {
		statusTitle = "✅ Simulation Succeeded"
	}

	fields := []DiscordEmbedField{
		{
			Name:   "Network",
			Value:  report.Network,
			Inline: true,
		},
		{
			Name:   "Status",
			Value:  report.Status,
			Inline: true,
		},
		{
			Name:   "TX Hash",
			Value:  fmt.Sprintf("`%s`", report.TxHash),
			Inline: false,
		},
		{
			Name:   "Trace ID",
			Value:  fmt.Sprintf("`%s`", report.TraceID),
			Inline: false,
		},
	}

	// Add error if present
	if report.Error != "" {
		fields = append(fields, DiscordEmbedField{
			Name:   "Error",
			Value:  fmt.Sprintf("```\n%s\n```", truncateString(report.Error, 400)),
			Inline: false,
		})
	}

	// Add diagnostic events summary
	if len(report.DiagnosticEvents) > 0 {
		eventsValue := fmt.Sprintf("Recorded %d diagnostic events", len(report.DiagnosticEvents))
		if len(report.DiagnosticEvents) <= 3 {
			eventsValue += "\n"
			for i, evt := range report.DiagnosticEvents {
				eventsValue += fmt.Sprintf("• `%s`: %s\n", evt.EventType, truncateString(evt.Data, 80))
				if i >= 2 {
					break
				}
			}
		}
		fields = append(fields, DiscordEmbedField{
			Name:   "Diagnostic Events",
			Value:  eventsValue,
			Inline: false,
		})
	}

	// Add audit log link if available
	if report.AuditLogURL != "" {
		fields = append(fields, DiscordEmbedField{
			Name:   "Links",
			Value:  fmt.Sprintf("[View Audit Log](%s)", report.AuditLogURL),
			Inline: false,
		})
	}

	embed := DiscordEmbed{
		Title:       statusTitle,
		Description: fmt.Sprintf("Debugging report from %s", report.Timestamp.Format("2006-01-02 15:04:05 MST")),
		Color:       colorInt,
		Fields:      fields,
		Timestamp:   report.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		Footer: DiscordEmbedFooter{
			Text: "ERST Simulator",
		},
	}

	return DiscordMessage{
		Username: "ERST Debugger",
		Content:  fmt.Sprintf("New debugging report: %s", report.Status),
		Embeds:   []DiscordEmbed{embed},
	}
}

// Helper functions

func colorForStatus(status string) string {
	switch status {
	case "success":
		return "36a64f" // Green
	case "error":
		return "e74c3c" // Red
	case "warning":
		return "f39c12" // Orange
	default:
		return "95a5a6" // Gray
	}
}

func hexToDecimal(hex string) int {
	var value int
	fmt.Sscanf(hex, "%x", &value)
	return value
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
