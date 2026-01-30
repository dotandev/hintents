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
	"os"

	"github.com/dotandev/hintents/internal/trace"
	"github.com/spf13/cobra"
)

var (
	traceFile string
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
		var filename string
		if len(args) > 0 {
			filename = args[0]
		} else if traceFile != "" {
			filename = traceFile
		} else {
			return fmt.Errorf("trace file required. Use: erst trace <file> or --file <file>")
		}

		// Check if file exists
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			return fmt.Errorf("trace file not found: %s", filename)
		}

		// Load trace from file
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read trace file: %w", err)
		}

		executionTrace, err := trace.FromJSON(data)
		if err != nil {
			return fmt.Errorf("failed to parse trace file: %w", err)
		}

		// Start interactive viewer
		viewer := trace.NewInteractiveViewer(executionTrace)
		return viewer.Start()
	},
}

func init() {
	traceCmd.Flags().StringVarP(&traceFile, "file", "f", "", "Trace file to load")
	rootCmd.AddCommand(traceCmd)
}
