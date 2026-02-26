// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package simulator

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func prepareCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateCommand(cmd *exec.Cmd, graceTimeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	targetPID := cmd.Process.Pid
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		targetPID = -pgid
	}

	if err := syscall.Kill(targetPID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}

	deadline := time.Now().Add(graceTimeout)
	for time.Now().Before(deadline) {
		if processExited(cmd.Process) {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}

	if err := syscall.Kill(targetPID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func processExited(process *os.Process) bool {
	if process == nil {
		return true
	}
	err := process.Signal(syscall.Signal(0))
	return errors.Is(err, syscall.ESRCH)
}
