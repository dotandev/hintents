// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/spf13/cobra"
)

var (
	searchLimitFlag      int
	searchNetworkFlag    string
	searchHorizonURLFlag string
	searchMaxPagesFlag   int
)

var searchCmd = &cobra.Command{
	Use:     "search <query>",
	GroupID: "management",
	Short:   "Find contracts by symbol, creator, or contract ID",
	Long: `Search contracts on Horizon/Soroban-backed networks using one query string.

The query is matched against:
  - contract symbol (when available from Horizon metadata)
  - creator/sponsor account address
  - partial contract ID

Search walks Horizon contract pages with a bounded scan (see --max-pages). Results can be
incomplete on large networks; a warning is printed when the scan stops before the catalog ends.

For a full Stellar account strkey (56 characters starting with G), Horizon sponsor= filtering is used when supported.`,
	Example: `  # Find token contracts by symbol
  erst search usdc --network testnet

  # Find contracts by creator account
  erst search GABCDEF... --network testnet

  # Find contracts by partial contract ID
  erst search CAXYZ --network testnet

  # Increase page walk limit (default scales with ERST_CONTRACT_SEARCH_MAX_PAGES)
  erst search usdc --network testnet --max-pages 50`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		network := strings.TrimSpace(searchNetworkFlag)
		switch rpc.Network(network) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
		default:
			return errors.WrapInvalidNetwork(network)
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		horizonURL := strings.TrimSpace(searchHorizonURLFlag)
		if horizonURL == "" {
			switch rpc.Network(network) {
			case rpc.Mainnet:
				horizonURL = rpc.MainnetHorizonURL
			case rpc.Futurenet:
				horizonURL = rpc.FuturenetHorizonURL
			default:
				horizonURL = rpc.TestnetHorizonURL
			}
		}

		maxPages := searchMaxPagesFlag
		if maxPages == 0 {
			if v := strings.TrimSpace(os.Getenv("ERST_CONTRACT_SEARCH_MAX_PAGES")); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					maxPages = n
				}
			}
		}

		res, err := rpc.SearchContracts(cmd.Context(), rpc.SearchContractsOptions{
			Query:      args[0],
			HorizonURL: horizonURL,
			Limit:      searchLimitFlag,
			Timeout:    time.Duration(cfg.RequestTimeout) * time.Second,
			MaxPages:   maxPages,
		})
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(res.Results) == 0 {
			fmt.Println("No matching contracts found.")
			if res.IncompleteScan {
				printContractSearchIncompleteWarning(res)
			}
			return nil
		}

		fmt.Printf("Found %d matching contracts on %s:\n", len(res.Results), network)
		for _, contract := range res.Results {
			fmt.Println("--------------------------------------------------")
			fmt.Printf("Contract ID: %s\n", contract.ID)
			if contract.Symbol != "" {
				fmt.Printf("Symbol: %s\n", contract.Symbol)
			}
			if contract.Creator != "" {
				fmt.Printf("Creator: %s\n", contract.Creator)
			}
			if contract.LastModifiedLedger > 0 {
				fmt.Printf("Latest Activity Ledger: %d\n", contract.LastModifiedLedger)
			}
			if contract.LastModifiedTime != "" {
				fmt.Printf("Latest Activity Time: %s\n", contract.LastModifiedTime)
			}
		}
		fmt.Println("--------------------------------------------------")

		if res.IncompleteScan {
			printContractSearchIncompleteWarning(res)
		}
		return nil
	},
}

func printContractSearchIncompleteWarning(res *rpc.SearchContractsResult) {
	fmt.Fprintf(os.Stderr, "Warning: contract search stopped after scanning %d contract row(s) from Horizon; additional matches may exist on the network (scan budget ≤ %d rows). Increase --max-pages or set ERST_CONTRACT_SEARCH_MAX_PAGES.\n",
		res.ScannedRecords, res.MaxScanBudget)
}

func init() {
	searchCmd.Flags().IntVar(&searchLimitFlag, "limit", 10, "Maximum number of results to return")
	searchCmd.Flags().StringVarP(&searchNetworkFlag, "network", "n", string(rpc.Testnet), "Stellar network to search (testnet, mainnet, futurenet)")
	searchCmd.Flags().IntVar(&searchMaxPagesFlag, "max-pages", 0, "Maximum Horizon pages to scan (0 = default or ERST_CONTRACT_SEARCH_MAX_PAGES)")
	searchCmd.Flags().StringVar(&searchHorizonURLFlag, "horizon-url", "", "Override Horizon URL (advanced)")
	_ = searchCmd.Flags().MarkHidden("horizon-url")
	_ = searchCmd.RegisterFlagCompletionFunc("network", completeNetworkFlag)

	rootCmd.AddCommand(searchCmd)
}
