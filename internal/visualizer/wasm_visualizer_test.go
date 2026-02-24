// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"strings"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stretchr/testify/assert"
)

func TestGenerateWasmReport(t *testing.T) {
	analysis := &simulator.WasmAnalysis{
		TotalSize: 1000,
		Sections: []simulator.WasmSection{
			{Name: "code", Size: 500, Category: "Logic"},
			{Name: ".debug_info", Size: 400, Category: "Debug"},
			{Name: "data", Size: 100, Category: "Data"},
		},
	}

	html := GenerateWasmReport(analysis)

	assert.True(t, strings.Contains(html, "WASM Binary Analysis"))
	assert.True(t, strings.Contains(html, "1000"))
	assert.True(t, strings.Contains(html, "Logic"))
	assert.True(t, strings.Contains(html, "Debug Info"))
	assert.True(t, strings.Contains(html, "cat-Logic"))
	assert.True(t, strings.Contains(html, "cat-Debug"))
}
