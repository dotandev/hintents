// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	regressionTestCount       int
	regressionProtocolVersion uint32
	regressionStartSeq        uint32
	regressionMaxWorkers      int
)

var regressionTestCmd = &cobra.Command{
	Use:     "regression-test",
	GroupID: "testing",
	Short:   "Run protocol regression tests against historic transactions",
	Long: `Execute a comprehensive regression test suite by downloading historic failed
transactions from Mainnet and ensuring erst-sim yields identical results.

This command fetches up to the specified number of failed transactions and
simulates them in parallel, verifying that the simulator produces the same
traps and events as the original network execution.

The tests help ensure that protocol changes don't introduce regressions.

Example:
  erst regression-test --count 100
  erst regression-test --count 1000 --workers 8
  erst regression-test --count 500 --network mainnet --protocol-version 22`,
	RunE: runRegressionTest,
}

func runRegressionTest(cmd *cobra.Command, args []string) error {
	if regressionTestCount <= 0 {
		return fmt.Errorf("--count must be greater than 0")
	}

	if regressionMaxWorkers <= 0 {
		regressionMaxWorkers = 4
	}

	fmt.Printf("Starting regression test suite\n")
	fmt.Printf("  Target count: %d transactions\n", regressionTestCount)
	fmt.Printf("  Network: %s\n", networkFlag)
	fmt.Printf("  Workers: %d\n", regressionMaxWorkers)

	if regressionProtocolVersion > 0 {
		// Validate protocol version
		if err := simulator.Validate(regressionProtocolVersion); err != nil {
			return fmt.Errorf("invalid protocol version: %w", err)
		}
		fmt.Printf("  Protocol version override: %d\n", regressionProtocolVersion)
	}

	// Create RPC client
	opts := []rpc.ClientOption{
		rpc.WithNetwork(rpc.Network(networkFlag)),
		rpc.WithToken(rpcTokenFlag),
	}
	if rpcURLFlag != "" {
		opts = append(opts, rpc.WithHorizonURL(rpcURLFlag))
	}

	client, err := rpc.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("failed to create RPC client: %w", err)
	}

	// Create simulator runner
	runner, err := simulator.NewRunner("", false)
	if err != nil {
		return fmt.Errorf("failed to initialize simulator: %w", err)
	}

	// Create regression harness
	harness := simulator.NewRegressionHarness(runner, client, regressionMaxWorkers)
	harness.Verbose = verbose

	// Run the regression tests
	ctx := cmd.Context()

	var protVersion *uint32
	if regressionProtocolVersion > 0 {
		protVersion = &regressionProtocolVersion
	}

	suite, err := harness.RunRegressionTests(ctx, regressionTestCount, protVersion, regressionStartSeq)
	if err != nil {
		return fmt.Errorf("regression tests failed: %w", err)
	}

	// Print summary
	fmt.Println("\n" + suite.Summary())

	// Print failed results if any
	failed := suite.FailedResults()
	if len(failed) > 0 {
		fmt.Printf("\n%d test(s) failed:\n", len(failed))
		for i, result := range failed {
			if i < 10 { // Show first 10 failures
				fmt.Printf("  [%d] %s: %s\n", i+1, result.TransactionHash, result.ErrorMessage)
			}
		}
		if len(failed) > 10 {
			fmt.Printf("  ... and %d more failures\n", len(failed)-10)
		}
		return fmt.Errorf("regression test failed with %d failures", len(failed))
	}

	fmt.Println("\nAll regression tests passed!")
	return nil
}

func init() {
	regressionTestCmd.Flags().IntVar(
		&regressionTestCount,
		"count",
		100,
		"Number of historic failed transactions to test (max 1000)",
	)

	regressionTestCmd.Flags().Uint32Var(
		&regressionStartSeq,
		"start-seq",
		0,
		"Starting ledger sequence number for fetching transactions",
	)

	regressionTestCmd.Flags().IntVar(
		&regressionMaxWorkers,
		"workers",
		4,
		"Number of parallel workers for testing",
	)

	regressionTestCmd.Flags().Uint32Var(
		&regressionProtocolVersion,
		"protocol-version",
		0,
		"Optional protocol version override for all tests",
	)

	regressionTestCmd.Flags().StringVarP(
		&networkFlag,
		"network",
		"n",
		string(rpc.Mainnet),
		"Stellar network to fetch transactions from (mainnet, testnet, futurenet)",
	)

	regressionTestCmd.Flags().StringVar(
		&rpcURLFlag,
		"rpc-url",
		"",
		"Custom RPC URL",
	)

	regressionTestCmd.Flags().StringVar(
		&rpcTokenFlag,
		"rpc-token",
		"",
		"RPC authentication token",
	)

	regressionTestCmd.Flags().BoolVarP(
		&verbose,
		"verbose",
		"v",
		false,
		"Enable verbose output",
	)

	rootCmd.AddCommand(regressionTestCmd)
}
