// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"

// TransactionResponse holds the XDR data for a transaction
type TransactionResponse struct {
	EnvelopeXdr   string
	ResultXdr     string
	ResultMetaXdr string

	// LedgerSequence and LedgerCloseTime place the transaction in time. The
	// host refuses to run without ledger info ("missing ledger info"), and a
	// contract reading the sequence or timestamp needs the values it
	// originally saw, so a replay has to carry them.
	LedgerSequence  uint32
	LedgerCloseTime int64

	// Successful is the on-chain outcome, the reference a replay is judged
	// against.
	Successful bool
}

// ParseTransactionResponse converts a Horizon transaction into a TransactionResponse
func ParseTransactionResponse(tx hProtocol.Transaction) *TransactionResponse {
	var ledgerSequence uint32
	if tx.Ledger > 0 {
		ledgerSequence = uint32(tx.Ledger)
	}

	var closeTime int64
	if !tx.LedgerCloseTime.IsZero() {
		closeTime = tx.LedgerCloseTime.Unix()
	}

	return &TransactionResponse{
		EnvelopeXdr:     tx.EnvelopeXdr,
		ResultXdr:       tx.ResultXdr,
		ResultMetaXdr:   tx.ResultMetaXdr,
		LedgerSequence:  ledgerSequence,
		LedgerCloseTime: closeTime,
		Successful:      tx.Successful,
	}
}

// ExtractEnvelopeXdr extracts the envelope XDR from a transaction response
func ExtractEnvelopeXdr(resp *TransactionResponse) string {
	if resp == nil {
		return ""
	}
	return resp.EnvelopeXdr
}

// ExtractResultXdr extracts the result XDR from a transaction response
func ExtractResultXdr(resp *TransactionResponse) string {
	if resp == nil {
		return ""
	}
	return resp.ResultXdr
}

// ExtractResultMetaXdr extracts the result meta XDR from a transaction response
func ExtractResultMetaXdr(resp *TransactionResponse) string {
	if resp == nil {
		return ""
	}
	return resp.ResultMetaXdr
}
