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

package decoder

// Example usage demonstrating how to use the decoder

import (
	"encoding/base64"
	"fmt"

	"github.com/stellar/go/xdr"
)

// DecodeResultXDR decodes a base64-encoded TransactionResult XDR and returns human-readable output
func DecodeResultXDR(resultXDR string) (string, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(resultXDR)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Parse XDR
	var result xdr.TransactionResult
	if err := xdr.SafeUnmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal XDR: %w", err)
	}

	// Format the result
	return FormatTransactionResult(result), nil
}

// Example demonstrates how to use the decoder
func Example() {
	// Example: Decode a transaction that failed due to insufficient balance
	fmt.Println("=== Example 1: Insufficient Balance ===")
	txCodeInfo := DecodeTransactionResultCode(xdr.TransactionResultCodeTxInsufficientBalance)
	fmt.Printf("Error: %s (%s)\n", txCodeInfo.Description, txCodeInfo.Code)
	fmt.Printf("Explanation: %s\n\n", txCodeInfo.Explanation)

	// Example: Decode a payment operation that failed due to no trustline
	fmt.Println("=== Example 2: Payment No Trustline ===")
	opCodeInfo := DecodePaymentResultCode(xdr.PaymentResultCodePaymentNoTrust)
	fmt.Printf("Error: %s (%s)\n", opCodeInfo.Description, opCodeInfo.Code)
	fmt.Printf("Explanation: %s\n\n", opCodeInfo.Explanation)

	// Example: Decode a create account operation that failed
	fmt.Println("=== Example 3: Create Account Underfunded ===")
	createAcctInfo := DecodeCreateAccountResultCode(xdr.CreateAccountResultCodeCreateAccountUnderfunded)
	fmt.Printf("Error: %s (%s)\n", createAcctInfo.Description, createAcctInfo.Code)
	fmt.Printf("Explanation: %s\n", createAcctInfo.Explanation)

	// Output:
	// === Example 1: Insufficient Balance ===
	// Error: Insufficient Balance (tx_insufficient_balance)
	// Explanation: Fee would bring account below minimum reserve. Account needs more XLM
	//
	// === Example 2: Payment No Trustline ===
	// Error: No Trustline (payment_no_trust)
	// Explanation: Destination account doesn't have a trustline for this asset
	//
	// === Example 3: Create Account Underfunded ===
	// Error: Insufficient Funds (create_account_underfunded)
	// Explanation: Source account doesn't have enough XLM to create the account and maintain minimum balance
}
