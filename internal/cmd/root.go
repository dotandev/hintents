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
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {}
