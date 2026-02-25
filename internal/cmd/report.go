// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/report"
	"github.com/dotandev/hintents/internal/trace"
	"github.com/spf13/cobra"
)

var (
	reportFormat string
	reportOutput string
	reportFile   string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate debugging reports from traces",
	Long: `Generate professional PDF or HTML reports from execution traces.

Reports include:
  - Executive summary with key findings
  - Detailed execution steps and call stacks
  - Contract interaction analytics
  - Risk assessment with detected issues
  - Timeline and event distribution

Examples:
  erst report --file trace.json --format html --output reports/
  erst report --file trace.json --format pdf --output reports/
  erst report --file trace.json --format html,pdf --output reports/`,
	RunE: reportExec,
}

func reportExec(cmd *cobra.Command, args []string) error {
	if reportFile == "" {
		return errors.WrapCliArgumentRequired("file")
	}

	if _, err := os.Stat(reportFile); os.IsNotExist(err) {
		return errors.WrapValidationError(fmt.Sprintf("trace file not found: %s", reportFile))
	}

	if reportOutput == "" {
		reportOutput = "."
	}

	traceData, err := os.ReadFile(reportFile)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to read trace file: %v", err))
	}

	executionTrace, err := trace.FromJSON(traceData)
	if err != nil {
		return errors.WrapUnmarshalFailed(err, "trace")
	}

	builder := report.NewBuilder("Execution Trace Report")
	builder.WithTransactionHash(executionTrace.TransactionHash)

	// Build summary
	totalSteps := len(executionTrace.States)
	errorCount := countErrors(executionTrace.States)
	successRate := calculateSuccessRate(executionTrace.States)

	duration := executionTrace.EndTime.Sub(executionTrace.StartTime).String()
	builder.SetSummary("success", duration, totalSteps, errorCount, countContracts(executionTrace.States), successRate)

	// Add execution steps
	for i, state := range executionTrace.States {
		op := state.Operation
		if state.ContractID != "" && state.Function != "" {
			op = state.ContractID + "::" + state.Function
		}

		status := "success"
		if state.Error != "" {
			status = "error"
		}

		builder.AddExecutionStep(i, op, status, state.Error)
	}

	// Analyze for findings
	if errorCount > 0 {
		builder.AddKeyFinding(fmt.Sprintf("%d errors detected during execution", errorCount))
	}

	contractCount := countContracts(executionTrace.States)
	builder.AddKeyFinding(fmt.Sprintf("%d unique contracts called", contractCount))

	// Risk assessment
	riskLevel := assessRisk(executionTrace.States)
	builder.SetRiskAssessment(riskLevel, calculateRiskScore(executionTrace.States))

	// Metadata
	builder.SetMetadata("execution_trace", "1.0.0", map[string]string{
		"generated_by": "erst",
		"timestamp":    time.Now().Format(time.RFC3339),
	})

	generatedReport := builder.Build()

	exporter, err := report.NewExporter(reportOutput)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to create exporter: %v", err))
	}

	var formats []string
	switch reportFormat {
	case "html":
		formats = []string{"html"}
	case "pdf":
		formats = []string{"pdf"}
	case "html,pdf", "pdf,html":
		formats = []string{"html", "pdf"}
	case "json":
		formats = []string{}
	default:
		formats = []string{"html"}
	}

	if reportFormat == "json" {
		jsonData, err := json.MarshalIndent(generatedReport, "", "  ")
		if err != nil {
			return errors.WrapMarshalFailed(err)
		}

		filename := reportOutput + "/report.json"
		if err := os.WriteFile(filename, jsonData, 0644); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to write JSON report: %v", err))
		}

		fmt.Printf("[OK] Report generated: %s\n", filename)
		return nil
	}

	results, err := exporter.ExportMultiple(generatedReport, formats)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to export report: %v", err))
	}

	for format, path := range results {
		fmt.Printf("[OK] %s report generated: %s\n", string(format), path)
	}

	return nil
}

func countErrors(states []trace.ExecutionState) int {
	count := 0
	for _, state := range states {
		if state.Error != "" {
			count++
		}
	}
	return count
}

func countContracts(states []trace.ExecutionState) int {
	contracts := make(map[string]bool)
	for _, state := range states {
		if state.ContractID != "" {
			contracts[state.ContractID] = true
		}
	}
	return len(contracts)
}

func calculateSuccessRate(states []trace.ExecutionState) float64 {
	if len(states) == 0 {
		return 100.0
	}

	successful := 0
	for _, state := range states {
		if state.Error == "" {
			successful++
		}
	}

	return (float64(successful) / float64(len(states))) * 100
}

func assessRisk(states []trace.ExecutionState) string {
	errorCount := countErrors(states)

	switch {
	case errorCount >= len(states)/2:
		return "critical"
	case errorCount >= len(states)/4:
		return "high"
	case errorCount > 0:
		return "medium"
	default:
		return "low"
	}
}

func calculateRiskScore(states []trace.ExecutionState) float64 {
	if len(states) == 0 {
		return 0
	}

	errorCount := countErrors(states)
	return (float64(errorCount) / float64(len(states))) * 100
}

func init() {
	reportCmd.Flags().StringVar(&reportFormat, "format", "html", "Output format: html, pdf, json, or html,pdf")
	reportCmd.Flags().StringVar(&reportOutput, "output", ".", "Output directory for reports")
	reportCmd.Flags().StringVar(&reportFile, "file", "", "Trace file to analyze")

	rootCmd.AddCommand(reportCmd)
}
