// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/cmd"
	"github.com/dotandev/hintents/internal/decoder"
	"github.com/dotandev/hintents/internal/updater"
)

// Version is the current version of erst
// This should be set via ldflags during build: -ldflags "-X main.Version=v1.2.3"
var Version = "dev"

func main() {
	// Set version in cmd package
	if len(os.Args) < 2 {
		fmt.Println("Usage: txdecode <base64-envelope>")
		os.Exit(1)
	}
	cmd.Version = Version

	env, err := decoder.AnalyzeEnvelope(os.Args[1])
	if err != nil {
		panic(err)
	}

	decoder.PrintEnvelope(env)
	// Start update checker in background (non-blocking)
	checker := updater.NewChecker(Version)
	go checker.CheckForUpdates()

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
