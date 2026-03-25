// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	sandboxDefaultAmountXLM uint64 = 10_000
	sandboxStroopsPerXLM    uint64 = 10_000_000
	sandboxDefaultStateFile        = "overrides/sandbox.json"
)

var (
	sandboxAmountFlag uint64
	sandboxStateFile  string
)

// sandboxCmd is the parent command for local sandbox helpers.
var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Local sandbox utilities for simulated ledger state",
	Long: `Manage a local \"sandbox\" overlay for simulations.

This command family operates purely on local override state and never
submits transactions on-chain.`,
}

// sandboxFundCmd implements:
//
//	erst sandbox fund <account>
//
// It creates or updates a JSON override file with a synthetic native XLM
// balance for the given account, suitable for use with local simulations.
var sandboxFundCmd = &cobra.Command{
	Use:   "fund <account>",
	Short: "Mock-fund an account in the local sandbox ledger",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account := args[0]

		if err := validateSandboxAccount(account); err != nil {
			return errors.WrapValidationError(
				fmt.Sprintf("invalid Stellar account ID %q: %v", account, err),
			)
		}

		amountXLM := sandboxAmountFlag
		if amountXLM == 0 {
			amountXLM = sandboxDefaultAmountXLM
		}

		if sandboxStateFile == "" {
			sandboxStateFile = sandboxDefaultStateFile
		}

		if err := ensureSandboxDir(filepath.Dir(sandboxStateFile)); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to prepare sandbox directory: %v", err))
		}

		if err := writeSandboxFunding(account, amountXLM, sandboxStateFile); err != nil {
			return err
		}

		fmt.Printf("Sandbox funded: %s with %d XLM (override state: %s)\n", account, amountXLM, sandboxStateFile)
		fmt.Println("Use this override file when running simulations to activate the local sandbox balance.")
		return nil
	},
}

// validateSandboxAccount performs a basic strkey check so we fail fast with
// a helpful error instead of panicking inside xdr.MustAddress.
func validateSandboxAccount(addr string) error {
	decoded, err := strkey.Decode(strkey.VersionByteAccountID, addr)
	if err != nil {
		return err
	}
	if len(decoded) != 32 {
		return fmt.Errorf("account id must decode to 32 bytes, got %d", len(decoded))
	}
	return nil
}

func ensureSandboxDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// writeSandboxFunding creates or updates a JSON override file with a single
// native-account ledger entry for the given account.
func writeSandboxFunding(account string, amountXLM uint64, path string) error {
	// Load existing override data if present so repeated calls are additive.
	override := OverrideData{
		LedgerEntries: make(map[string]string),
	}

	if data, err := os.ReadFile(path); err == nil {
		// Best-effort parse; on failure we surface a clear error instead of
		// silently discarding the existing file.
		if err := json.Unmarshal(data, &override); err != nil {
			return errors.WrapValidationError(
				fmt.Sprintf("failed to parse existing sandbox state %s: %v", path, err),
			)
		}
		if override.LedgerEntries == nil {
			override.LedgerEntries = make(map[string]string)
		}
	} else if !os.IsNotExist(err) {
		return errors.WrapValidationError(
			fmt.Sprintf("failed to read existing sandbox state %s: %v", path, err),
		)
	}

	accountID := xdr.MustAddress(account)

	if amountXLM > math.MaxInt64/sandboxStroopsPerXLM {
		return errors.WrapValidationError("requested amount is too large for sandbox balance")
	}
	amountStroops := int64(amountXLM * sandboxStroopsPerXLM)

	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 1,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{
				AccountId: accountID,
				Balance:   xdr.Int64(amountStroops),
				SeqNum:    xdr.SequenceNumber(1),
			},
		},
	}

	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: accountID,
		},
	}

	keyXDR, err := rpc.EncodeLedgerKey(key)
	if err != nil {
		return err
	}
	entryXDR, err := rpc.EncodeLedgerEntry(entry)
	if err != nil {
		return err
	}

	if override.LedgerEntries == nil {
		override.LedgerEntries = make(map[string]string)
	}
	override.LedgerEntries[keyXDR] = entryXDR

	data, err := json.MarshalIndent(override, "", "  ")
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to serialize sandbox state: %v", err))
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to write sandbox state %s: %v", path, err))
	}

	return nil
}

func init() {
	sandboxFundCmd.Flags().Uint64Var(
		&sandboxAmountFlag,
		"amount",
		sandboxDefaultAmountXLM,
		"Amount of native tokens (XLM) to mock-fund in the sandbox ledger",
	)
	sandboxFundCmd.Flags().StringVar(
		&sandboxStateFile,
		"state-file",
		sandboxDefaultStateFile,
		"JSON override file to create/update for sandbox ledger state",
	)

	sandboxCmd.AddCommand(sandboxFundCmd)
	rootCmd.AddCommand(sandboxCmd)
}
