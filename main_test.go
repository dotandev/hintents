// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dotandev/hintents/internal/cmd"
)

func TestRun_Interrupted(t *testing.T) {
	var stderr bytes.Buffer
	code := run(func() error { return cmd.ErrInterrupted }, &stderr)
	if code != cmd.InterruptExitCode {
		t.Fatalf("expected %d, got %d", cmd.InterruptExitCode, code)
	}
	if got := stderr.String(); got != "Interrupted. Shutting down...\n" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func TestRun_GenericError(t *testing.T) {
	var stderr bytes.Buffer
	code := run(func() error { return errors.New("boom") }, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if got := stderr.String(); got != "Error: boom\n" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func TestRun_Success(t *testing.T) {
	var stderr bytes.Buffer
	code := run(func() error { return nil }, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}
