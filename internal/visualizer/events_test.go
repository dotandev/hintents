// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/decoder"
)

func TestGenerateEventTree(t *testing.T) {
	root := &decoder.CallNode{
		ContractID: "ROOT",
		Function:   "TOP_LEVEL",
		SubCalls: []*decoder.CallNode{
			{
				ContractID: "CA12345678901234567890",
				Function:   "main_fn",
				Events: []decoder.DecodedEvent{
					{Topics: []string{"fn_call", "main_fn"}, Data: ""},
					{Topics: []string{"storage_read", "key1"}, Data: "value1"},
				},
				SubCalls: []*decoder.CallNode{
					{
						ContractID: "CB09876543210987654321",
						Function:   "sub_fn",
						Events: []decoder.DecodedEvent{
							{Topics: []string{"fn_call", "sub_fn"}, Data: ""},
							{Topics: []string{"transfer"}, Data: "100 XLM"},
							{Topics: []string{"fn_return", "sub_fn"}, Data: ""},
						},
					},
				},
			},
		},
	}

	output := GenerateEventTree(root)

	// Basic check for expected content
	if !strings.Contains(output, "main_fn") {
		t.Errorf("Output missing main function name")
	}
	if !strings.Contains(output, "sub_fn") {
		t.Errorf("Output missing sub function name")
	}
	if !strings.Contains(output, "storage_read, key1") {
		t.Errorf("Output missing event topics")
	}
	if !strings.Contains(output, "value1") {
		t.Errorf("Output missing event data")
	}
	if !strings.Contains(output, "transfer") {
		t.Errorf("Output missing nested event")
	}

	// Check for tree structure markers
	if !strings.Contains(output, "└── ") && !strings.Contains(output, "├── ") {
		t.Errorf("Output missing tree markers")
	}
}
