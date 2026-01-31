// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stellar/go/xdr"
	"github.com/spf13/cobra"
)

var (
	dryRunNetworkFlag  string
	dryRunRPCURLFlag   string
	dryRunRPCTokenFlag string
)

// dryRunCmd performs a pre-submission simulation of a locally provided transaction envelope XDR
// and prints a fee estimate derived from observed resource usage.
//
// NOTE: High-precision fee estimation ultimately depends on network fee configuration. This command
// provides a deterministic estimate based on the simulator's reported resource usage, intended as a
// safe lower bound / guidance for setting fee/budget.
var dryRunCmd = &cobra.Command{
	Use:   "dry-run <tx.xdr>",
	Short: "Pre-submission dry run to estimate Soroban transaction cost",
	Long: `Replay a local transaction envelope (not yet on chain) against current network state.

This command:
  1) Loads a base64-encoded TransactionEnvelope XDR from a local file
  2) Fetches required ledger entries from the configured Soroban RPC
  3) Replays the transaction locally via the Rust simulator
  4) Prints an estimated required fee based on the observed resource usage

Example:
  erst dry-run ./tx.xdr --network testnet`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate network flag
		switch rpc.Network(dryRunNetworkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			return nil
		default:
			return fmt.Errorf("invalid network: %s. Must be one of: testnet, mainnet, futurenet", dryRunNetworkFlag)
		}
	},
	RunE: runDryRun,
}

func init() {
	dryRunCmd.Flags().StringVarP(&dryRunNetworkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	dryRunCmd.Flags().StringVar(&dryRunRPCURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")
	dryRunCmd.Flags().StringVar(&dryRunRPCTokenFlag, "rpc-token", "", "RPC authentication token (can also use ERST_RPC_TOKEN env var)")

	rootCmd.AddCommand(dryRunCmd)
}

func runDryRun(cmd *cobra.Command, args []string) error {
	path := args[0]
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read tx file: %w", err)
	}
	envXdrB64 := string(bytesTrimSpace(b))
	if envXdrB64 == "" {
		return fmt.Errorf("tx file is empty")
	}

	// Validate envelope is parseable
	envBytes, err := base64.StdEncoding.DecodeString(envXdrB64)
	if err != nil {
		return fmt.Errorf("failed to decode envelope base64: %w", err)
	}
	var envelope xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshal(envBytes, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal TransactionEnvelope: %w", err)
	}

	// Create RPC client
	var client *rpc.Client
	if dryRunRPCURLFlag != "" {
		client = rpc.NewClientWithURL(dryRunRPCURLFlag, rpc.Network(dryRunNetworkFlag), dryRunRPCTokenFlag)
	} else {
		client = rpc.NewClient(rpc.Network(dryRunNetworkFlag), dryRunRPCTokenFlag)
	}

	ctx := cmd.Context()

	// Preferred path: Soroban RPC preflight (simulateTransaction)
	if preflight, err := client.SimulateTransaction(ctx, envXdrB64); err == nil {
		fee := preflight.Result.MinResourceFee
		cpu := preflight.Result.Cost.CpuInsns
		mem := preflight.Result.Cost.MemBytes
		if cpu == 0 {
			cpu = preflight.Result.Cost.CpuInsns_
		}
		if mem == 0 {
			mem = preflight.Result.Cost.MemBytes_
		}

		fmt.Printf("Min resource fee (stroops): %s\n", fee)
		if cpu != 0 || mem != 0 {
			fmt.Printf("Preflight cost: CPU=%d, MEM=%d\n", cpu, mem)
		}
		return nil
	}

	// Fallback: local simulator heuristic (best-effort)
	keys, err := extractLedgerKeysFromEnvelope(&envelope)
	if err != nil {
		return fmt.Errorf("failed to extract ledger keys from envelope: %w", err)
	}
	ledgerEntries, err := client.GetLedgerEntries(ctx, keys)
	if err != nil {
		return fmt.Errorf("failed to fetch ledger entries: %w", err)
	}

	runner, err := simulator.NewRunner("", false)
	if err != nil {
		return fmt.Errorf("failed to initialize simulator: %w", err)
	}

	// The current Rust simulator requires a non-empty result_meta_xdr.
	// For dry-run we don't have it (tx not on-chain), so we use a placeholder.
	simReq := &simulator.SimulationRequest{
		EnvelopeXdr:   envXdrB64,
		ResultMetaXdr: "AAAAAQ==", // placeholder base64
		LedgerEntries: ledgerEntries,
	}

	resp, err := runner.Run(simReq)
	if err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	if resp.BudgetUsage == nil {
		return fmt.Errorf("simulator did not return budget usage")
	}

	est, err := estimateFeeFromBudget(*resp.BudgetUsage)
	if err != nil {
		return err
	}

	fmt.Printf("Estimated required fee (stroops): %d\n", est)
	fmt.Printf("Budget usage: CPU=%d, MEM=%d\n", resp.BudgetUsage.CPUInstructions, resp.BudgetUsage.MemoryBytes)

	return nil
}

func estimateFeeFromBudget(b simulator.BudgetUsage) (int64, error) {
	// Conservative heuristic for now.
	// TODO: Replace with exact network pricing once fee config is exposed by public RPC.
	// Base fee + CPU + memory components.
	const base int64 = 100
	cpu := int64(b.CPUInstructions / 10000)      // 1 stroop per 10k insns
	mem := int64(b.MemoryBytes / (64 * 1024))    // 1 stroop per 64KiB
	if cpu < 0 || mem < 0 {
		return 0, fmt.Errorf("invalid budget usage")
	}
	return base + cpu + mem, nil
}

func bytesTrimSpace(b []byte) []byte {
	// Small local trim to avoid importing bytes.
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return b[start:end]
}

func extractLedgerKeysFromEnvelope(env *xdr.TransactionEnvelope) ([]string, error) {
	// Best-effort extraction: for Soroban invoke operations, footprint lives in SorobanTransactionDataExt
	// which is not always present / easy to reconstruct without full parsing.
	// For now we return an empty list, letting simulation rely on internal defaults (may reduce accuracy).
	//
	// TODO: Implement full footprint extraction via soroban tx data when available.
	_ = env
	return []string{}, nil
}
