// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"strings"
	"testing"
)

func TestRenderEventTree(t *testing.T) {
	tests := []struct {
		name     string
		events   []Event
		expected string
	}{
		{
			name:     "Empty input",
			events:   []Event{},
			expected: "No events emitted",
		},
		{
			name:     "Nil slice",
			events:   nil,
			expected: "No events emitted",
		},
		{
			name: "Single event without params",
			events: []Event{
				{Name: "Transfer", Index: 1},
			},
			expected: "Events:\n└── Transfer\n",
		},
		{
			name: "Unknown event name fallback",
			events: []Event{
				{Name: "", Index: 1},
			},
			expected: "Events:\n└── UnknownEvent\n",
		},
		{
			name: "Sorting correctness",
			events: []Event{
				{Name: "Approval", Index: 2},
				{Name: "Transfer", Index: 1},
			},
			expected: "Events:\n" +
				"├── Transfer\n" +
				"└── Approval\n",
		},
		{
			name: "Key alignment and determinism",
			events: []Event{
				{
					Name: "Transfer",
					Index: 1,
					Params: map[string]string{
						"to":   "0xdef",
						"from": "0xabc",
					},
				},
			},
			expected: "Events:\n" +
				"└── Transfer\n" +
				"    ├── from: 0xabc\n" +
				"    └── to:   0xdef\n",
		},
		{
			name: "Long value truncation",
			events: []Event{
				{
					Name: "DataUpdate",
					Index: 1,
					Params: map[string]string{
						"value": strings.Repeat("A", 100),
					},
				},
			},
			expected: "Events:\n" +
				"└── DataUpdate\n" +
				"    └── value: " + strings.Repeat("A", 80) + "...\n",
		},
		{
			name: "Nil Params handling",
			events: []Event{
				{Name: "Ping", Index: 1, Params: nil},
			},
			expected: "Events:\n└── Ping\n",
		},
		{
			name: "Multiple events with varying param counts",
			events: []Event{
				{
					Name: "Transfer",
					Index: 1,
					Params: map[string]string{
						"from":  "0xabc",
						"to":    "0xdef",
						"value": "1000",
					},
				},
				{
					Name: "Approval",
					Index: 2,
					Params: map[string]string{
						"owner":   "0xabc",
						"spender": "0xdef",
					},
				},
			},
			expected: "Events:\n" +
				"├── Transfer\n" +
				"│   ├── from:  0xabc\n" +
				"│   ├── to:    0xdef\n" +
				"│   └── value: 1000\n" +
				"└── Approval\n" +
				"    ├── owner:   0xabc\n" +
				"    └── spender: 0xdef\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderEventTree(tt.events)
			// Normalize newlines for cross-platform comparison if necessary,
			// but the requirement says use "\n" ONLY.
			if got != tt.expected {
				t.Errorf("RenderEventTree() = \n%q, want \n%q", got, tt.expected)
				// Print visual diff for easier debugging
				t.Logf("GOT:\n%s", got)
				t.Logf("WANT:\n%s", tt.expected)
			}
		})
	}
}

