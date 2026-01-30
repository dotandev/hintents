// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpc

import hProtocol "github.com/stellar/go/protocols/horizon"

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
