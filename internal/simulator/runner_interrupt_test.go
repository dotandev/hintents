// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package simulator

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRunnerRun_ContextCancelTerminatesProcess(t *testing.T) {
	simPath := filepath.Join(t.TempDir(), "fake-erst-sim.sh")
	script := "#!/bin/sh\ntrap '' TERM\nsleep 30 &\nchild=$!\nwait $child\n"
	if err := os.WriteFile(simPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	runner := &Runner{
		BinaryPath: simPath,
		activeCmds: make(map[*exec.Cmd]struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := runner.Run(ctx, &SimulationRequest{
			EnvelopeXdr:   "x",
			ResultMetaXdr: "y",
		})
		done <- err
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not stop after context cancel")
	}

	if err := runner.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}
