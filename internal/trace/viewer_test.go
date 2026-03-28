// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureOutput temporarily redirects stdout so that the supplied function's
// output can be captured and returned as a string.
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	return buf.String()
}

func TestInteractiveViewer_HelpContents(t *testing.T) {
	// simple trace just to create viewer
	trace := NewExecutionTrace("tx", 1)
	viewer := NewInteractiveViewer(trace)

	out := captureOutput(func() {
		viewer.showHelp()
	})

	if !strings.Contains(out, "Keyboard Shortcuts") {
		t.Errorf("help output missing header: %s", out)
	}
	// check for required keywords referenced by issue
	for _, want := range []string{"expand", "search", "jump", "toggle"} {
		if !strings.Contains(strings.ToLower(out), want) {
			t.Errorf("help output should mention %q, got: %s", want, out)
		}
	}
}

func TestInteractiveViewer_HandleCommand_HelpAlias(t *testing.T) {
	trace := NewExecutionTrace("tx", 1)
	viewer := NewInteractiveViewer(trace)

	out := captureOutput(func() {
		exit := viewer.handleCommand("?")
		if exit {
			t.Error("help command should not signal exit")
		}
	})

	if !strings.Contains(out, "Keyboard Shortcuts") {
		t.Errorf("help alias '?' did not display help overlay: %s", out)
	}
}

func TestInteractiveViewer_StatusBarLineFormat(t *testing.T) {
	trace := NewExecutionTrace("tx", 1)
	trace.AddState(ExecutionState{
		Operation:    "require_auth",
		Arguments:    []interface{}{"A", 1},
		HostState:    map[string]interface{}{"status": "ok"},
		Memory:       map[string]interface{}{"nonce": 42, "counter": 7},
		RawArguments: []string{"AAAAAQ=="},
	})

	viewer := NewInteractiveViewer(trace)
	state, err := trace.GetCurrentState()
	if err != nil {
		t.Fatalf("GetCurrentState failed: %v", err)
	}

	line := viewer.statusBarLine(state)

	for _, want := range []string{"Step 1/1", "Payload:", "kb", "Memory:", "mb", "Snapshot ID:", "snap-000@0"} {
		if !strings.Contains(line, want) {
			t.Fatalf("status bar line missing %q: %s", want, line)
		}
	}
}

func TestInteractiveViewer_SnapshotIDForStep(t *testing.T) {
	trace := NewExecutionTrace("tx", 2)
	for i := 0; i < 5; i++ {
		trace.AddState(ExecutionState{Operation: "op", Memory: map[string]interface{}{"i": i}})
	}

	viewer := NewInteractiveViewer(trace)

	if got := viewer.snapshotIDForStep(0); got != "snap-000@0" {
		t.Fatalf("snapshotIDForStep(0) = %q, want snap-000@0", got)
	}
	if got := viewer.snapshotIDForStep(3); got != "snap-001@2" {
		t.Fatalf("snapshotIDForStep(3) = %q, want snap-001@2", got)
	}
}
