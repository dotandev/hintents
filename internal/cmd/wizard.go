// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/wizard"
	"github.com/spf13/cobra"
)

var wizardCmd = &cobra.Command{
	Use:     "wizard",
	GroupID: "development",
	Short:   "Interactive transaction selection wizard",
	Long:    "Find and select recent failed transactions for debugging.",
	RunE: func(cmd *cobra.Command, args []string) error {
		account, _ := cmd.Flags().GetString("account")
		network, _ := cmd.Flags().GetString("network")

		if account == "" {
			return errors.WrapCliArgumentRequired("account")
		}

		client, err := rpc.NewClient(rpc.WithNetwork(rpc.Network(network)))
		if err != nil {
			return errors.WrapValidationError(err.Error())
		}

		w := wizard.New(client)
		result, err := w.SelectTransaction(cmd.Context(), account)
		if err != nil {
			return err
		}

		fmt.Printf("\nSelected: %s\nStatus: %s\nCreated: %s\n\nRun: erst debug %s\n",
			result.Hash, result.Status, result.CreatedAt, result.Hash)
		return nil
	},
}

func init() {
	wizardCmd.Flags().StringP("account", "a", "", "Stellar account address")
	wizardCmd.Flags().StringP("network", "n", string(rpc.Mainnet), "Network (testnet, mainnet, futurenet)")
	rootCmd.AddCommand(wizardCmd)
}
