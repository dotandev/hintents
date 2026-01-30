// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"time"

	hProtocol "github.com/stellar/go/protocols/horizon"
)

// LedgerHeaderResponse contains essential ledger header information
// needed for transaction replay simulation. This structure provides
// all the metadata required to recreate the blockchain state at the
// time a transaction was executed.
type LedgerHeaderResponse struct {
	// Core ledger identifiers
	Sequence uint32 // Ledger sequence number
	Hash     string // Ledger hash (hex-encoded SHA-256)
	PrevHash string // Previous ledger hash

	// Timing information
	CloseTime time.Time // When the ledger closed

	// Protocol and network parameters
	ProtocolVersion uint32 // Stellar protocol version
	BaseFee         int32  // Base fee in stroops (1 stroop = 0.0000001 XLM)
	BaseReserve     int32  // Base reserve in stroops
	MaxTxSetSize    int32  // Maximum transaction set size

	// Network state
	TotalCoins string // Total lumens in circulation
	FeePool    string // Fee pool amount

	// XDR data
	HeaderXDR string // Base64-encoded LedgerHeader XDR

	// Transaction statistics
	SuccessfulTxCount int32 // Number of successful transactions
	FailedTxCount     int32 // Number of failed transactions
	OperationCount    int32 // Total operations in ledger
}

// FromHorizonLedger converts a Horizon ledger response to our internal structure.
// This provides a clean abstraction layer between the Horizon API and our
// internal representation, making it easier to add alternative data sources
// (like Soroban RPC) in the future.
func FromHorizonLedger(hl hProtocol.Ledger) *LedgerHeaderResponse {
	failedTxCount := int32(0)
	if hl.FailedTransactionCount != nil {
		failedTxCount = *hl.FailedTransactionCount
	}

	return &LedgerHeaderResponse{
		Sequence:          uint32(hl.Sequence),
		Hash:              hl.Hash,
		PrevHash:          hl.PrevHash,
		CloseTime:         hl.ClosedAt,
		ProtocolVersion:   uint32(hl.ProtocolVersion),
		BaseFee:           hl.BaseFee,
		BaseReserve:       hl.BaseReserve,
		MaxTxSetSize:      hl.MaxTxSetSize,
		TotalCoins:        hl.TotalCoins,
		FeePool:           hl.FeePool,
		HeaderXDR:         hl.HeaderXDR,
		SuccessfulTxCount: hl.SuccessfulTransactionCount,
		FailedTxCount:     failedTxCount,
		OperationCount:    hl.OperationCount,
	}
}
