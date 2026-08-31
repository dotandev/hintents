// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/pipeline"
	"github.com/spf13/cobra"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage and run Programmable Transaction Pipelines (PTB-style)",
}

var pipelineRunCmd = &cobra.Command{
	Use:   "run [file.json]",
	Short: "Run a pipeline from a JSON file (or stdin)",
	RunE: func(cmd *cobra.Command, args []string) error {
		var data []byte
		var err error

		if len(args) > 0 {
			data, err = os.ReadFile(args[0])
			if err != nil {
				return errors.WrapValidationError(fmt.Sprintf("failed to read file: %v", err))
			}
		} else {
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				data, err = io.ReadAll(os.Stdin)
				if err != nil {
					return errors.WrapValidationError("failed to read from stdin")
				}
			} else {
				return errors.WrapValidationError("must provide file or pipe JSON to stdin")
			}
		}

		p, err := pipeline.FromJSON(data)
		if err != nil {
			return err
		}

		fmt.Printf("Pipeline loaded with %d commands.\n", len(p.Commands))
		fmt.Println("Executing Pipeline (Simulation)...")
		for i, c := range p.Commands {
			fmt.Printf("[%d] %s -> %s (Args: %v)\n", i, c.Type, c.Target, c.Args)
		}
		fmt.Println("Pipeline execution finished.")
		return nil
	},
}

func init() {
	pipelineCmd.AddCommand(pipelineRunCmd)
	rootCmd.AddCommand(pipelineCmd)
}
