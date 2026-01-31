package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "erst",
	Short: "Erst - Stellar Soroban error decoder and transaction replay tool",
	Long: `Erst is a specialized developer tool for the Stellar network, designed to solve 
the "black box" debugging experience on Soroban.

It helps developers understand why a Stellar smart contract transaction failed by:
  • Fetching failed transaction data from the network
  • Replaying transactions locally with full diagnostic output
  • Decoding cryptic XDR errors into readable messages`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags can be added here
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.erst.yaml)")
}
