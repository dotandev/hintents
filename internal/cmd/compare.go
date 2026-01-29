// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/compare"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	compareWasmFlag string
)

var compareCmd = &cobra.Command{
	Use:   "compare <transaction-hash>",
	Short: "Compare on-chain vs local WASM execution",
	Long: `Replay a transaction against both on-chain WASM and a local WASM file,
then show side-by-side differences in events, logs, and execution results.

This is essential for "What broke when I updated?" debugging.

Example:
  erst compare <tx-hash> --wasm ./target/wasm32-unknown-unknown/release/my_contract.wasm
  erst compare --network testnet <tx-hash> --wasm ./contract.wasm`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if compareWasmFlag == "" {
			return fmt.Errorf("--wasm flag is required")
		}
		if _, err := os.Stat(compareWasmFlag); err != nil {
			return fmt.Errorf("WASM file not found: %s: %w", compareWasmFlag, err)
		}
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
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
		} else {
			client = rpc.NewClient(rpc.Network(networkFlag))
		}

		fmt.Printf("Comparing execution for transaction: %s\n", txHash)
		fmt.Printf("Network: %s\n", networkFlag)
		fmt.Printf("Local WASM: %s\n\n", compareWasmFlag)

		// Fetch transaction details
		txResp, err := client.GetTransaction(ctx, txHash)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		// Initialize simulator
		runner, err := simulator.NewRunner()
		if err != nil {
			return fmt.Errorf("failed to initialize simulator: %w", err)
		}

		// Run on-chain simulation (normal flow)
		fmt.Printf("Running on-chain simulation...\n")
		onChainReq := &simulator.SimulationRequest{
			EnvelopeXdr:   txResp.EnvelopeXdr,
			ResultMetaXdr: txResp.ResultMetaXdr,
			LedgerEntries: nil,
		}
		onChainResp, err := runner.Run(onChainReq)
		if err != nil {
			return fmt.Errorf("on-chain simulation failed: %w", err)
		}

		// Read local WASM file
		wasmBytes, err := os.ReadFile(compareWasmFlag)
		if err != nil {
			return fmt.Errorf("failed to read WASM file: %w", err)
		}
		wasmBase64 := base64.StdEncoding.EncodeToString(wasmBytes)

		// Run local WASM simulation
		// NOTE: For MVP, we inject WASM into the request.
		// The Rust simulator needs to be updated to actually use this field.
		fmt.Printf("Running local WASM simulation...\n")
		localReq := &simulator.SimulationRequest{
			EnvelopeXdr:   txResp.EnvelopeXdr,
			ResultMetaXdr: txResp.ResultMetaXdr,
			LedgerEntries: nil,
			// TODO: Add WasmOverride field to SimulationRequest schema
			// and update Rust simulator to use it when loading contracts
		}
		// For now, we'll run the same simulation and note the limitation
		// In a full implementation, we'd inject the WASM into ledger entries
		// or pass it as a separate field that the simulator uses.
		localResp, err := runner.Run(localReq)
		if err != nil {
			return fmt.Errorf("local WASM simulation failed: %w", err)
		}

		// Compare results
		diff := compare.CompareResults(onChainResp, localResp)

		fmt.Printf("\n=== Comparison Results ===\n")
		fmt.Print(diff.FormatSideBySide())

		// Show WASM info
		fmt.Printf("\n=== WASM Info ===\n")
		fmt.Printf("Local WASM size: %d bytes (base64: %d chars)\n", len(wasmBytes), len(wasmBase64))
		fmt.Printf("\nNote: Full WASM injection requires Rust simulator updates.\n")
		fmt.Printf("Current implementation shows diff structure; actual WASM override\n")
		fmt.Printf("needs to be implemented in simulator/src/main.rs to replace\n")
		fmt.Printf("contract code from ledger entries with the provided WASM file.\n")

		return nil
	},
}

func init() {
	compareCmd.Flags().StringVar(&compareWasmFlag, "wasm", "", "Path to local WASM file to compare against on-chain version (required)")
	compareCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	compareCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")

	rootCmd.AddCommand(compareCmd)
}
