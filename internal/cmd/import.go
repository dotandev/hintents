// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"time"

	"github.com/dotandev/hintents/internal/cli"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/spf13/cobra"
)

var (
	importContractIDsFlag []string
	importLedgerKeysFlag  []string
	importOutputFlag      string
	importNetworkFlag     string
	importRPCURLFlag      string
	importRPCTokenFlag    string
	importTimeoutFlag     time.Duration
)

// importCmd fetches contract state from a Soroban RPC endpoint and writes it
// into a local snapshot the simulator can load.
var importCmd = &cobra.Command{
	Use:     "import --contract <id> [--contract <id>...] -o snapshot.json",
	GroupID: "testing",
	Short:   "Import remote network state into a local simulator snapshot",
	Long: `Fetch contract data via Soroban RPC and load it into a local snapshot file,
so simulations can run against mainnet/testnet state.

For each --contract the contract instance entry and its referenced WASM code
entry are fetched, producing a self-contained snapshot for local replay.
Additional raw base64 XDR ledger keys can be imported with --key.
If the output file already exists, fetched entries are merged into it.

Examples:
  erst import --contract CABC... -o snapshot.json --network mainnet
  erst import --contract CABC... --contract CDEF... -o snapshot.json
  erst import --key AAAA... --key BBBB... -o snapshot.json --network testnet`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if importOutputFlag == "" {
			return errors.WrapCliArgumentRequired("--output")
		}
		if len(importContractIDsFlag) == 0 && len(importLedgerKeysFlag) == 0 {
			return fmt.Errorf("at least one --contract or --key is required")
		}
		switch rpc.Network(importNetworkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			return nil
		default:
			return errors.WrapInvalidNetwork(importNetworkFlag)
		}
	},
	RunE: runImport,
}

func init() {
	importCmd.Flags().StringArrayVar(&importContractIDsFlag, "contract", nil, "Contract ID (C... strkey or 32-byte hex); repeatable")
	importCmd.Flags().StringArrayVar(&importLedgerKeysFlag, "key", nil, "Raw base64 XDR LedgerKey to fetch; repeatable")
	importCmd.Flags().StringVarP(&importOutputFlag, "output", "o", "", "Output snapshot JSON path (required)")
	importCmd.Flags().StringVarP(&importNetworkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to fetch from (testnet, mainnet, futurenet)")
	importCmd.Flags().StringVar(&importRPCURLFlag, "rpc-url", "", "Custom Soroban RPC URL overriding the network default")
	importCmd.Flags().StringVar(&importRPCTokenFlag, "rpc-token", "", "RPC authentication token (can also use ERST_RPC_TOKEN env var)")
	importCmd.Flags().DurationVar(&importTimeoutFlag, "timeout", 60*time.Second, "Overall import timeout")

	_ = importCmd.RegisterFlagCompletionFunc("network", completeNetworkFlag)

	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, _ []string) error {
	cfg := cli.ImportConfig{
		ContractIDs: importContractIDsFlag,
		LedgerKeys:  importLedgerKeysFlag,
		OutputPath:  importOutputFlag,
		Network:     importNetworkFlag,
		RPCURL:      importRPCURLFlag,
		Token:       importRPCTokenFlag,
		Timeout:     importTimeoutFlag,
	}

	fmt.Printf("Importing network state (%s) → %s\n", cfg.Network, cfg.OutputPath)
	result, err := cli.ImportNetworkState(cmd.Context(), cfg)
	if err != nil {
		return err
	}

	fmt.Printf("Contracts imported: %d\n", len(result.Contracts))
	fmt.Printf("Ledger entries: %d (%d newly fetched)\n", result.Entries, result.FetchedKeys)
	fmt.Printf("Fingerprint: %s\n", result.Fingerprint)
	fmt.Printf("Snapshot written: %s\n", result.OutputPath)
	return nil
}
