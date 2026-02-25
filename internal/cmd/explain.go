// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/heuristic"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	explainNetworkFlag string
	explainRPCURLFlag  string
	explainRPCToken    string
)

var explainCmd = &cobra.Command{
	Use:   "explain [transaction-hash]",
	Short: "Summarize why a transaction failed in plain English",
	Long: `Apply heuristic analysis to a transaction and output a single-paragraph
explanation of the root cause of the failure.

If a transaction hash is provided the command fetches and simulates it.
When run immediately after 'erst debug', the active session is used instead.

Examples:
  erst explain 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
  erst explain --network testnet <tx-hash>
  erst debug <tx-hash> && erst explain`,
	Args: cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		if err := rpc.ValidateTransactionHash(args[0]); err != nil {
			return fmt.Errorf("invalid transaction hash: %w", err)
		}
		switch rpc.Network(explainNetworkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
		default:
			return fmt.Errorf("invalid network: %s; must be testnet, mainnet, or futurenet", explainNetworkFlag)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return explainFromSession()
		}
		return explainFromNetwork(cmd, args[0])
	},
}

func explainFromSession() error {
	sess := GetCurrentSession()
	if sess == nil {
		return fmt.Errorf("no active session; run 'erst debug <tx-hash>' first or provide a transaction hash")
	}

	var simResp simulator.SimulationResponse
	if sess.SimResponseJSON != "" {
		if err := json.Unmarshal([]byte(sess.SimResponseJSON), &simResp); err != nil {
			return fmt.Errorf("failed to parse session simulation data: %w", err)
		}
	}

	in := heuristic.Input{
		TxHash:           sess.TxHash,
		Network:          sess.Network,
		Status:           simResp.Status,
		Error:            simResp.Error,
		Events:           simResp.Events,
		Logs:             simResp.Logs,
		DiagnosticEvents: simResp.DiagnosticEvents,
		BudgetUsage:      simResp.BudgetUsage,
	}
	fmt.Println(heuristic.Summarize(in))
	return nil
}

func explainFromNetwork(cmd *cobra.Command, txHash string) error {
	token := explainRPCToken
	if token == "" {
		token = os.Getenv("ERST_RPC_TOKEN")
	}
	if token == "" {
		if cfg, err := config.LoadConfig(); err == nil && cfg.RPCToken != "" {
			token = cfg.RPCToken
		}
	}

	opts := []rpc.ClientOption{
		rpc.WithNetwork(rpc.Network(explainNetworkFlag)),
		rpc.WithToken(token),
	}
	if explainRPCURLFlag != "" {
		opts = append(opts, rpc.WithHorizonURL(explainRPCURLFlag))
	}

	client, err := rpc.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.GetTransaction(cmd.Context(), txHash)
	if err != nil {
		return fmt.Errorf("failed to fetch transaction: %w", err)
	}

	keys, err := extractLedgerKeys(resp.ResultMetaXdr)
	if err != nil {
		keys = nil
	}

	ledgerEntries, err := rpc.ExtractLedgerEntriesFromMeta(resp.ResultMetaXdr)
	if err != nil {
		ledgerEntries, err = client.GetLedgerEntries(cmd.Context(), keys)
		if err != nil {
			ledgerEntries = nil
		}
	}

	runner, err := simulator.NewRunner("", false)
	if err != nil {
		return fmt.Errorf("failed to initialize simulator: %w", err)
	}

	simResp, err := runner.Run(&simulator.SimulationRequest{
		EnvelopeXdr:   resp.EnvelopeXdr,
		ResultMetaXdr: resp.ResultMetaXdr,
		LedgerEntries: ledgerEntries,
	})
	if err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	in := heuristic.Input{
		TxHash:           txHash,
		Network:          explainNetworkFlag,
		Status:           simResp.Status,
		Error:            simResp.Error,
		Events:           simResp.Events,
		Logs:             simResp.Logs,
		DiagnosticEvents: simResp.DiagnosticEvents,
		BudgetUsage:      simResp.BudgetUsage,
	}
	fmt.Println(heuristic.Summarize(in))
	return nil
}

func init() {
	explainCmd.Flags().StringVarP(&explainNetworkFlag, "network", "n", "mainnet", "Stellar network (testnet, mainnet, futurenet)")
	explainCmd.Flags().StringVar(&explainRPCURLFlag, "rpc-url", "", "Custom RPC URL")
	explainCmd.Flags().StringVar(&explainRPCToken, "rpc-token", "", "RPC authentication token (can also use ERST_RPC_TOKEN env var)")
	rootCmd.AddCommand(explainCmd)
}
