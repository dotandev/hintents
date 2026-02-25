// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/profile"
	"github.com/dotandev/hintents/internal/trace"
	"github.com/spf13/cobra"
)

var (
	profileTraceFile string
	profileOutput    string
)

var profileCmd = &cobra.Command{
	Use:   "profile [trace-file]",
	Short: "Export trace as pprof profile for gas-to-function mapping",
	Long: `Synthesize trace events into a pprof-compliant profile that maps gas
consumption to functions. The output can be viewed with go tool pprof.

Example:
  erst profile execution.json -o gas.pb.gz
  erst profile --file debug_trace.json -o gas.pb.gz
  go tool pprof gas.pb.gz`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var filename string
		if len(args) > 0 {
			filename = args[0]
		} else if profileTraceFile != "" {
			filename = profileTraceFile
		} else {
			return fmt.Errorf("trace file required. Use: erst profile <file> or --file <file>")
		}

		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return fmt.Errorf("trace file not found: %s", filename)
		}

		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read trace file: %w", err)
		}

		execTrace, err := trace.FromJSON(data)
		if err != nil {
			return fmt.Errorf("failed to parse trace file: %w", err)
		}

		outPath := profileOutput
		if outPath == "" {
			outPath = "profile.pb.gz"
		}

		out, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer out.Close()

		if err := profile.WritePprof(execTrace, out); err != nil {
			return fmt.Errorf("failed to write pprof profile: %w", err)
		}

		fmt.Printf("Profile written to %s\n", outPath)
		fmt.Printf("View with: go tool pprof %s\n", outPath)
		return nil
	},
}

func init() {
	profileCmd.Flags().StringVarP(&profileTraceFile, "file", "f", "", "Trace file to load")
	profileCmd.Flags().StringVarP(&profileOutput, "output", "o", "profile.pb.gz", "Output pprof file path")
	rootCmd.AddCommand(profileCmd)
}
