// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package cli

import (
	"errors"
	"os/exec"
	"syscall"
)

// prepareBatchCommand isolates the simulator (and any descendants it spawns)
// in its own process group so that a timeout or cancellation can tear down
// the entire process tree instead of only the direct child.
func prepareBatchCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateBatchCommand force-kills the simulator's whole process group so
// that descendants which inherited the simulator's stdio cannot keep the
// batch runner blocked past the configured per-simulation timeout.
func terminateBatchCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	// Prefer the process group (negative PID) so every descendant forked by
	// the simulator dies with it. Fall back to the direct child if the group
	// lookup fails.
	targetPID := cmd.Process.Pid
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		targetPID = -pgid
	}

	if err := syscall.Kill(targetPID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
