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

import (
	"strings"
	"testing"

	"github.com/stellar/go/xdr"
)

func TestDecodeTransactionResultCode(t *testing.T) {
	tests := []struct {
		name        string
		code        xdr.TransactionResultCode
		wantCode    string
		wantDesc    string
		containsExp string
	}{
		{
			name:        "tx_success",
			code:        xdr.TransactionResultCodeTxSuccess,
			wantCode:    "tx_success",
			wantDesc:    "Transaction Successful",
			containsExp: "successfully",
		},
		{
			name:        "tx_failed",
			code:        xdr.TransactionResultCodeTxFailed,
			wantCode:    "tx_failed",
			wantDesc:    "Transaction Failed",
			containsExp: "operations failed",
		},
		{
			name:        "tx_insufficient_balance",
			code:        xdr.TransactionResultCodeTxInsufficientBalance,
			wantCode:    "tx_insufficient_balance",
			wantDesc:    "Insufficient Balance",
			containsExp: "minimum reserve",
		},
		{
			name:        "tx_bad_seq",
			code:        xdr.TransactionResultCodeTxBadSeq,
			wantCode:    "tx_bad_seq",
			wantDesc:    "Bad Sequence Number",
			containsExp: "sequence number",
		},
		{
			name:        "tx_bad_auth",
			code:        xdr.TransactionResultCodeTxBadAuth,
			wantCode:    "tx_bad_auth",
			wantDesc:    "Bad Authentication",
			containsExp: "signatures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeTransactionResultCode(tt.code)

			if result.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", result.Code, tt.wantCode)
			}
			if result.Description != tt.wantDesc {
				t.Errorf("Description = %v, want %v", result.Description, tt.wantDesc)
			}
			if !strings.Contains(strings.ToLower(result.Explanation), strings.ToLower(tt.containsExp)) {
				t.Errorf("Explanation %q does not contain %q", result.Explanation, tt.containsExp)
			}
		})
	}
}

func TestDecodeOperationResultCode(t *testing.T) {
	tests := []struct {
		name        string
		code        xdr.OperationResultCode
		wantCode    string
		wantDesc    string
		containsExp string
	}{
		{
			name:        "op_bad_auth",
			code:        xdr.OperationResultCodeOpBadAuth,
			wantCode:    "op_bad_auth",
			wantDesc:    "Bad Authentication",
			containsExp: "signatures",
		},
		{
			name:        "op_no_account",
			code:        xdr.OperationResultCodeOpNoAccount,
			wantCode:    "op_no_account",
			wantDesc:    "Source Account Not Found",
			containsExp: "does not exist",
		},
		{
			name:        "op_too_many_subentries",
			code:        xdr.OperationResultCodeOpTooManySubentries,
			wantCode:    "op_too_many_subentries",
			wantDesc:    "Too Many Subentries",
			containsExp: "1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeOperationResultCode(tt.code)

			if result.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", result.Code, tt.wantCode)
			}
			if result.Description != tt.wantDesc {
				t.Errorf("Description = %v, want %v", result.Description, tt.wantDesc)
			}
			if !strings.Contains(result.Explanation, tt.containsExp) {
				t.Errorf("Explanation %q does not contain %q", result.Explanation, tt.containsExp)
			}
		})
	}
}

func TestDecodePaymentResultCode(t *testing.T) {
	tests := []struct {
		name        string
		code        xdr.PaymentResultCode
		wantCode    string
		wantDesc    string
		containsExp string
	}{
		{
			name:        "payment_underfunded",
			code:        xdr.PaymentResultCodePaymentUnderfunded,
			wantCode:    "payment_underfunded",
			wantDesc:    "Insufficient Funds",
			containsExp: "doesn't have enough",
		},
		{
			name:        "payment_no_trust",
			code:        xdr.PaymentResultCodePaymentNoTrust,
			wantCode:    "payment_no_trust",
			wantDesc:    "No Trustline",
			containsExp: "trustline",
		},
		{
			name:        "payment_not_authorized",
			code:        xdr.PaymentResultCodePaymentNotAuthorized,
			wantCode:    "payment_not_authorized",
			wantDesc:    "Not Authorized",
			containsExp: "not authorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodePaymentResultCode(tt.code)

			if result.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", result.Code, tt.wantCode)
			}
			if result.Description != tt.wantDesc {
				t.Errorf("Description = %v, want %v", result.Description, tt.wantDesc)
			}
			if !strings.Contains(result.Explanation, tt.containsExp) {
				t.Errorf("Explanation %q does not contain %q", result.Explanation, tt.containsExp)
			}
		})
	}
}

func TestDecodeCreateAccountResultCode(t *testing.T) {
	tests := []struct {
		name        string
		code        xdr.CreateAccountResultCode
		wantCode    string
		wantDesc    string
		containsExp string
	}{
		{
			name:        "create_account_underfunded",
			code:        xdr.CreateAccountResultCodeCreateAccountUnderfunded,
			wantCode:    "create_account_underfunded",
			wantDesc:    "Insufficient Funds",
			containsExp: "doesn't have enough",
		},
		{
			name:        "create_account_already_exist",
			code:        xdr.CreateAccountResultCodeCreateAccountAlreadyExist,
			wantCode:    "create_account_already_exist",
			wantDesc:    "Account Already Exists",
			containsExp: "already exists",
		},
		{
			name:        "create_account_low_reserve",
			code:        xdr.CreateAccountResultCodeCreateAccountLowReserve,
			wantCode:    "create_account_low_reserve",
			wantDesc:    "Low Reserve",
			containsExp: "minimum reserve",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeCreateAccountResultCode(tt.code)

			if result.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", result.Code, tt.wantCode)
			}
			if result.Description != tt.wantDesc {
				t.Errorf("Description = %v, want %v", result.Description, tt.wantDesc)
			}
			if !strings.Contains(result.Explanation, tt.containsExp) {
				t.Errorf("Explanation %q does not contain %q", result.Explanation, tt.containsExp)
			}
		})
	}
}

func TestFormatTransactionResult(t *testing.T) {
	// Test successful transaction
	successResult := xdr.TransactionResult{
		Result: xdr.TransactionResultResult{
			Code: xdr.TransactionResultCodeTxSuccess,
		},
	}

	output := FormatTransactionResult(successResult)
	if !strings.Contains(output, "Transaction Successful") {
		t.Errorf("Expected 'Transaction Successful' in output, got: %s", output)
	}
	if !strings.Contains(output, "tx_success") {
		t.Errorf("Expected 'tx_success' in output, got: %s", output)
	}
}
