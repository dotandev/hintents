// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package cmd

import (
	"syscall"
)

// getSystemFDLimit retrieves the current soft file descriptor limit using syscall.Getrlimit.
func getSystemFDLimit() (uint64, error) {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return 0, err
	}
	return uint64(rLimit.Cur), nil
}
