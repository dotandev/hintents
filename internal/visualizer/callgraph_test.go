// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/decoder"
	"github.com/stretchr/testify/assert"
)

func TestGenerateCallGraphSVG(t *testing.T) {
	// Create a mock call tree
	root := &decoder.CallNode{
		ContractID: "ROOT",
		Function:   "TOP_LEVEL",
		SubCalls: []*decoder.CallNode{
			{
				ContractID:      "CONTRACT_A",
				Function:        "funcA",
				CPUInstructions: 1000,
				MemoryBytes:     512,
				SubCalls: []*decoder.CallNode{
					{
						ContractID:      "CONTRACT_B",
						Function:        "funcB",
						CPUInstructions: 500,
						MemoryBytes:     256,
					},
				},
			},
			{
				ContractID:      "CONTRACT_C",
				Function:        "funcC",
				CPUInstructions: 2000,
				MemoryBytes:     1024,
			},
		},
	}

	svg := GenerateCallGraphSVG(root)

	// Basic SVG verification
	assert.Contains(t, svg, "<svg")
	assert.Contains(t, svg, "</svg>")
	assert.Contains(t, svg, "funcA")
	assert.Contains(t, svg, "funcB")
	assert.Contains(t, svg, "funcC")
	assert.Contains(t, svg, "CPU: 1000")
	assert.Contains(t, svg, "CPU: 500")
	assert.Contains(t, svg, "CPU: 2000")
	assert.Contains(t, svg, "Mem: 512 B")
	assert.Contains(t, svg, "Mem: 256 B")
	assert.Contains(t, svg, "Mem: 1.0 KB")

	// Verify dark mode styles are present
	assert.Contains(t, svg, "@media (prefers-color-scheme: dark)")
	assert.Contains(t, svg, "--bg: #0d1117")
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		b        uint64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{2048, "2.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1536 * 1024, "1.5 MB"},
	}

	for _, tt := range tests {
		actual := formatBytes(tt.b)
		if actual != tt.expected {
			t.Errorf("formatBytes(%d) = %q, expected %q", tt.b, actual, tt.expected)
		}
	}
}
