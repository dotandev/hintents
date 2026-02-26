// Copyright 2025 Erst Users
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
