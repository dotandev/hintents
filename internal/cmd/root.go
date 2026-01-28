package cmd

import (
	"github.com/spf13/cobra"
)

// Global flag variables
var (
	ProfileFlag bool
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
	rootCmd.PersistentFlags().BoolVar(
		&ProfileFlag,
		"profile",
		false,
		"Enable CPU/Memory profiling and generate a flamegraph SVG",
	)
}
