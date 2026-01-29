// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/dotandev/hintents/internal/localization"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "erst",
	Short: "Erst - Soroban Error Decoder & Debugger",
	Long: `Erst is a specialized developer tool for the Stellar network,
designed to solve the "black box" debugging experience on Soroban.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return localization.LoadTranslations()
	},
// Global flag variables
var (
	ProfileFlag bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "erst",
	Short: "Soroban smart contract debugger and transaction analyzer",
	Long: `Erst is a specialized developer tool for the Stellar network that helps you
debug failed Soroban transactions and analyze smart contract execution.

Key features:
  • Debug failed transactions with detailed error traces
  • Simulate transaction execution locally
  • Track token flows and contract events
  • Manage debugging sessions for complex workflows
  • Cache transaction data for offline analysis

Examples:
  erst debug abc123...def                    Debug a transaction
  erst debug --network testnet abc123...def  Debug on testnet
  erst session list                          View saved sessions
  erst cache status                          Check cache usage

Get started with 'erst debug --help' or visit the documentation.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {}
func init() {
	// Root command initialization
	rootCmd.PersistentFlags().BoolVar(
		&ProfileFlag,
		"profile",
		false,
		"Enable CPU/Memory profiling and generate a flamegraph SVG",
	)
}
