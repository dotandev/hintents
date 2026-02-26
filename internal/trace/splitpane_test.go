// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTempSource creates a temp file with the given lines and returns its path.
func writeTempSource(t *testing.T, lines []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.rs")
	require.NoError(t, err)
	_, err = f.WriteString(strings.Join(lines, "\n"))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoadSourceContext_RoundTrip(t *testing.T) {
	src := []string{
		"fn transfer(amount: u64) {",
		"    let balance = get_balance();",
		"    require(balance >= amount);",
		"    do_transfer(amount);",
		"}",
	}
	path := writeTempSource(t, src)
	ctx, err := LoadSourceContext(SourceRef{File: path, Line: 3}, 2)
	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.Equal(t, "    require(balance >= amount);", ctx.Lines[ctx.FocusIndex])
	assert.Equal(t, 2, ctx.FocusIndex)
}

func TestLoadSourceContext_FileNotFound(t *testing.T) {
	_, err := LoadSourceContext(SourceRef{File: filepath.Join(t.TempDir(), "missing.rs"), Line: 1}, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot open")
}

func TestLoadSourceContext_LineClampedToEnd(t *testing.T) {
	src := []string{"line1", "line2", "line3"}
	path := writeTempSource(t, src)
	ctx, err := LoadSourceContext(SourceRef{File: path, Line: 99}, 2)
	require.NoError(t, err)
	assert.Equal(t, "line3", ctx.Lines[ctx.FocusIndex])
}

func TestLoadSourceContext_LineZeroClampedToStart(t *testing.T) {
	src := []string{"line1", "line2"}
	path := writeTempSource(t, src)
	ctx, err := LoadSourceContext(SourceRef{File: path, Line: 0}, 2)
	require.NoError(t, err)
	assert.Equal(t, 0, ctx.FocusIndex)
}

func TestLoadSourceContext_EmptyFile(t *testing.T) {
	path := writeTempSource(t, []string{})
	ctx, err := LoadSourceContext(SourceRef{File: path, Line: 1}, 5)
	require.NoError(t, err)
	assert.Empty(t, ctx.Lines)
}

func TestSplitPane_Render_NoSource(t *testing.T) {
	node := NewTraceNode("call-1", "contract_call")
	node.ContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	node.Function = "transfer"

	var buf bytes.Buffer
	pane := &SplitPane{Width: 80, TraceRows: 6, SrcRows: 4}
	pane.Render(&buf, node, nil)

	out := buf.String()
	assert.Contains(t, out, "Trace Node")
	assert.Contains(t, out, "transfer")
	assert.Contains(t, out, "No source mapping available")
}

func TestSplitPane_Render_WithSource(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("// rust line %d", i+1)
	}
	path := writeTempSource(t, lines)

	node := NewTraceNode("call-1", "contract_call")
	node.Function = "do_transfer"
	node.SourceRef = &SourceRef{File: path, Line: 10}

	ctx, err := LoadSourceContext(*node.SourceRef, 3)
	require.NoError(t, err)

	var buf bytes.Buffer
	pane := &SplitPane{Width: 80, TraceRows: 6, SrcRows: 8}
	pane.Render(&buf, node, ctx)

	out := buf.String()
	assert.Contains(t, out, "do_transfer")
	assert.Contains(t, out, "// rust line 10")
	assert.Contains(t, out, "10")
}

func TestSplitPane_Render_ErrorNode(t *testing.T) {
	node := NewTraceNode("err-1", "error")
	node.Error = "Insufficient balance"

	var buf bytes.Buffer
	pane := &SplitPane{Width: 60, TraceRows: 5, SrcRows: 4}
	pane.Render(&buf, node, nil)

	out := buf.String()
	assert.Contains(t, out, "Insufficient balance")
}

func TestSplitPane_Render_NodeWithSourceRef(t *testing.T) {
	node := NewTraceNode("call-2", "host_fn")
	node.Function = "require_auth"
	node.SourceRef = &SourceRef{File: "token.rs", Line: 45, Column: 12}

	var buf bytes.Buffer
	pane := &SplitPane{Width: 80, TraceRows: 8, SrcRows: 4}
	pane.Render(&buf, node, nil)

	out := buf.String()
	assert.Contains(t, out, "token.rs:45:12")
}

func TestHBorder_WidthConsistency(t *testing.T) {
	for _, width := range []int{20, 40, 80, 120} {
		border := hBorder("", width)
		assert.Equal(t, width, len(border), "border width mismatch for width=%d", width)
	}
}

func TestHBorder_LabelCentred(t *testing.T) {
	b := hBorder(" Trace Node ", 40)
	assert.Equal(t, 40, len(b))
	assert.Contains(t, b, " Trace Node ")
}

func TestHBorder_LabelTruncated(t *testing.T) {
	label := " a very long label that exceeds the width "
	b := hBorder(label, 10)
	assert.Equal(t, 10, len(b))
}

func TestNodeDisplayLines_AllFields(t *testing.T) {
	node := NewTraceNode("n1", "contract_call")
	node.ContractID = "CXYZ"
	node.Function = "swap"
	node.EventData = "some data"
	node.Error = "out of gas"
	node.SourceRef = &SourceRef{File: "pool.rs", Line: 88}

	lines := nodeDisplayLines(node)
	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "CONTRACT_CALL")
	assert.Contains(t, joined, "CXYZ")
	assert.Contains(t, joined, "swap")
	assert.Contains(t, joined, "some data")
	assert.Contains(t, joined, "out of gas")
	assert.Contains(t, joined, "pool.rs:88")
}

func TestNodeDisplayLines_MinimalNode(t *testing.T) {
	node := NewTraceNode("n2", "log")
	lines := nodeDisplayLines(node)
	assert.NotEmpty(t, lines)
	assert.Contains(t, lines[0], "LOG")
}

func TestExecutionStateToNode(t *testing.T) {
	state := &ExecutionState{
		Step:       5,
		Operation:  "InvokeHostFunction",
		ContractID: "CTEST",
		Function:   "mint",
		Error:      "",
	}
	node := executionStateToNode(state)
	assert.Equal(t, "step-5", node.ID)
	assert.Equal(t, "InvokeHostFunction", node.Type)
	assert.Equal(t, "CTEST", node.ContractID)
	assert.Equal(t, "mint", node.Function)
}

func TestExecutionStateToNode_ErrorPromotesType(t *testing.T) {
	state := &ExecutionState{
		Step:      2,
		Operation: "InvokeHostFunction",
		Error:     "wasm trap",
	}
	node := executionStateToNode(state)
	assert.Equal(t, "error", node.Type)
	assert.Equal(t, "wasm trap", node.Error)
}
