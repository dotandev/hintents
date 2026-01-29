// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// Version is set by main.go from build flags
var Version = "dev"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "erst",
	Short: "Erst - Soroban Error Decoder & Debugger",
	Long: `Erst is a specialized developer tool for the Stellar network,
designed to solve the "black box" debugging experience on Soroban.

It helps clarify why a Stellar smart contract transaction failed by:
  - Fetching failed transaction envelopes and ledger state
  - Re-executing transactions locally for detailed analysis
  - Mapping execution failures back to readable source code`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Register all subcommands using the modular registry pattern
	RegisterCommands(rootCmd)
}

// RegisterCommands registers all subcommands to the root command.
// This function is called during initialization and provides a central
// place to manage command registration, keeping root.go clean and focused.
func RegisterCommands(root *cobra.Command) {
	// Commands are registered in alphabetical order to maintain
	// consistent help output ordering

	// Register the debug command
	registerDebugCommand(root)

	// Register the version command
	registerVersionCommand(root)

	// Future commands can be registered here:
	// registerAnalyzeCommand(root)
	// registerReplayCommand(root)
	// registerSessionCommand(root)
	// registerTraceCommand(root)
}

// currentSession stores the active debugging session
var currentSession interface{}

// SetCurrentSession stores the current session data
func SetCurrentSession(session interface{}) {
	currentSession = session
}

// GetCurrentSession retrieves the current session data
func GetCurrentSession() interface{} {
	return currentSession
}