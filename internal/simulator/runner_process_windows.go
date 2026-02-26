// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package simulator

import (
	"os/exec"
	"time"
)

func prepareCommand(cmd *exec.Cmd) {
	_ = cmd
}

func terminateCommand(cmd *exec.Cmd, graceTimeout time.Duration) error {
	_ = graceTimeout
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
