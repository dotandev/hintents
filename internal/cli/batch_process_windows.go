// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package cli

import (
	"os/exec"
)

// prepareBatchCommand is a no-op on Windows: there are no POSIX process
// groups, so terminateBatchCommand kills the direct child only.
func prepareBatchCommand(cmd *exec.Cmd) {
	_ = cmd
}

// terminateBatchCommand stops the simulator process. On Windows only the
// direct child can be targeted, mirroring internal/simulator behavior.
func terminateBatchCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
