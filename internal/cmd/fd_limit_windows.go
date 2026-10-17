// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package cmd

// getSystemFDLimit on Windows returns 0 and nil because Windows does not enforce Unix-style rlimits.
func getSystemFDLimit() (uint64, error) {
	return 0, nil
}
