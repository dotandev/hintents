// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dotandev/hintents/internal/profile"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/trace"
	"github.com/spf13/cobra"
)

var (
	profileTraceFile string
	profileOutput    string
	profileXdr       string
	profileNetwork   string
	profileRPCURL    string
	profileRPCToken  string
	profileOutJSON   string
)

var profileCmd = &cobra.Command{
	Use:     "profile [trace-file]",
	GroupID: "testing",
	Short:   "Advanced gas usage analysis and optimization advisor",
	Long: `Analyze gas consumption and receive actionable optimization recommendations.

This command can either:
  1) Load an existing execution trace file (pprof format by default)
  2) Run a new simulation from an XDR envelope and generate a comprehensive report

The report includes a breakdown of CPU/Memory usage, an efficiency score, and 
specific tips for reducing gas costs.

Example:
  erst profile execution.json -o gas.pb.gz
  erst profile --xdr <tx.xdr> --network testnet
  erst profile --xdr <tx.xdr> --out-json profile.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProfile,
}

func init() {
	profileCmd.Flags().StringVarP(&profileTraceFile, "file", "f", "", "Trace file to profile")
	profileCmd.Flags().StringVarP(&profileOutput, "output", "o", "profile.pb.gz", "Output path for the pprof profile")
	profileCmd.Flags().StringVar(&profileXdr, "xdr", "", "XDR encoded transaction envelope to profile")
	profileCmd.Flags().StringVarP(&profileNetwork, "network", "n", "mainnet", "Stellar network for simulation (testnet, mainnet, futurenet)")
	profileCmd.Flags().StringVar(&profileRPCURL, "rpc-url", "", "Custom Horizon RPC URL")
	profileCmd.Flags().StringVar(&profileRPCToken, "rpc-token", "", "RPC authentication token")
	profileCmd.Flags().StringVar(&profileOutJSON, "out-json", "", "Export the optimization report as JSON")

	rootCmd.AddCommand(profileCmd)
}

func runProfile(cmd *cobra.Command, args []string) error {
	if profileXdr != "" {
		return runSimulationProfile(cmd)
	}

	var filename string
	if len(args) > 0 {
		filename = args[0]
	} else if profileTraceFile != "" {
		filename = profileTraceFile
	} else {
		return fmt.Errorf("either --xdr or a trace file is required")
	}

	return runTraceProfile(filename)
}

func runSimulationProfile(cmd *cobra.Command) error {
	ctx := cmd.Context()

	// Handle XDR input (file or string)
	var xdrB64 string
	if _, err := os.Stat(profileXdr); err == nil {
		content, err := os.ReadFile(profileXdr)
		if err != nil {
			return fmt.Errorf("failed to read XDR file: %w", err)
		}
		xdrB64 = string(bytes.TrimSpace(content))
	} else {
		xdrB64 = profileXdr
	}

	// Create simulator runner
	runner, err := simulator.NewRunner("", false)
	if err != nil {
		return fmt.Errorf("failed to create simulator runner: %w", err)
	}
	defer runner.Close()

	// Build request
	builder := simulator.NewSimulationRequestBuilder().
		WithEnvelopeXDR(xdrB64).
		WithResultMetaXDR("AAAAAQ=="). // Placeholder
		WithOptimizationAdvisor(true)

	req, err := builder.Build()
	if err != nil {
		return err
	}

	fmt.Println("[DEPLOY] Running gas analysis simulation...")
	resp, err := runner.Run(ctx, req)
	if err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	if resp.OptimizationReport == nil {
		return fmt.Errorf("simulator did not return an optimization report")
	}

	// Display report
	displayOptimizationReport(resp.OptimizationReport, resp.BudgetUsage)

	// Export to JSON if requested
	if profileOutJSON != "" {
		data, err := json.MarshalIndent(resp.OptimizationReport, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal report to JSON: %w", err)
		}
		if err := os.WriteFile(profileOutJSON, data, 0644); err != nil {
			return fmt.Errorf("failed to write JSON report: %w", err)
		}
		fmt.Printf("\n[OK] Optimization report exported to: %s\n", profileOutJSON)
	}

	return nil
}

func runTraceProfile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read trace file: %w", err)
	}

	executionTrace, err := trace.FromJSON(data)
	if err != nil {
		return fmt.Errorf("failed to parse trace: %w", err)
	}

	fmt.Printf("Analyzing trace: %s\n", filename)
	fmt.Printf("Steps: %d\n", len(executionTrace.States))

	f, err := os.Create(profileOutput)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	if err := profile.WritePprof(executionTrace, f); err != nil {
		return fmt.Errorf("failed to generate pprof profile: %w", err)
	}

	fmt.Printf("[OK] Profile saved to: %s\n", profileOutput)
	fmt.Println("Use 'go tool pprof' to analyze the output.")
	return nil
}

func displayOptimizationReport(report *simulator.OptimizationReport, budget *simulator.BudgetUsage) {
	fmt.Printf("\n=== Gas optimization report ===\n")
	fmt.Printf("Overall Efficiency: %.1f%%\n", report.OverallEfficiency*100)
	fmt.Printf("Status: %s\n", report.ComparisonToBaseline)

	if budget != nil {
		fmt.Printf("\nResource Usage:\n")
		fmt.Printf("  CPU Instructions: %d (%.1f%% of limit)\n", budget.CPUInstructions, budget.CPUUsagePercent)
		fmt.Printf("  Memory Bytes:     %d (%.1f%% of limit)\n", budget.MemoryBytes, budget.MemoryUsagePercent)
		fmt.Printf("  Operations:       %d\n", budget.OperationsCount)
	}

	if len(report.BudgetBreakdown) > 0 {
		fmt.Printf("\nBudget Breakdown:\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "Category\tCost (Instructions)\tPercentage")
		var total float64
		for _, v := range report.BudgetBreakdown {
			total += v
		}
		for cat, cost := range report.BudgetBreakdown {
			pct := (cost / total) * 100
			fmt.Fprintf(w, "%s\t%.0f\t%.1f%%\n", cat, cost, pct)
		}
		w.Flush()
	}

	if len(report.Tips) > 0 {
		fmt.Printf("\n[INFO] Optimization Tips:\n")
		for _, tip := range report.Tips {
			severity := tip.Severity
			icon := "🟢 "
			switch severity {
			case "High":
				icon = "🔴 "
			case "Medium":
				icon = "🟡 "
			}
			fmt.Printf("\n[%s%s] %s: %s\n", icon, severity, tip.Category, tip.Message)
			if tip.EstimatedSavings != "" {
				fmt.Printf("   Estimated Savings: %s\n", tip.EstimatedSavings)
			}
			if tip.CodeLocation != nil {
				fmt.Printf("   Location: %s\n", *tip.CodeLocation)
			}
		}
	} else {
		fmt.Println("\n[OK] No specific optimizations identified. Your contract seems gas-efficient!")
	}
}
