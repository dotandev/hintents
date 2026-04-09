// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

// LedgerEntryResult represents a single ledger entry returned by the Soroban RPC.
type LedgerEntryResult struct {
	Key                string `json:"key"`
	Xdr                string `json:"xdr"`
	LastModifiedLedger int    `json:"lastModifiedLedgerSeq"`
	LiveUntilLedger    int    `json:"liveUntilLedgerSeq"`
}
