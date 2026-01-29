// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	hProtocol "github.com/stellar/go/protocols/horizon"
)

// parseTransactionResponse converts a Horizon transaction into a TransactionResponse
func parseTransactionResponse(tx hProtocol.Transaction) *TransactionResponse {
	return &TransactionResponse{
		EnvelopeXdr:   tx.EnvelopeXdr,
		ResultXdr:     tx.ResultXdr,
		ResultMetaXdr: tx.ResultMetaXdr,
	}
}
