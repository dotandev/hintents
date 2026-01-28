// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/db"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/localization"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	networkFlag string
	rpcURLFlag  string
)

var debugCmd = &cobra.Command{
	Use:   "debug <transaction-hash>",
	Short: localization.Get("cli.debug.short"),
	Long:  localization.Get("cli.debug.long"),
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		switch rpc.Network(networkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			return nil
		default:
			return errors.WrapInvalidNetwork(networkFlag)
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

		client := rpc.NewClient(rpc.Network(networkFlag))
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
		}

		resp, err := client.GetTransaction(cmd.Context(), txHash)
		if err != nil {
			return fmt.Errorf(localization.Get("error.fetch_transaction"), err)
		}

		fmt.Printf(localization.Get("output.transaction_envelope")+"\n", len(resp.EnvelopeXdr))

		// Initialize Simulator
		runner, err := simulator.NewRunner()
		if err != nil {
			return fmt.Errorf("failed to initialize simulator: %w", err)
		}

		// Run Simulation
		simReq := &simulator.SimulationRequest{
			EnvelopeXdr:   resp.EnvelopeXdr,
			ResultMetaXdr: resp.ResultMetaXdr,
		}

		simResp, err := runner.Run(simReq)
		if err != nil {
			return fmt.Errorf("simulation failed: %w", err)
		}

		// Save to DB
		store, err := db.InitDB()
		if err != nil {
			fmt.Printf("Warning: failed to initialize session history DB: %v\n", err)
		} else {
			session := &db.Session{
				TxHash:   txHash,
				Network:  networkFlag,
				Status:   simResp.Status,
				ErrorMsg: simResp.Error,
				Events:   simResp.Events,
				Logs:     simResp.Logs,
			}
			if err := store.SaveSession(session); err != nil {
				fmt.Printf("Warning: failed to save session to history: %v\n", err)
			} else {
				fmt.Println("Session saved to history.")
			}
		}

		fmt.Printf("Simulation Status: %s\n", simResp.Status)
		if simResp.Error != "" {
			fmt.Printf("Error: %s\n", simResp.Error)
		}
		if len(simResp.Events) > 0 {
			fmt.Println("Diagnostic Events:")
			for _, e := range simResp.Events {
				fmt.Printf(" - %s\n", e)
			}
		}

		return nil
	},
}

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), localization.Get("cli.debug.flag.network"))
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", localization.Get("cli.debug.flag.rpc_url"))

	rootCmd.AddCommand(debugCmd)
}
