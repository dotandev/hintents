// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"fmt"
	"os"


	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/visualizer"
	"github.com/spf13/cobra"
)

var (
	htmlReportFlag bool
)

var analyzeWasmCmd = &cobra.Command{
	Use:   "analyze-wasm <path-to-wasm>",
	Short: "Analyze WASM binary size and section breakdown",
	Long: `Read a WASM binary and provide a detailed breakdown of its sections,
categorizing them into Logic, Debug Info, Data, and Other.

This is useful for understanding how much of your contract size is dedicated
to debug information versus actual executable logic.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wasmPath := args[0]

		// Read WASM file
		wasmBytes, err := os.ReadFile(wasmPath)
		if err != nil {
			return fmt.Errorf("failed to read WASM file: %w", err)
		}

		wasmBase64 := base64.StdEncoding.EncodeToString(wasmBytes)

		// Initialize simulator runner
		runner, err := simulator.NewRunner("", false)
		if err != nil {
			return fmt.Errorf("failed to initialize simulator: %w", err)
		}

		// Create request
		req := &simulator.SimulationRequest{
			ContractWasm: &wasmBase64,
			AnalyzeOnly:   true,
		}

		// Run analysis
		fmt.Printf("Analyzing WASM: %s (%d bytes)\n", wasmPath, len(wasmBytes))
		resp, err := runner.Run(req)
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}

		if resp.WasmAnalysis == nil {
			return fmt.Errorf("no analysis data returned from simulator")
		}

		// Print summary
		printAnalysisSummary(resp.WasmAnalysis)

		if htmlReportFlag {
			report := visualizer.GenerateWasmReport(resp.WasmAnalysis)
			outputPath := "wasm_analysis_report.html"
			if err := os.WriteFile(outputPath, []byte(report), 0644); err != nil {
				return fmt.Errorf("failed to save HTML report: %w", err)
			}
			fmt.Printf("\nHTML visualizer report generated: %s\n", outputPath)
		}

		return nil
	},
}

func printAnalysisSummary(analysis *simulator.WasmAnalysis) {
	fmt.Printf("\nWASM Binary Breakdown (Total: %d bytes):\n", analysis.TotalSize)
	fmt.Println("--------------------------------------------------")

	categorySizes := make(map[string]uint64)
	for _, section := range analysis.Sections {
		categorySizes[section.Category] += section.Size
	}

	for cat, size := range categorySizes {
		pct := (float64(size) / float64(analysis.TotalSize)) * 100.0
		fmt.Printf("%-15s %10d bytes (%6.2f%%)\n", cat+":", size, pct)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Printf("%-15s %10d bytes (100.00%%)\n", "Total:", analysis.TotalSize)
}

func init() {
	rootCmd.AddCommand(analyzeWasmCmd)
	analyzeWasmCmd.Flags().BoolVar(&htmlReportFlag, "html", false, "Generate an HTML visualizer report")
}
