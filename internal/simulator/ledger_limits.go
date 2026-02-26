// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// MaxLedgerEntriesSizeBytes is the Soroban network limit for the total size of
// ledger entries passed to a single transaction.  Transactions whose read/write
// footprint exceeds this limit are rejected by the network before execution.
//
// Source: Soroban protocol limit `max_transaction_size_bytes` = 1 MiB.
// Ref: https://github.com/stellar/stellar-core/blob/master/src/herder/TxSetUtils.cpp
const MaxLedgerEntriesSizeBytes = 1 * 1024 * 1024 // 1 MiB

// LedgerSizeWarning is returned by CheckLedgerEntriesSize when the total
// base64-decoded size of the provided entries exceeds MaxLedgerEntriesSizeBytes.
type LedgerSizeWarning struct {
	TotalBytes int
	LimitBytes int
	EntryCount int
}

func (w *LedgerSizeWarning) Error() string {
	return fmt.Sprintf(
		"ledger entries total size %d bytes (%d entries) exceeds the %d-byte network limit — "+
			"this transaction would be rejected by the Soroban network",
		w.TotalBytes, w.EntryCount, w.LimitBytes,
	)
}

// CheckLedgerEntriesSize computes the total byte size of the decoded ledger
// entry values in entries and returns a *LedgerSizeWarning if it exceeds
// MaxLedgerEntriesSizeBytes.  It returns nil when the size is within limits.
//
// entries is a map of base64-encoded XDR LedgerKey → base64-encoded XDR
// LedgerEntry, as produced by rpc.Client.GetLedgerEntries.  Both key and
// value bytes are counted because both are transmitted in the transaction
// footprint.
//
// Entries whose values cannot be base64-decoded are counted as zero bytes
// (the simulator will surface the decode error separately).
func CheckLedgerEntriesSize(entries map[string]string) *LedgerSizeWarning {
	total := 0
	for k, v := range entries {
		// Key bytes
		if kb, err := base64.StdEncoding.DecodeString(k); err == nil {
			total += len(kb)
		}
		// Value bytes
		if vb, err := base64.StdEncoding.DecodeString(v); err == nil {
			total += len(vb)
		}
	}

	if total > MaxLedgerEntriesSizeBytes {
		return &LedgerSizeWarning{
			TotalBytes: total,
			LimitBytes: MaxLedgerEntriesSizeBytes,
			EntryCount: len(entries),
		}
	}
	return nil
}

// WarnLedgerEntriesSize calls CheckLedgerEntriesSize and, if the size limit is
// exceeded, writes a formatted warning to w (typically os.Stderr).  It returns
// true when a warning was emitted so callers can decide whether to surface it
// differently (e.g. prefix with a command name).
func WarnLedgerEntriesSize(entries map[string]string, w io.Writer) bool {
	warning := CheckLedgerEntriesSize(entries)
	if warning == nil {
		return false
	}

	fmt.Fprintf(w,
		"WARNING: %s\n"+
			"         Total: %s across %d entries (limit: %s)\n"+
			"         The network will reject this transaction. "+
			"Reduce the number of ledger entries read in a single invocation.\n",
		warning.Error(),
		formatBytes(warning.TotalBytes),
		warning.EntryCount,
		formatBytes(warning.LimitBytes),
	)
	return true
}

// WarnLedgerEntriesSizeToStderr is a convenience wrapper that writes to
// os.Stderr, matching the CLI convention that warnings go to stderr so they
// don't corrupt stdout output.
func WarnLedgerEntriesSizeToStderr(entries map[string]string) bool {
	return WarnLedgerEntriesSize(entries, os.Stderr)
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(n int) string {
	const kib = 1024
	const mib = 1024 * kib
	switch {
	case n >= mib:
		return fmt.Sprintf("%.2f MiB (%d bytes)", float64(n)/float64(mib), n)
	case n >= kib:
		return fmt.Sprintf("%.2f KiB (%d bytes)", float64(n)/float64(kib), n)
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}