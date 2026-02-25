// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/trace"
	"github.com/dotandev/hintents/internal/visualizer"
	"github.com/spf13/cobra"
)

var (
	traceFile      string
	traceThemeFlag string
)

var traceCmd = &cobra.Command{
	Use:   "trace <trace-file>",
	Short: "Interactive trace navigation and debugging",
	Long: `Launch an interactive trace viewer for bi-directional navigation through execution traces.

The trace viewer allows you to:
- Step forward and backward through execution
- Jump to specific steps
- Reconstruct state at any point
- View memory and host state changes

Example:
  erst trace execution.json
  erst trace --file debug_trace.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Apply theme if specified, otherwise auto-detect
		if traceThemeFlag != "" {
			visualizer.SetTheme(visualizer.Theme(traceThemeFlag))
		} else {
			visualizer.SetTheme(visualizer.DetectTheme())
		}

		var filename string
		if len(args) > 0 {
			filename = args[0]
		} else if traceFile != "" {
			filename = traceFile
		} else {
			return errors.WrapCliArgumentRequired("file")
		}

		// Check if file exists
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return errors.WrapValidationError(fmt.Sprintf("trace file not found: %s", filename))
		}

		// Load trace from file
		data, err := os.ReadFile(filename)
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to read trace file: %v", err))
		}

		executionTrace, err := trace.FromJSON(data)
		if err != nil {
			return errors.WrapUnmarshalFailed(err, "trace")
		}

		// Start interactive viewer
		viewer := trace.NewInteractiveViewer(executionTrace)
		return viewer.Start()
	},
}

func init() {
	traceCmd.Flags().StringVarP(&traceFile, "file", "f", "", "Trace file to load")
	traceCmd.Flags().StringVar(&traceThemeFlag, "theme", "", "Color theme (default, deuteranopia, protanopia, tritanopia, high-contrast)")
	rootCmd.AddCommand(traceCmd)
}
