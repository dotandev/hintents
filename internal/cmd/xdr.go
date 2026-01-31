// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"fmt"

	"github.com/dotandev/hintents/internal/decoder"
	"github.com/spf13/cobra"
)

var (
	xdrFormat string
	xdrData   string
	xdrType   string
)

var xdrCmd = &cobra.Command{
	Use:   "xdr",
	Short: "Format and decode XDR data",
	Long:  `Decode and format XDR structures to JSON or table format for easy inspection.`,
	RunE:  xdrExec,
}

func xdrExec(cmd *cobra.Command, args []string) error {
	if xdrData == "" {
		return fmt.Errorf("XDR data required (use --data or pipe via stdin)")
	}

	data, err := base64.StdEncoding.DecodeString(xdrData)
	if err != nil {
		return fmt.Errorf("invalid base64 input: %w", err)
	}

	var output interface{}

	switch xdrType {
	case "ledger-entry":
		le, err := decoder.DecodeXDRBase64AsLedgerEntry(string(data))
		if err != nil {
			return fmt.Errorf("failed to decode ledger entry: %w", err)
		}
		output = le

	case "diagnostic-event":
		event, err := decoder.DecodeXDRBase64AsDiagnosticEvent(string(data))
		if err != nil {
			return fmt.Errorf("failed to decode diagnostic event: %w", err)
		}
		output = event

	default:
		return fmt.Errorf("unsupported XDR type: %s (use: ledger-entry, diagnostic-event)", xdrType)
	}

	formatter := decoder.NewXDRFormatter(decoder.FormatType(xdrFormat))
	result, err := formatter.Format(output)
	if err != nil {
		return fmt.Errorf("formatting failed: %w", err)
	}

	fmt.Println(result)
	return nil
}

func init() {
	rootCmd.AddCommand(xdrCmd)

	xdrCmd.Flags().StringVar(&xdrData, "data", "", "Base64-encoded XDR data to decode")
	xdrCmd.Flags().StringVar(&xdrFormat, "format", "json", "Output format: json or table")
	xdrCmd.Flags().StringVar(&xdrType, "type", "ledger-entry", "XDR type: ledger-entry, diagnostic-event")

	_ = xdrCmd.MarkFlagRequired("data")
}
