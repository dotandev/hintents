package cmd

import (
	"github.com/spf13/cobra"
)

// Global flag variables
var (
	TimestampFlag int64
	WindowFlag    int64
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "erst",
	Short: "Erst - Soroban Error Decoder & Debugger",
	Long: `Erst is a specialized developer tool for the Stellar network,
designed to solve the "black box" debugging experience on Soroban.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
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
}
