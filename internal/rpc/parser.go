// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"

// TransactionResponse holds the XDR data for a transaction
type TransactionResponse struct {
	EnvelopeXdr   string
	ResultXdr     string
	ResultMetaXdr string
}

// ParseTransactionResponse converts a Horizon transaction into a TransactionResponse
func ParseTransactionResponse(tx hProtocol.Transaction) *TransactionResponse {
	return &TransactionResponse{
		EnvelopeXdr:   tx.EnvelopeXdr,
		ResultXdr:     tx.ResultXdr,
		ResultMetaXdr: tx.ResultMetaXdr,
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
