// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/cmd"
)

var Version = "dev"

func main() {
	// Set version in cmd package (used for upgrade banner and async version check)
	cmd.Version = Version

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
