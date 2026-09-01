// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dotandev/hintents/internal/cli"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/spf13/cobra"
)

var (
	batchSimulateInputDirFlag    string
	batchSimulateOutputDirFlag   string
	batchSimulateConcurrencyFlag int
	batchSimulateFilePatternFlag string
	batchSimulateTimeoutFlag     time.Duration
	batchSimulateFailFastFlag    bool
	batchSimulateFormatFlag      string
)

// batchSimulateCmd executes parallel simulations of multiple transaction files.
var batchSimulateCmd = &cobra.Command{
	Use:     "batch-simulate",
	GroupID: "testing",
	Short:   "Simulate multiple transactions in parallel",
	Long: `Simulate multiple transactions in parallel with configurable concurrency.

This command:
  1) Walks the input directory for transaction files matching the pattern
  2) Spawns parallel simulator instances (up to --concurrency)
  3) Captures results in the output directory
  4) Optionally stops on first failure with --fail-fast

The simulator binary is discovered via:
  - ERST_SIM_PATH environment variable
  - Local directory (./erst-sim, ./bin/erst-sim)
  - Dev targets (simulator/target/debug/erst-sim, etc.)
  - Global PATH

Example:
  erst batch-simulate --input-dir ./transactions --output-dir ./results
  erst batch-simulate --input-dir ./txs --concurrency 4 --timeout 1m --fail-fast`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if batchSimulateInputDirFlag == "" {
			return fmt.Errorf("--input-dir is required")
		}
		return nil
	},
	RunE: runBatchSimulate,
}

func init() {
	batchSimulateCmd.Flags().StringVar(&batchSimulateInputDirFlag, "input-dir", "", "Directory containing transaction files (required)")
	batchSimulateCmd.Flags().StringVar(&batchSimulateOutputDirFlag, "output-dir", "./batch-output", "Directory for simulation results")
	batchSimulateCmd.Flags().IntVar(&batchSimulateConcurrencyFlag, "concurrency", 0, "Number of parallel simulators (default: number of CPU cores)")
	batchSimulateCmd.Flags().StringVar(&batchSimulateFilePatternFlag, "file-pattern", "*.json", "Glob pattern for transaction files")
	batchSimulateCmd.Flags().DurationVar(&batchSimulateTimeoutFlag, "timeout", 30*time.Second, "Per-simulation timeout")
	batchSimulateCmd.Flags().BoolVar(&batchSimulateFailFastFlag, "fail-fast", false, "Stop all on first failure")
	batchSimulateCmd.Flags().StringVar(&batchSimulateFormatFlag, "format", "text", "Output format: text or json")

	_ = batchSimulateCmd.MarkFlagRequired("input-dir")

	rootCmd.AddCommand(batchSimulateCmd)
}

func runBatchSimulate(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	switch strings.ToLower(batchSimulateFormatFlag) {
	case "", "text":
		return runBatchSimulateText(ctx, cmd)
	case "json":
		return runBatchSimulateJSON(ctx, cmd)
	default:
		return errors.WrapValidationError(fmt.Sprintf("unsupported format: %s (use: text, json)", batchSimulateFormatFlag))
	}
}

func runBatchSimulateText(ctx context.Context, cmd *cobra.Command) error {
	// Set default concurrency if not specified
	concurrency := batchSimulateConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 0 // Will be set to runtime.NumCPU() by RunBatch
	}

	cfg := cli.BatchConfig{
		InputDir:    batchSimulateInputDirFlag,
		OutputDir:   batchSimulateOutputDirFlag,
		Concurrency: concurrency,
		FilePattern: batchSimulateFilePatternFlag,
		Timeout:     batchSimulateTimeoutFlag,
		FailFast:    batchSimulateFailFastFlag,
	}

	startTime := time.Now()
	results, err := cli.RunBatch(ctx, cfg)
	duration := time.Since(startTime)

	// Count successes and failures
	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	// Print summary
	fmt.Printf("Batch complete: %d succeeded, %d failed, %d total in %ds\n",
		successCount, failureCount, len(results), int64(duration.Seconds()))

	// Print failures with details
	if failureCount > 0 {
		fmt.Println("\nFailures:")
		for _, result := range results {
			if !result.Success {
				fmt.Printf("  %s: %v\n", result.FilePath, result.Error)
				if len(result.Output) > 0 {
					logger.Logger.Debug("Failure output", "file", result.FilePath, "output", string(result.Output))
				}
			}
		}
	}

	// Exit code: 0 if all succeeded, 1 if any failed
	if failureCount > 0 {
		return fmt.Errorf("batch simulate: %d file(s) failed", failureCount)
	}

	return err
}

func runBatchSimulateJSON(ctx context.Context, cmd *cobra.Command) error {
	concurrency := batchSimulateConcurrencyFlag
	if concurrency <= 0 {
		concurrency = 0
	}

	cfg := cli.BatchConfig{
		InputDir:    batchSimulateInputDirFlag,
		OutputDir:   batchSimulateOutputDirFlag,
		Concurrency: concurrency,
		FilePattern: batchSimulateFilePatternFlag,
		Timeout:     batchSimulateTimeoutFlag,
		FailFast:    batchSimulateFailFastFlag,
	}

	startTime := time.Now()
	results, err := cli.RunBatch(ctx, cfg)
	duration := time.Since(startTime)

	summary := batchSimulateJSONSummary{
		Succeeded:      0,
		Failed:         0,
		Total:          len(results),
		DurationSeconds: duration.Seconds(),
		Results:        make([]batchSimulateJSONResult, 0, len(results)),
	}

	for _, result := range results {
		item := batchSimulateJSONResult{
			FilePath:        result.FilePath,
			Success:         result.Success,
			DurationSeconds: result.Duration.Seconds(),
		}
		if result.Error != nil {
			item.Error = result.Error.Error()
		}
		if len(result.Output) > 0 {
			item.Output = string(result.Output)
		}
		summary.Results = append(summary.Results, item)
		if result.Success {
			summary.Succeeded++
		} else {
			summary.Failed++
		}
	}

	if err != nil {
		summary.Error = err.Error()
	}
	if summary.Failed > 0 {
		summary.Status = "failed"
	} else if summary.Error != "" {
		summary.Status = "error"
	} else {
		summary.Status = "success"
	}

	payload, marshalErr := json.MarshalIndent(summary, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("marshal batch json output: %w", marshalErr)
	}
	if _, printErr := fmt.Fprintln(cmd.OutOrStdout(), string(payload)); printErr != nil {
		return printErr
	}

	if summary.Failed > 0 {
		return fmt.Errorf("batch simulate: %d file(s) failed", summary.Failed)
	}
	return err
}

type batchSimulateJSONSummary struct {
	Status          string                     `json:"status"`
	Succeeded       int                        `json:"succeeded"`
	Failed          int                        `json:"failed"`
	Total           int                        `json:"total"`
	DurationSeconds float64                    `json:"duration_seconds"`
	Error           string                     `json:"error,omitempty"`
	Results         []batchSimulateJSONResult `json:"results"`
}

type batchSimulateJSONResult struct {
	FilePath        string  `json:"file_path"`
	Success         bool    `json:"success"`
	Error           string  `json:"error,omitempty"`
	Output          string  `json:"output,omitempty"`
	DurationSeconds float64 `json:"duration_seconds"`
}
