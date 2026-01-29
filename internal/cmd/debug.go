// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/session"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/tokenflow"
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
		ctx := cmd.Context()
		txHash := args[0]

		var client *rpc.Client
		var horizonURL string
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
			horizonURL = rpcURLFlag
		} else {
			client = rpc.NewClient(rpc.Network(networkFlag))
			// Get default Horizon URL for the network
			switch rpc.Network(networkFlag) {
			case rpc.Testnet:
				horizonURL = rpc.TestnetHorizonURL
			case rpc.Futurenet:
				horizonURL = rpc.FuturenetHorizonURL
			default:
				horizonURL = rpc.MainnetHorizonURL
			}
		}

		fmt.Printf("Debugging transaction: %s\n", txHash)
		fmt.Printf("Network: %s\n", networkFlag)
		if rpcURLFlag != "" {
			fmt.Printf("RPC URL: %s\n", rpcURLFlag)
		}

		txResp, err := client.GetTransaction(ctx, txHash)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(txResp.EnvelopeXdr))

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
			EnvelopeXdr:   txResp.EnvelopeXdr,
			ResultMetaXdr: txResp.ResultMetaXdr,
			LedgerEntries: ledgerEntries,
		}

		fmt.Printf("Running simulation...\n")
		simResp, err := runner.Run(simReq)
		if err != nil {
			return fmt.Errorf("simulation failed: %w", err)
		}

		fmt.Printf("\nSimulation Results:\n")
		fmt.Printf("  Status: %s\n", simResp.Status)
		if simResp.Error != "" {
			fmt.Printf("  Error: %s\n", simResp.Error)
		}
		if len(simResp.Events) > 0 {
			fmt.Printf("  Events: %d\n", len(simResp.Events))
			for i, event := range simResp.Events {
				if i < 5 {
					fmt.Printf("    - %s\n", event)
				}
			}
			if len(simResp.Events) > 5 {
				fmt.Printf("    ... and %d more\n", len(simResp.Events)-5)
			}
		}
		if len(simResp.Logs) > 0 {
			fmt.Printf("  Logs: %d\n", len(simResp.Logs))
			for i, log := range simResp.Logs {
				if i < 5 {
					fmt.Printf("    - %s\n", log)
				}
			}
			if len(simResp.Logs) > 5 {
				fmt.Printf("    ... and %d more\n", len(simResp.Logs)-5)
			}
		}

		// Serialize simulation request/response for session storage
		simReqJSON, err := json.Marshal(simReq)
		if err != nil {
			return fmt.Errorf("failed to marshal simulation request: %w", err)
		}
		simRespJSON, err := json.Marshal(simResp)
		if err != nil {
			return fmt.Errorf("failed to marshal simulation response: %w", err)
		}

		// Create session data
		sessionData := &session.SessionData{
			ID:              session.GenerateID(txHash),
			CreatedAt:       time.Now(),
			LastAccessAt:    time.Now(),
			Status:          "active",
			Network:         networkFlag,
			HorizonURL:      horizonURL,
			TxHash:          txHash,
			EnvelopeXdr:     txResp.EnvelopeXdr,
			ResultXdr:       txResp.ResultXdr,
			ResultMetaXdr:   txResp.ResultMetaXdr,
			SimRequestJSON:  string(simReqJSON),
			SimResponseJSON: string(simRespJSON),
			ErstVersion:     getErstVersion(),
			SchemaVersion:   session.SchemaVersion,
		}

		// Token flow summary (native XLM + Soroban SAC via diagnostic events in ResultMetaXdr)
		if report, err := tokenflow.BuildReport(txResp.EnvelopeXdr, txResp.ResultMetaXdr); err != nil {
			fmt.Printf("\nToken Flow Summary: (failed to parse: %v)\n", err)
		} else if len(report.Agg) == 0 {
			fmt.Printf("\nToken Flow Summary: no transfers/mints detected\n")
		} else {
			fmt.Printf("\nToken Flow Summary:\n")
			for _, line := range report.SummaryLines() {
				fmt.Printf("  %s\n", line)
			}
			fmt.Printf("\nToken Flow Chart (Mermaid):\n")
			fmt.Println(report.MermaidFlowchart())
		}

		// Store as current session for potential saving
		SetCurrentSession(sessionData)

		fmt.Printf("\nSession created: %s\n", sessionData.ID)
		fmt.Printf("Run 'erst session save' to persist this session.\n")

		return nil
	},
}

// getErstVersion returns a version string for the current build
func getErstVersion() string {
	// In a real build, this would come from build flags or version.go
	// For now, return a placeholder
	return "dev"
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
