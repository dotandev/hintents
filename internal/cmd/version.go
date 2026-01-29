// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// registerVersionCommand registers the version command with the root command.
// This function is called from RegisterCommands in root.go.
func registerVersionCommand(root *cobra.Command) {
	root.AddCommand(newVersionCommand())
}

// newVersionCommand creates and returns the version command.
func newVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of erst",
		Long:  `Display the current version of the erst CLI tool.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("erst version %s\n", Version)
		},
	}

	return cmd
}