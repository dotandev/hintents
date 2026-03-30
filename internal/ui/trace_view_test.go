// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/dotandev/hintents/internal/debug"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/stretchr/testify/assert"
)

func TestDisplayTraceWithSearch_highlightsChangedKey(t *testing.T) {
	reg := debug.New("v1", "txhash", "testnet", "env", "meta")
	reg.Add(1000, snapshot.FromMap(map[string]string{"k": "v1"}))
	reg.Add(2000, snapshot.FromMap(map[string]string{"k": "v2"}))
	reg.Add(3000, snapshot.FromMap(map[string]string{"k": "v2"}))

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	os.Stdout = w

	view := NewTraceView(reg)
	assert.NoError(t, view.DisplayTraceWithSearch("changed-key:k"))

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = originalStdout

	output := buf.String()
	assert.Contains(t, output, "🔍")
	assert.Contains(t, output, "🔍 [1] Timestamp: 2000 - LEDGER KEY CHANGED")
	assert.Contains(t, output, "   [0] Timestamp: 1000")
	assert.Contains(t, output, "   [2] Timestamp: 3000")
	fmt.Println(output)
}
