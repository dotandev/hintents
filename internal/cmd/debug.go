// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/db"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	networkFlag      string
	rpcURLFlag       string
	overrideStateFlag string
)

type OverrideData struct {
	LedgerEntries map[string]string `json:"ledger_entries"`
}

var debugCmd = &cobra.Command{
	Use:   "debug <transaction-hash>",
	Short: "Debug a failed Soroban transaction",
	Long: `Fetch a transaction envelope from the Stellar network and prepare it for simulation.

Example:
  erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
  erst debug --network testnet <tx-hash>`,
	Args: cobra.ExactArgs(1),
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

		var client *rpc.Client
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
		} else {
			client = rpc.NewClient(rpc.Network(networkFlag))
		}

		fmt.Printf("Debugging transaction: %s\n", txHash)
		fmt.Printf("Network: %s\n", networkFlag)
		if rpcURLFlag != "" {
			fmt.Printf("RPC URL: %s\n", rpcURLFlag)
		}

		resp, err := client.GetTransaction(cmd.Context(), txHash)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(resp.EnvelopeXdr))

		var ledgerEntries map[string]string
		if overrideStateFlag != "" {
			overrideEntries, err := loadOverrideState(overrideStateFlag)
			if err != nil {
				return fmt.Errorf("failed to load override state: %w", err)
			}

			ledgerEntries = overrideEntries
			logger.Logger.Info("Sandbox mode active", "entries_overridden", len(overrideEntries))
			fmt.Printf("Sandbox mode active: %d entries overridden\n", len(overrideEntries))
		}

		runner, err := simulator.NewRunner()
		if err != nil {
			return fmt.Errorf("failed to initialize simulator: %w", err)
		}

		simReq := &simulator.SimulationRequest{
			EnvelopeXdr:   resp.EnvelopeXdr,
			ResultMetaXdr: resp.ResultMetaXdr,
			LedgerEntries: ledgerEntries,
		}

		simResp, err := runner.Run(simReq)
		if err != nil {
			return fmt.Errorf("simulation failed: %w", err)
		}

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
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")
	debugCmd.Flags().StringVar(&overrideStateFlag, "override-state", "", "Path to JSON file with manual ledger entry overrides for sandbox mode")

	rootCmd.AddCommand(debugCmd)
}

func loadOverrideState(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read override file: %w", err)
	}

	var override OverrideData
	if err := json.Unmarshal(data, &override); err != nil {
		return nil, fmt.Errorf("failed to parse override JSON: %w", err)
	}

	if override.LedgerEntries == nil {
		return make(map[string]string), nil
	}

	return override.LedgerEntries, nil
}
