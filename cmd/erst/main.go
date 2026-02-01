// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/dotandev/hintents/internal/cmd"
	"github.com/dotandev/hintents/internal/updater"
)

var Version = "dev"

func main() {
	// Set version in cmd package
	cmd.Version = Version

	// Start update checker in background (non-blocking)
	checker := updater.NewChecker(Version)
	go checker.CheckForUpdates()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
