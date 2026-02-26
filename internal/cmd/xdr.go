// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"fmt"

	"github.com/dotandev/hintents/internal/decoder"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/spf13/cobra"
)

var (
	xdrFormat string
	xdrData   string
	xdrType   string
)

var xdrCmd = &cobra.Command{
	Use:     "xdr",
	GroupID: "utility",
	Short:   "Format and decode XDR data",
	Long:    `Decode and format XDR structures to JSON or table format for easy inspection.`,
	RunE:    xdrExec,
}

func xdrExec(cmd *cobra.Command, args []string) error {
	if xdrData == "" {
		return errors.WrapCliArgumentRequired("data")
	}

	data, err := base64.StdEncoding.DecodeString(xdrData)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("invalid base64 input: %v", err))
	}

	var output interface{}

	switch xdrType {
	case "ledger-entry":
		le, err := decoder.DecodeXDRBase64AsLedgerEntry(string(data))
		if err != nil {
			return errors.WrapUnmarshalFailed(err, "ledger entry")
		}
		output = le

	case "diagnostic-event":
		event, err := decoder.DecodeXDRBase64AsDiagnosticEvent(string(data))
		if err != nil {
			return errors.WrapUnmarshalFailed(err, "diagnostic event")
		}
		output = event

	default:
		return errors.WrapValidationError(fmt.Sprintf("unsupported XDR type: %s (use: ledger-entry, diagnostic-event)", xdrType))
	}

	formatter := decoder.NewXDRFormatter(decoder.FormatType(xdrFormat))
	result, err := formatter.Format(output)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("formatting failed: %v", err))
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
