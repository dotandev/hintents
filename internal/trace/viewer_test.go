// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// captureOutput temporarily redirects stdout so that the supplied function's
// output can be captured and returned as a string.
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
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
	for _, want := range []string{"expand", "search", "jump", "toggle", "replay"} {
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

func TestInteractiveViewer_DisplayCurrentState_ShowsFetchingPlaceholder(t *testing.T) {
	trace := NewExecutionTrace("tx", 1)
	trace.AddState(ExecutionState{Operation: "init", Memory: map[string]interface{}{"nonce": 1}})
	trace.AddState(ExecutionState{Operation: "next"})
	if _, err := trace.JumpToStep(1); err != nil {
		t.Fatalf("JumpToStep failed: %v", err)
	}

	viewer := NewInteractiveViewer(trace)
	viewer.fetchDelay = 100 * time.Millisecond

	out := captureOutput(func() {
		viewer.displayCurrentState()
	})

	if !strings.Contains(out, "[ FETCHING STATE... ]") {
		t.Fatalf("expected loading placeholder, got: %s", out)
	}
}

func TestInteractiveViewer_DisplayCurrentState_ClearsPlaceholderAfterFetch(t *testing.T) {
	trace := NewExecutionTrace("tx", 1)
	trace.AddState(ExecutionState{Operation: "init", Memory: map[string]interface{}{"nonce": 1}})
	trace.AddState(ExecutionState{Operation: "next"})
	if _, err := trace.JumpToStep(1); err != nil {
		t.Fatalf("JumpToStep failed: %v", err)
	}

	viewer := NewInteractiveViewer(trace)
	viewer.fetchDelay = 20 * time.Millisecond

	_ = captureOutput(func() {
		viewer.displayCurrentState()
	})

	select {
	case fetched := <-viewer.fetchCh:
		viewer.handleFetchedState(fetched)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for fetched state")
	}

	out := captureOutput(func() {
		viewer.displayCurrentState()
	})

	if strings.Contains(out, "[ FETCHING STATE... ]") {
		t.Fatalf("expected placeholder to be cleared, got: %s", out)
	}
	if !strings.Contains(out, "Memory: 1 entries") {
		t.Fatalf("expected reconstructed memory summary, got: %s", out)
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

func TestInteractiveViewer_ReplayForkUpdatesTrace(t *testing.T) {
	trace := NewExecutionTrace("tx", 1)
	trace.AddState(ExecutionState{
		Operation: "init",
		HostState: map[string]interface{}{"seed": "orig"},
	})
	trace.AddState(ExecutionState{
		Operation: "step-one",
		HostState: map[string]interface{}{"counter": 1},
	})
	trace.AddState(ExecutionState{
		Operation: "step-two",
		HostState: map[string]interface{}{"counter": 2},
	})

	viewer := NewInteractiveViewer(trace)

	out := captureOutput(func() {
		exit := viewer.handleCommand("r seed=forked")
		if exit {
			t.Fatal("replay command should not signal exit")
		}
	})

	if !strings.Contains(out, "ROLLBACK_AND_RESUME") {
		t.Fatalf("replay output should mention rollback-and-resume command, got: %s", out)
	}

	if !viewer.forked {
		t.Fatal("expected viewer to mark forked trace")
	}

	reconstructed, err := trace.ReconstructStateAt(2)
	if err != nil {
		t.Fatalf("ReconstructStateAt failed: %v", err)
	}

	if reconstructed.HostState["seed"] != "forked" {
		t.Fatalf("expected fork override to persist, got %v", reconstructed.HostState["seed"])
	}
}
