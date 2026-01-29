// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/session"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/tokenflow"
	"github.com/spf13/cobra"
)

// registerDebugCommand registers the debug command with the root command.
// This function is called from RegisterCommands in root.go.
func registerDebugCommand(root *cobra.Command) {
	root.AddCommand(newDebugCommand())
}

// newDebugCommand creates and returns the debug command.
// Keeping command creation separate from registration allows for
// better testing and modularity.
func newDebugCommand() *cobra.Command {
	var (
		networkFlag string
		rpcURLFlag  string
	)

	cmd := &cobra.Command{
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
			return runDebug(cmd, txHash, networkFlag, rpcURLFlag)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	cmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")

	return cmd
}

// runDebug executes the debug command logic.
// Separating the logic from command creation makes it easier to test.
func runDebug(cmd *cobra.Command, txHash, networkFlag, rpcURLFlag string) error {
	ctx := cmd.Context()

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

	// Fetch transaction details
	txResp, err := client.GetTransaction(ctx, txHash)
	if err != nil {
		return fmt.Errorf("failed to fetch transaction: %w", err)
	}

	fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(txResp.EnvelopeXdr))

	// Run simulation
	runner, err := simulator.NewRunner()
	if err != nil {
		return fmt.Errorf("failed to initialize simulator: %w", err)
	}

	// Build simulation request
	simReq := &simulator.SimulationRequest{
		EnvelopeXdr:   txResp.EnvelopeXdr,
		ResultMetaXdr: txResp.ResultMetaXdr,
		LedgerEntries: nil, // TODO: fetch ledger entries if needed
	}

	fmt.Printf("Running simulation...\n")
	simResp, err := runner.Run(simReq)
	if err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	// Display simulation results
	fmt.Printf("\nSimulation Results:\n")
	fmt.Printf("  Status: %s\n", simResp.Status)
	if simResp.Error != "" {
		fmt.Printf("  Error: %s\n", simResp.Error)
	}
	if len(simResp.Events) > 0 {
		fmt.Printf("  Events: %d\n", len(simResp.Events))
		for i, event := range simResp.Events {
			if i < 5 { // Show first 5 events
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
			if i < 5 { // Show first 5 logs
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
		ErstVersion:     Version,
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
}