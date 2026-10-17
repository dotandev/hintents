// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	invokeArgsFlag    []string
	invokeNoCacheFlag bool
)

// ResolveContractArgs resolves contract argument strings from CLI flags or standard input.
// When "--args -" is supplied, it reads the input from the provided io.Reader (or os.Stdin if nil),
// parsing JSON arrays, individual JSON values, or newline-delimited argument lines.
func ResolveContractArgs(rawArgs []string, r io.Reader) ([]string, error) {
	if len(rawArgs) == 1 && rawArgs[0] == "-" {
		if r == nil {
			r = os.Stdin
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read arguments from stdin: %w", err)
		}

		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return []string{}, nil
		}

		// 1. Try parsing as a JSON array of strings: ["arg1", "arg2"]
		var strArr []string
		if err := json.Unmarshal([]byte(trimmed), &strArr); err == nil {
			return strArr, nil
		}

		// 2. Try parsing as a generic JSON array: [{"key": "val"}, 123]
		var anyArr []any
		if err := json.Unmarshal([]byte(trimmed), &anyArr); err == nil {
			res := make([]string, len(anyArr))
			for i, item := range anyArr {
				if s, ok := item.(string); ok {
					res[i] = s
				} else {
					b, err := json.Marshal(item)
					if err != nil {
						res[i] = fmt.Sprintf("%v", item)
					} else {
						res[i] = string(b)
					}
				}
			}
			return res, nil
		}

		// 3. Fallback: Parse line-delimited arguments if multi-line
		lines := strings.Split(trimmed, "\n")
		var res []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				res = append(res, line)
			}
		}
		if len(res) > 0 {
			return res, nil
		}

		return []string{trimmed}, nil
	}

	return rawArgs, nil
}

// invokeCmd represents the "invoke" command, executing a Soroban contract
// with full support for piping arguments via stdin ("--args -").
var invokeCmd = &cobra.Command{
	Use:     "invoke <contract-path-or-url>",
	GroupID: "testing",
	Short:   "Invoke a Soroban smart contract locally or remotely (supports --args -)",
	Long: `Invoke a Soroban smart contract from a local file or remote URL.

Contract arguments can be provided directly via '--args arg1,arg2' or piped via stdin
using '--args -' (e.g., 'cat payload.json | erst invoke contract.wasm --args -').`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resolvedArgs, err := ResolveContractArgs(invokeArgsFlag, cmd.InOrStdin())
		if err != nil {
			return err
		}

		target := args[0]
		loc, err := ResolveContractPath(target)
		if err != nil {
			return err
		}

		contractBytes, err := LoadContractWithContext(cmd.Context(), target)
		if err != nil {
			return err
		}

		return executeLoadedContract(cmd.Context(), loc.ResolvedPath, contractBytes, resolvedArgs, invokeNoCacheFlag)
	},
}

func init() {
	invokeCmd.Flags().StringSliceVar(&invokeArgsFlag, "args", nil, "Arguments to pass to the contract (use '-' to read from stdin)")
	invokeCmd.Flags().BoolVar(&invokeNoCacheFlag, "no-cache", false, "Disable execution cache")
	rootCmd.AddCommand(invokeCmd)
}
