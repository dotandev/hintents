// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/dotandev/hintents/internal/cmd"
)

// Version is the current version of erst
// This should be set via ldflags during build: -ldflags "-X main.Version=v1.2.3"
var Version = "dev"

func main() {
	// Set version in cmd package
	cmd.Version = Version

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}