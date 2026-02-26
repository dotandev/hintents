// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/dce"
	"github.com/spf13/cobra"
)

var dceOutput string

var dceCmd = &cobra.Command{
	Use:   "dce <wasm-file>",
	Short: "Eliminate dead code from a WASM binary",
	Long: `Analyze a compiled WASM binary, build a call graph from exported functions,
and strip unreachable functions to reduce contract size.

Without -o, performs a dry run and prints statistics only.

Examples:
  erst dce ./contract.wasm -o ./contract-optimized.wasm
  erst dce ./contract.wasm`,
	Args: cobra.ExactArgs(1),
	RunE: dceExec,
}

func dceExec(cmd *cobra.Command, args []string) error {
	wasmBytes, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading WASM file: %w", err)
	}

	out, stats, err := dce.Eliminate(wasmBytes)
	if err != nil {
		return err
	}

	fmt.Printf("Total functions:    %d\n", stats.TotalFunctions)
	fmt.Printf("Removed functions:  %d\n", stats.RemovedFunctions)
	fmt.Printf("Original size:      %d bytes\n", stats.OriginalSize)
	fmt.Printf("Optimized size:     %d bytes\n", stats.OptimizedSize)

	if stats.OriginalSize > 0 {
		saved := stats.OriginalSize - stats.OptimizedSize
		pct := float64(saved) / float64(stats.OriginalSize) * 100
		fmt.Printf("Saved:              %d bytes (%.1f%%)\n", saved, pct)
	}

	if dceOutput == "" {
		return nil
	}

	if err := os.WriteFile(dceOutput, out, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	fmt.Printf("Written to:         %s\n", dceOutput)

	return nil
}

func init() {
	dceCmd.Flags().StringVarP(&dceOutput, "output", "o", "", "Output file path (omit for dry run)")
	rootCmd.AddCommand(dceCmd)
}
