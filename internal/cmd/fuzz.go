// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/hex"
	"fmt"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	fuzzIterations     uint64
	fuzzSeed           uint64
	fuzzTimeout        uint64
	fuzzMaxSize        int
	fuzzInputXDR       string
	fuzzEnableCov      bool
	fuzzTargetContract string
)

var fuzzCmd = &cobra.Command{
	Use:     "fuzz",
	GroupID: "testing",
	Short:   "Fuzz test XDR inputs against Soroban contracts",
	Long: `Perform fuzzing of XDR inputs to discover edge cases and potential crashes
in Soroban contract execution.

This command uses coverage-guided fuzzing to generate random inputs that are
run through the contract simulator. It helps identify crashes, edge cases, and
other issues in contract code.

Fuzzing can be started with a base XDR input which will be mutated for
subsequent iterations, or fuzzing can be run on random inputs.

Examples:
  erst fuzz --iterations 10000
  erst fuzz --iterations 50000 --workers 8
  erst fuzz --xdr <hex-encoded-xdr> --iterations 5000`,
	RunE: runFuzz,
}

func runFuzz(cmd *cobra.Command, args []string) error {
	if fuzzIterations == 0 {
		return fmt.Errorf("--iterations must be specified and greater than 0")
	}

	fmt.Printf("Starting fuzzing campaign\n")
	fmt.Printf("  Iterations: %d\n", fuzzIterations)
	fmt.Printf("  Timeout: %dms\n", fuzzTimeout)
	fmt.Printf("  Max Input Size: %d bytes\n", fuzzMaxSize)

	if fuzzInputXDR != "" {
		fmt.Printf("  Base Input: %s...\n", fuzzInputXDR[:min(32, len(fuzzInputXDR))])
	}

	if fuzzTargetContract != "" {
		fmt.Printf("  Target Contract: %s\n", fuzzTargetContract)
	}

	// Initialize simulator runner
	runner, err := simulator.NewRunner("", false)
	if err != nil {
		return fmt.Errorf("failed to initialize simulator: %w", err)
	}

	// Create fuzzing configuration
	config := simulator.FuzzingConfig{
		MaxIterations:    fuzzIterations,
		TimeoutMs:        fuzzTimeout,
		MaxInputSize:     fuzzMaxSize,
		EnableCoverage:   fuzzEnableCov,
		TargetContractID: fuzzTargetContract,
	}

	// Create fuzzing harness
	harness := simulator.NewFuzzingHarness(runner, config)

	// If specific XDR is provided, validate and fuzz it
	if fuzzInputXDR != "" {
		// Validate it's valid hex
		if _, err := hex.DecodeString(fuzzInputXDR); err != nil {
			return fmt.Errorf("invalid XDR hex encoding: %w", err)
		}

		result, err := harness.FuzzXDR(fuzzInputXDR)
		if err != nil {
			return fmt.Errorf("fuzzing failed: %w", err)
		}

		fmt.Printf("\nFuzz Test Result:\n")
		fmt.Printf("  Status: %s\n", result.Status)
		if result.ErrorMessage != "" {
			fmt.Printf("  Error: %s\n", result.ErrorMessage)
		}
		fmt.Printf("  Execution Time: %dms\n", result.ExecutionTimeMs)
		fmt.Printf("  Code Coverage: %d%%\n", result.CodeCoverage)

		if result.Status == "crash" {
			return fmt.Errorf("fuzzing found a crash")
		}

		return nil
	}

	// Run normal fuzzing campaign without base input
	fmt.Println("\nNo base XDR provided - using random generation")
	fmt.Println("Starting fuzzing campaign...")

	// Create empty base input for fuzzing
	baseInput := &simulator.FuzzerInput{
		EnvelopeXdr:   "",
		LedgerEntries: make(map[string]string),
		Args:          []string{},
	}

	ctx := cmd.Context()

	// Note: This is a simplified version. Production fuzzing would:
	// 1. Use libFuzzer binaries directly
	// 2. Have proper corpus management
	// 3. Integrate with coverage feedback

	_ = ctx // Use context for potential future cancellation

	results, crashingInputs, err := harness.Fuzz(baseInput)
	if err != nil {
		return fmt.Errorf("fuzzing campaign failed: %w", err)
	}

	// Print summary
	fmt.Println("\n" + harness.Summary())

	// Print first few crashing inputs if found
	if len(crashingInputs) > 0 {
		fmt.Printf("\n%d unique crash(es) found!\n", len(crashingInputs))
		for i, input := range crashingInputs {
			if i < 5 {
				fmt.Printf("  Crash %d (seed %d): %s...\n",
					i+1,
					input.Seed,
					fuzzInputXDR[:min(20, len(fuzzInputXDR))],
				)
			}
		}
		if len(crashingInputs) > 5 {
			fmt.Printf("  ... and %d more crashes\n", len(crashingInputs)-5)
		}
		return fmt.Errorf("fuzzing found %d crashes", len(crashingInputs))
	}

	if len(results) > 0 {
		fmt.Printf("\nFuzzing completed: %d/%d tests passed\n",
			len(results),
			fuzzIterations,
		)
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	fuzzCmd.Flags().Uint64Var(
		&fuzzIterations,
		"iterations",
		0,
		"Number of fuzzing iterations (required)",
	)

	fuzzCmd.Flags().Uint64Var(
		&fuzzTimeout,
		"timeout",
		5000,
		"Timeout per fuzz iteration in milliseconds",
	)

	fuzzCmd.Flags().IntVar(
		&fuzzMaxSize,
		"max-size",
		262144,
		"Maximum input size in bytes (default 256KB)",
	)

	fuzzCmd.Flags().StringVar(
		&fuzzInputXDR,
		"xdr",
		"",
		"Optional base XDR input to fuzz (hex-encoded)",
	)

	fuzzCmd.Flags().BoolVar(
		&fuzzEnableCov,
		"coverage",
		false,
		"Enable code coverage tracking (requires instrumented binary)",
	)

	fuzzCmd.Flags().StringVar(
		&fuzzTargetContract,
		"target",
		"",
		"Optional target contract ID to focus fuzzing on",
	)

	rootCmd.AddCommand(fuzzCmd)
}
