// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	// "github.com/dotandev/hintents/internal/cmd"
	"github.com/dotandev/hintents/internal/cmd"
	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/crashreport"
	"github.com/dotandev/hintents/internal/version"
)

// Build-time variables injected via -ldflags.
var (
	buildVersion   = "dev"
	buildCommitSHA = "unknown"
)

func init() {
	// Set version from build-time variables
	version.Version = buildVersion
	version.CommitSHA = buildCommitSHA
}

// ─── Example RPC handler ──────────────────────────────────────────────────────

/*
func rpcHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"result":  "0xdeadbeef",
		"id":      1,
	})
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}
*/

func main() {
	ctx := context.Background()

	// Load config to determine whether crash reporting is opted in.
	cfg, err := config.LoadConfig()
	if err != nil {
		// Non-fatal: fall back to a reporter that is disabled by default.
		cfg = config.DefaultConfig()
	}

	reporter := crashreport.New(crashreport.Config{
		Enabled:   cfg.CrashReporting,
		SentryDSN: cfg.CrashSentryDSN,
		Endpoint:  cfg.CrashEndpoint,
		Version:   buildVersion,
		CommitSHA: buildCommitSHA,
	})

	// Catch any unrecovered panic, report it, then re-panic.
	defer reporter.HandlePanic(ctx, "erst")

	execute := func() error {
		execErr := cmd.Execute()
		if execErr != nil && reporter.IsEnabled() && !cmd.IsInterrupted(execErr) {
			// Report fatal command errors that were not recovered as panics.
			stack := debug.Stack()
			_ = reporter.Send(ctx, execErr, stack, "erst")
		}
		return execErr
	}

	os.Exit(run(execute, os.Stderr))
}

func run(execute func() error, stderr io.Writer) int {
	if err := execute(); err != nil {
		if cmd.IsInterrupted(err) {
			_, _ = fmt.Fprintln(stderr, "Interrupted. Shutting down...")
			return cmd.InterruptExitCode
		}
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
