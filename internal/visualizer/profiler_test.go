// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescribeHostFunction(t *testing.T) {
	doc := DescribeHostFunction("require_auth")
	assert.Contains(t, doc, "ensure")

	nonexistent := DescribeHostFunction("unknown_fn")
	assert.Contains(t, nonexistent, "host function")
}

func TestFormatGasSummary(t *testing.T) {
	assert.Equal(t, "CPU: 150000 instructions · Memory: 2048 bytes", FormatGasSummary(150000, 2048))
}

func TestEstimateGasHint(t *testing.T) {
	high := EstimateGasHint(250000, 8192)
	assert.Contains(t, high, "High resource usage")

	low := EstimateGasHint(30000, 512)
	assert.Contains(t, low, "Low resource usage")
}

func TestDiagnosticsForSource(t *testing.T) {
	source := "require_auth(account)\nstorage_put(key, value)\nunchanged_line"
	hints := DiagnosticsForSource(source)
	assert.Len(t, hints, 2)
	assert.Equal(t, 0, hints[0].Line)
	assert.Contains(t, hints[0].Message, "require_auth")
	assert.Equal(t, 1, hints[1].Line)
	assert.Contains(t, hints[1].Message, "storage_put")
}
