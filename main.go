// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/dotandev/hintents/internal/cmd"
	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/crashreport"
)

// Build-time variables injected via -ldflags.
var (
	version   = "dev"
	commitSHA = "unknown"
)

// ─── Example RPC handler ──────────────────────────────────────────────────────

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
		Version:   version,
		CommitSHA: commitSHA,
	})

	// Catch any unrecovered panic, report it, then re-panic.
	defer reporter.HandlePanic(ctx, "erst")

	if execErr := cmd.Execute(); execErr != nil {
		// Report fatal command errors that were not recovered as panics.
		if reporter.IsEnabled() {
			stack := debug.Stack()
			_ = reporter.Send(ctx, execErr, stack, "erst")
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", execErr)
		os.Exit(1)
	}
}
