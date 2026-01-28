package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version will be set by the main package
	Version = "dev"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of erst",
	Long:  `Display the current version of the erst CLI tool.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("erst version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
