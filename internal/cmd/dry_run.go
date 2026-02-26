// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/xdr"
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
	Use:     "dry-run <tx.xdr>",
	GroupID: "testing",
	Short:   "Pre-submission dry run to estimate Soroban transaction cost",
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
		default:
			return errors.WrapInvalidNetwork(dryRunNetworkFlag)
		}
	},
	RunE: runDryRun,
}

func init() {
	dryRunCmd.Flags().StringVarP(&dryRunNetworkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	dryRunCmd.Flags().StringVar(&dryRunRPCURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")
	dryRunCmd.Flags().StringVar(&dryRunRPCTokenFlag, "rpc-token", "", "RPC authentication token (can also use ERST_RPC_TOKEN env var)")

	_ = dryRunCmd.RegisterFlagCompletionFunc("network", completeNetworkFlag)

	rootCmd.AddCommand(dryRunCmd)
}

func runDryRun(cmd *cobra.Command, args []string) error {
	path := args[0]
	b, err := os.ReadFile(path)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to read tx file: %v", err))
	}
	envXdrB64 := string(bytesTrimSpace(b))
	if envXdrB64 == "" {
		return errors.WrapValidationError("tx file is empty")
	}

	// Validate envelope is parseable
	envBytes, err := base64.StdEncoding.DecodeString(envXdrB64)
	if err != nil {
		return errors.WrapUnmarshalFailed(err, "envelope base64")
	}
	var envelope xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshal(envBytes, &envelope); err != nil {
		return errors.WrapUnmarshalFailed(err, "TransactionEnvelope")
	}

	// Create RPC client
	opts := []rpc.ClientOption{
		rpc.WithNetwork(rpc.Network(dryRunNetworkFlag)),
		rpc.WithToken(dryRunRPCTokenFlag),
	}
	if dryRunRPCURLFlag != "" {
		opts = append(opts, rpc.WithHorizonURL(dryRunRPCURLFlag))
	}

	client, err := rpc.NewClient(opts...)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to create client: %v", err))
	}
	registerCacheFlushHook()

	ctx := cmd.Context()

	if checkErr := client.CheckStaleness(ctx, dryRunNetworkFlag); checkErr != nil {
		// Optional: you can print this to stderr if you want to see why the check failed
		// fmt.Fprintf(os.Stderr, "Note: Could not verify node freshness: %v\n", checkErr)
	}

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
		return errors.WrapSimulationLogicError(fmt.Sprintf("failed to extract ledger keys from envelope: %v", err))
	}
	ledgerEntries, err := client.GetLedgerEntries(ctx, keys)
	if err != nil {
		return errors.WrapRPCConnectionFailed(err)
	}

	// Warn if the fetched ledger entries exceed the Soroban network size limit.
	// The network rejects transactions whose footprint exceeds 1 MiB, so there
	// is no point invoking the simulator — the tx will never land on-chain.
	simulator.WarnLedgerEntriesSizeToStderr(ledgerEntries)

	runner, err := simulator.NewRunner("", false)
	if err != nil {
		return errors.WrapSimulatorNotFound(err.Error())
	}
	registerRunnerCloseHook("dry-run-simulator-runner", runner)
	defer func() { _ = runner.Close() }()

	// The current Rust simulator requires a non-empty result_meta_xdr.
	// For dry-run we don't have it (tx not on-chain), so we use a placeholder.
	simReq := &simulator.SimulationRequest{
		EnvelopeXdr:   envXdrB64,
		ResultMetaXdr: "AAAAAQ==", // placeholder base64
		LedgerEntries: ledgerEntries,
	}

	gas, err := simulator.EstimateGas(runner, simReq)
	resp, err := runner.Run(ctx, simReq)
	if err != nil {
		return errors.WrapSimulationFailed(fmt.Errorf("gas estimation: %w", err), "")
	}

	fmt.Printf("Estimated required fee (stroops): %d\n", gas.EstimatedFeeLowerBound)
	fmt.Printf("Budget usage: CPU=%d, MEM=%d\n", gas.CPUCost, gas.MemoryCost)

	return nil
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