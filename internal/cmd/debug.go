// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
	"github.com/stellar/go/xdr"
)

var (
	networkFlag        string
	rpcURLFlag         string
	compareNetworkFlag string
)

var debugCmd = &cobra.Command{
	Use:   "debug <transaction-hash>",
	Short: "Debug a failed Soroban transaction",
	Long: `Fetch a transaction envelope from the Stellar network and prepare it for simulation.

Example:
  erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
  erst debug --network testnet <tx-hash>
  erst debug --network mainnet --compare-network testnet <tx-hash>`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate network flag
		switch rpc.Network(networkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			// valid
		default:
			return fmt.Errorf("invalid network: %s. Must be one of: testnet, mainnet, futurenet", networkFlag)
		}

		// Validate compare network flag if present
		if compareNetworkFlag != "" {
			switch rpc.Network(compareNetworkFlag) {
			case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
				// valid
			default:
				return fmt.Errorf("invalid compare-network: %s. Must be one of: testnet, mainnet, futurenet", compareNetworkFlag)
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

		// 1. Setup Primary Client
		var client *rpc.Client
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
		} else {
			client = rpc.NewClient(rpc.Network(networkFlag))
		}

		fmt.Printf("Debugging transaction: %s\n", txHash)
		fmt.Printf("Primary Network: %s\n", networkFlag)
		if compareNetworkFlag != "" {
			fmt.Printf("Comparing against Network: %s\n", compareNetworkFlag)
		}

		// 2. Fetch transaction details from Primary Network
		resp, err := client.GetTransaction(cmd.Context(), txHash)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}
		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(resp.EnvelopeXdr))

		// 3. Extract Ledger Keys from ResultMeta
		keys, err := extractLedgerKeys(resp.ResultMetaXdr)
		if err != nil {
			return fmt.Errorf("failed to extract ledger keys: %w", err)
		}
		logger.Logger.Info("Extracted ledger keys", "count", len(keys))

		// 4. Initialize Simulator Runner
		runner, err := simulator.NewRunner()
		if err != nil {
			return fmt.Errorf("failed to initialize simulator runner: %w", err)
		}

		// 5. Run Simulations
		if compareNetworkFlag == "" {
			// Single Run
			// Fetch Ledger Entries
			primaryEntries, err := client.GetLedgerEntries(cmd.Context(), keys)
			if err != nil {
				return fmt.Errorf("failed to fetch primary ledger entries: %w", err)
			}
			fmt.Printf("Fetched %d ledger entries from %s\n", len(primaryEntries), networkFlag)

			fmt.Printf("Running simulation on %s...\n", networkFlag)
			primaryReq := &simulator.SimulationRequest{
				EnvelopeXdr:   resp.EnvelopeXdr,
				ResultMetaXdr: resp.ResultMetaXdr,
				LedgerEntries: primaryEntries,
			}
			primaryResult, err := runner.Run(primaryReq)
			if err != nil {
				return fmt.Errorf("simulation failed on primary network: %w", err)
			}
			printSimulationResult(networkFlag, primaryResult)

		} else {
			// Parallel Execution
			var wg sync.WaitGroup
			var primaryResult, compareResult *simulator.SimulationResponse
			var primaryErr, compareErr error

			wg.Add(2)

			// Primary Network Routine
			go func() {
				defer wg.Done()
				
				// Fetch entries
				primaryEntries, err := client.GetLedgerEntries(cmd.Context(), keys)
				if err != nil {
					primaryErr = fmt.Errorf("failed to fetch primary ledger entries: %w", err)
					return
				}
				fmt.Printf("Fetched %d ledger entries from %s\n", len(primaryEntries), networkFlag)

				// Run Simulation
				fmt.Printf("Running simulation on %s...\n", networkFlag)
				primaryReq := &simulator.SimulationRequest{
					EnvelopeXdr:   resp.EnvelopeXdr,
					ResultMetaXdr: resp.ResultMetaXdr,
					LedgerEntries: primaryEntries,
				}
				primaryResult, primaryErr = runner.Run(primaryReq)
			}()

			// Compare Network Routine
			go func() {
				defer wg.Done()
				
				compareClient := rpc.NewClient(rpc.Network(compareNetworkFlag))
				
				// Fetch entries
				compareEntries, err := compareClient.GetLedgerEntries(cmd.Context(), keys)
				if err != nil {
					compareErr = fmt.Errorf("failed to fetch ledger entries from %s: %w", compareNetworkFlag, err)
					return
				}
				fmt.Printf("Fetched %d ledger entries from %s\n", len(compareEntries), compareNetworkFlag)

				// Run Simulation
				fmt.Printf("Running simulation on %s...\n", compareNetworkFlag)
				compareReq := &simulator.SimulationRequest{
					EnvelopeXdr:   resp.EnvelopeXdr,
					ResultMetaXdr: resp.ResultMetaXdr,
					LedgerEntries: compareEntries,
				}
				compareResult, compareErr = runner.Run(compareReq)
			}()

			wg.Wait()

			if primaryErr != nil {
				return fmt.Errorf("error on primary network: %w", primaryErr)
			}
			if compareErr != nil {
				return fmt.Errorf("error on compare network: %w", compareErr)
			}

			// Print and Diff
			printSimulationResult(networkFlag, primaryResult)
			printSimulationResult(compareNetworkFlag, compareResult)
			diffResults(primaryResult, compareResult, networkFlag, compareNetworkFlag)
		}

		return nil
	},
}

func extractLedgerKeys(metaXdr string) ([]string, error) {
	// Decode Base64
	data, err := base64.StdEncoding.DecodeString(metaXdr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	// Unmarshal XDR
	var meta xdr.TransactionResultMeta
	if err := xdr.SafeUnmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("xdr unmarshal failed: %w", err)
	}

	keysMap := make(map[string]struct{})

	// Helper to add key
	addKey := func(k xdr.LedgerKey) error {
		keyBytes, err := k.MarshalBinary()
		if err != nil {
			return err
		}
		keyB64 := base64.StdEncoding.EncodeToString(keyBytes)
		keysMap[keyB64] = struct{}{}
		return nil
	}

	// Iterate over changes
	var changes []xdr.LedgerEntryChange

	// Helper to collect changes from different versions
	collectChanges := func(l xdr.LedgerEntryChanges) {
		changes = append(changes, l...)
	}

	switch meta.V {
	case 0:
		collectChanges(meta.Operations)
	case 1:
		collectChanges(meta.V1.TxApplyProcessing.FeeProcessing)
		collectChanges(meta.V1.TxApplyProcessing.TxApplyProcessing)
	case 2:
		collectChanges(meta.V2.TxApplyProcessing.FeeProcessing)
		collectChanges(meta.V2.TxApplyProcessing.TxApplyProcessing)
	case 3:
		collectChanges(meta.V3.TxApplyProcessing.FeeProcessing)
		collectChanges(meta.V3.TxApplyProcessing.TxApplyProcessing)
	}

	for _, change := range changes {
		switch change.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			if err := addKey(change.Created.LedgerKey()); err != nil {
				return nil, err
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			if err := addKey(change.Updated.LedgerKey()); err != nil {
				return nil, err
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			if err := addKey(change.Removed); err != nil {
				return nil, err
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryState:
			if err := addKey(change.State.LedgerKey()); err != nil {
				return nil, err
			}
		}
	}

	result := make([]string, 0, len(keysMap))
	for k := range keysMap {
		result = append(result, k)
	}
	return result, nil
}

func printSimulationResult(network string, res *simulator.SimulationResponse) {
	fmt.Printf("\n--- Result for %s ---\n", network)
	fmt.Printf("Status: %s\n", res.Status)
	if res.Error != "" {
		fmt.Printf("Error: %s\n", res.Error)
	}
	fmt.Printf("Events: %d\n", len(res.Events))
	for i, ev := range res.Events {
		fmt.Printf("  [%d] %s\n", i, ev)
	}
}

func diffResults(res1, res2 *simulator.SimulationResponse, net1, net2 string) {
	fmt.Printf("\n=== Comparison: %s vs %s ===\n", net1, net2)
	
	if res1.Status != res2.Status {
		fmt.Printf("Status Mismatch: %s (%s) vs %s (%s)\n", res1.Status, net1, res2.Status, net2)
	} else {
		fmt.Printf("Status Match: %s\n", res1.Status)
	}

	// Compare Events
	fmt.Println("\nEvent Diff:")
	maxEvents := len(res1.Events)
	if len(res2.Events) > maxEvents {
		maxEvents = len(res2.Events)
	}

	for i := 0; i < maxEvents; i++ {
		var ev1, ev2 string
		if i < len(res1.Events) {
			ev1 = res1.Events[i]
		} else {
			ev1 = "<missing>"
		}
		
		if i < len(res2.Events) {
			ev2 = res2.Events[i]
		} else {
			ev2 = "<missing>"
		}

		if ev1 != ev2 {
			fmt.Printf("  [%d] MISMATCH:\n", i)
			fmt.Printf("    %s: %s\n", net1, ev1)
			fmt.Printf("    %s: %s\n", net2, ev2)
		} else {
			// Optional: Print matches if verbose
		}
	}
}

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")
	debugCmd.Flags().StringVar(&compareNetworkFlag, "compare-network", "", "Network to compare against (testnet, mainnet, futurenet)")

	rootCmd.AddCommand(debugCmd)
}
