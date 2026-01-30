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
	"fmt"

	"github.com/dotandev/hintents/internal/authtrace"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/spf13/cobra"
)

var (
	authNetworkFlag    string
	authRPCURLFlag     string
	authDetailedFlag   bool
	authJSONOutputFlag bool
)

var authDebugCmd = &cobra.Command{
	Use:   "auth-debug <transaction-hash>",
	Short: "Debug multi-signature and threshold-based authorization failures",
	Long: `Analyze multi-signature authorization flows and identify which signatures or thresholds failed.

Examples:
  erst auth-debug <tx-hash>
  erst auth-debug --detailed <tx-hash>
  erst auth-debug --json <tx-hash>`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		switch rpc.Network(authNetworkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
		default:
			return fmt.Errorf("invalid network: %s", authNetworkFlag)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

		client := rpc.NewClient(rpc.Network(authNetworkFlag), "")
		if authRPCURLFlag != "" {
			client = rpc.NewClientWithURL(authRPCURLFlag, rpc.Network(authNetworkFlag), "")
		}

		logger.Logger.Info("Fetching transaction for auth analysis", "tx_hash", txHash)

		resp, err := client.GetTransaction(cmd.Context(), txHash)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		fmt.Printf("Transaction Envelope: %d bytes\n", len(resp.EnvelopeXdr))

		config := authtrace.AuthTraceConfig{
			TraceCustomContracts: true,
			CaptureSigDetails:    true,
			MaxEventDepth:        1000,
		}

		tracker := authtrace.NewTracker(config)
		trace := tracker.GenerateTrace()
		reporter := authtrace.NewDetailedReporter(trace)

		if authJSONOutputFlag {
			jsonStr, err := reporter.GenerateJSONString()
			if err != nil {
				return err
			}
			fmt.Println(jsonStr)
		} else {
			fmt.Println(reporter.GenerateReport())
			if authDetailedFlag {
				printDetailedAnalysis(reporter)
			}
		}

		return nil
	},
}

func printDetailedAnalysis(reporter *authtrace.DetailedReporter) {
	metrics := reporter.SummaryMetrics()
	fmt.Println("\n--- SUMMARY METRICS ---")
	for key, value := range metrics {
		fmt.Printf("%s: %v\n", key, value)
	}

	missingKeys := reporter.IdentifyMissingKeys()
	if len(missingKeys) > 0 {
		fmt.Println("\n--- MISSING SIGNATURES ---")
		for _, signer := range missingKeys {
			fmt.Printf("  - %s (required weight: %d)\n", signer.SignerKey, signer.Weight)
		}
	}
}

func init() {
	authDebugCmd.Flags().StringVarP(&authNetworkFlag, "network", "n", string(rpc.Mainnet), "Stellar network (testnet, mainnet, futurenet)")
	authDebugCmd.Flags().StringVar(&authRPCURLFlag, "rpc-url", "", "Custom Horizon RPC URL")
	authDebugCmd.Flags().BoolVar(&authDetailedFlag, "detailed", false, "Show detailed analysis and missing signatures")
	authDebugCmd.Flags().BoolVar(&authJSONOutputFlag, "json", false, "Output as JSON")
	rootCmd.AddCommand(authDebugCmd)
}
