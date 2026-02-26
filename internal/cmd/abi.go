// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/abi"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/spf13/cobra"
)

var abiFormat string

var abiCmd = &cobra.Command{
	Use:     "abi <wasm-file>",
	GroupID: "utility",
	Short:   "Decompile and display a Soroban contract ABI",
	Long: `Parse a compiled Soroban WASM file and pretty-print the contract specification
(functions, structs, enums, unions, error enums, and events).

The contract spec is read from the "contractspecv0" WASM custom section, which
Soroban compilers embed automatically.

Examples:
  erst abi ./target/wasm32-unknown-unknown/release/contract.wasm
  erst abi --format json ./contract.wasm`,
	Args: cobra.ExactArgs(1),
	RunE: abiExec,
}

func abiExec(cmd *cobra.Command, args []string) error {
	wasmBytes, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading WASM file: %w", err)
	}

	specBytes, err := abi.ExtractCustomSection(wasmBytes, "contractspecv0")
	if err != nil {
		return err
	}
	if specBytes == nil {
		return errors.WrapSpecNotFound()
	}

	spec, err := abi.DecodeContractSpec(specBytes)
	if err != nil {
		return err
	}

	switch abiFormat {
	case "json":
		output, err := abi.FormatJSON(spec)
		if err != nil {
			return err
		}
		fmt.Println(output)
	case "text":
		fmt.Print(abi.FormatText(spec))
	default:
		return errors.WrapValidationError(fmt.Sprintf("unsupported format: %s (use: text, json)", abiFormat))
	}

	return nil
}

func init() {
	abiCmd.Flags().StringVar(&abiFormat, "format", "text", "Output format: text or json")
	rootCmd.AddCommand(abiCmd)
}
