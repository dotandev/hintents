// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package decoder

// Example usage demonstrating how to use the decoder

import (
	"encoding/base64"
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
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
