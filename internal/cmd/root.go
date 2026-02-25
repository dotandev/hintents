// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/dotandev/hintents/internal/localization"
	"github.com/dotandev/hintents/internal/updater"
	"github.com/spf13/cobra"
)

// Global flag variables
var (
	TimestampFlag int64
	WindowFlag    int64
	ProfileFlag   bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "erst",
	Short: "Soroban smart contract debugger and transaction analyzer",
	Long: `Erst is a specialized developer tool for the Stellar network that helps you
debug failed Soroban transactions and analyze smart contract execution.

Key features:
  - Debug failed transactions with detailed error traces
  - Simulate transaction execution locally
  - Track token flows and contract events
  - Manage debugging sessions for complex workflows
  - Cache transaction data for offline analysis
  - Local WASM replay for rapid contract development

Examples:
  erst debug abc123...def                    Debug a transaction
  erst debug --network testnet abc123...def  Debug on testnet
  erst debug --wasm ./contract.wasm          Test contract locally
  erst session list                          View saved sessions
  erst cache status                          Check cache usage

Get started with 'erst debug --help' or visit the documentation.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load localizations
		if err := localization.LoadTranslations(); err != nil {
			return err
		}

		// Check for updates asynchronously (non-blocking)
		checkForUpdatesAsync()

		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

// checkForUpdatesAsync runs the update check in a goroutine to not block CLI startup
func checkForUpdatesAsync() {
	// Run update check in background goroutine
	go func() {
		// Use the Version variable from version.go
		checker := updater.NewChecker(Version)
		checker.CheckForUpdates()
	}()
}

func init() {
	// Root command initialization
	rootCmd.PersistentFlags().Int64Var(
		&TimestampFlag,
		"timestamp",
		0,
		"Override the ledger header timestamp (Unix epoch)",
	)

	rootCmd.PersistentFlags().Int64Var(
		&WindowFlag,
		"window",
		0,
		"Run range simulation across a time window (seconds)",
	)

	rootCmd.PersistentFlags().BoolVar(
		&ProfileFlag,
		"profile",
		false,
		"Enable CPU/Memory profiling and generate a flamegraph SVG",
	)

	// Register commands
}
