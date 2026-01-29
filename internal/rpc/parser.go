package rpc

import hProtocol "github.com/stellar/go/protocols/horizon"

// TransactionResponse contains the raw XDR fields needed for simulation
type TransactionResponse struct {
	EnvelopeXdr   string
	ResultXdr     string
	ResultMetaXdr string
}

// ParseTransactionResponse converts a Horizon transaction into our response format
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
