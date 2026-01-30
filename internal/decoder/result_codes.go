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
	"fmt"

	"github.com/stellar/go/xdr"
)

// TransactionResultCodeInfo contains human-readable information about a transaction result code
type TransactionResultCodeInfo struct {
	Code        string
	Description string
	Explanation string
}

// OperationResultCodeInfo contains human-readable information about an operation result code
type OperationResultCodeInfo struct {
	Code        string
	Description string
	Explanation string
}

// DecodeTransactionResultCode converts a TransactionResultCode to human-readable format
func DecodeTransactionResultCode(code xdr.TransactionResultCode) TransactionResultCodeInfo {
	switch code {
	case xdr.TransactionResultCodeTxSuccess:
		return TransactionResultCodeInfo{
			Code:        "tx_success",
			Description: "Transaction Successful",
			Explanation: "All operations completed successfully",
		}
	case xdr.TransactionResultCodeTxFailed:
		return TransactionResultCodeInfo{
			Code:        "tx_failed",
			Description: "Transaction Failed",
			Explanation: "One or more operations failed. Check individual operation results for details",
		}
	case xdr.TransactionResultCodeTxTooEarly:
		return TransactionResultCodeInfo{
			Code:        "tx_too_early",
			Description: "Transaction Too Early",
			Explanation: "Ledger closeTime before minTime. Wait until the specified minimum time",
		}
	case xdr.TransactionResultCodeTxTooLate:
		return TransactionResultCodeInfo{
			Code:        "tx_too_late",
			Description: "Transaction Too Late",
			Explanation: "Ledger closeTime after maxTime. Transaction expired",
		}
	case xdr.TransactionResultCodeTxMissingOperation:
		return TransactionResultCodeInfo{
			Code:        "tx_missing_operation",
			Description: "Missing Operation",
			Explanation: "Transaction has no operations. At least one operation is required",
		}
	case xdr.TransactionResultCodeTxBadSeq:
		return TransactionResultCodeInfo{
			Code:        "tx_bad_seq",
			Description: "Bad Sequence Number",
			Explanation: "Sequence number does not match source account. Expected sequence number is account sequence + 1",
		}
	case xdr.TransactionResultCodeTxBadAuth:
		return TransactionResultCodeInfo{
			Code:        "tx_bad_auth",
			Description: "Bad Authentication",
			Explanation: "Too few valid signatures or wrong network. Verify signatures and network passphrase",
		}
	case xdr.TransactionResultCodeTxInsufficientBalance:
		return TransactionResultCodeInfo{
			Code:        "tx_insufficient_balance",
			Description: "Insufficient Balance",
			Explanation: "Fee would bring account below minimum reserve. Account needs more XLM",
		}
	case xdr.TransactionResultCodeTxNoAccount:
		return TransactionResultCodeInfo{
			Code:        "tx_no_account",
			Description: "Source Account Not Found",
			Explanation: "Source account does not exist on the network",
		}
	case xdr.TransactionResultCodeTxInsufficientFee:
		return TransactionResultCodeInfo{
			Code:        "tx_insufficient_fee",
			Description: "Insufficient Fee",
			Explanation: "Fee is too small for the transaction. Increase the fee",
		}
	case xdr.TransactionResultCodeTxBadAuthExtra:
		return TransactionResultCodeInfo{
			Code:        "tx_bad_auth_extra",
			Description: "Unused Signatures",
			Explanation: "Transaction has unused signatures. Remove extra signatures",
		}
	case xdr.TransactionResultCodeTxInternalError:
		return TransactionResultCodeInfo{
			Code:        "tx_internal_error",
			Description: "Internal Error",
			Explanation: "An unknown error occurred. This is likely a network issue",
		}
	case xdr.TransactionResultCodeTxNotSupported:
		return TransactionResultCodeInfo{
			Code:        "tx_not_supported",
			Description: "Not Supported",
			Explanation: "Transaction type is not supported by this version of the protocol",
		}
	case xdr.TransactionResultCodeTxFeeBumpInnerSuccess:
		return TransactionResultCodeInfo{
			Code:        "tx_fee_bump_inner_success",
			Description: "Fee Bump Inner Success",
			Explanation: "Fee bump inner transaction succeeded",
		}
	case xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		return TransactionResultCodeInfo{
			Code:        "tx_fee_bump_inner_failed",
			Description: "Fee Bump Inner Failed",
			Explanation: "Fee bump inner transaction failed",
		}
	case xdr.TransactionResultCodeTxBadSponsorship:
		return TransactionResultCodeInfo{
			Code:        "tx_bad_sponsorship",
			Description: "Bad Sponsorship",
			Explanation: "Sponsorship is not confirmed or invalid",
		}
	case xdr.TransactionResultCodeTxBadMinSeqAgeOrGap:
		return TransactionResultCodeInfo{
			Code:        "tx_bad_min_seq_age_or_gap",
			Description: "Bad Min Sequence Age or Gap",
			Explanation: "Preconditions on minSeqAge or minSeqLedgerGap not met",
		}
	case xdr.TransactionResultCodeTxMalformed:
		return TransactionResultCodeInfo{
			Code:        "tx_malformed",
			Description: "Malformed Transaction",
			Explanation: "Transaction is malformed or has invalid parameters",
		}
	case xdr.TransactionResultCodeTxSorobanInvalid:
		return TransactionResultCodeInfo{
			Code:        "tx_soroban_invalid",
			Description: "Soroban Invalid",
			Explanation: "Soroban-specific validation failed",
		}
	default:
		return TransactionResultCodeInfo{
			Code:        fmt.Sprintf("unknown_%d", code),
			Description: "Unknown Error Code",
			Explanation: fmt.Sprintf("Unrecognized transaction result code: %d", code),
		}
	}
}

// DecodeOperationResultCode converts an OperationResultCode to human-readable format
func DecodeOperationResultCode(code xdr.OperationResultCode) OperationResultCodeInfo {
	switch code {
	case xdr.OperationResultCodeOpInner:
		return OperationResultCodeInfo{
			Code:        "op_inner",
			Description: "Operation Specific Result",
			Explanation: "Check the operation-specific result for details",
		}
	case xdr.OperationResultCodeOpBadAuth:
		return OperationResultCodeInfo{
			Code:        "op_bad_auth",
			Description: "Bad Authentication",
			Explanation: "Not enough signatures or wrong signatures for this operation",
		}
	case xdr.OperationResultCodeOpNoAccount:
		return OperationResultCodeInfo{
			Code:        "op_no_account",
			Description: "Source Account Not Found",
			Explanation: "Operation source account does not exist",
		}
	case xdr.OperationResultCodeOpNotSupported:
		return OperationResultCodeInfo{
			Code:        "op_not_supported",
			Description: "Operation Not Supported",
			Explanation: "This operation is not supported in the current protocol version",
		}
	case xdr.OperationResultCodeOpTooManySubentries:
		return OperationResultCodeInfo{
			Code:        "op_too_many_subentries",
			Description: "Too Many Subentries",
			Explanation: "Account has too many subentries (trustlines, offers, etc.). Maximum is 1000",
		}
	case xdr.OperationResultCodeOpExceededWorkLimit:
		return OperationResultCodeInfo{
			Code:        "op_exceeded_work_limit",
			Description: "Exceeded Work Limit",
			Explanation: "Operation exceeded the computational work limit",
		}
	case xdr.OperationResultCodeOpTooManySponsoring:
		return OperationResultCodeInfo{
			Code:        "op_too_many_sponsoring",
			Description: "Too Many Sponsoring",
			Explanation: "Account is sponsoring too many entries",
		}
	default:
		return OperationResultCodeInfo{
			Code:        fmt.Sprintf("unknown_%d", code),
			Description: "Unknown Error Code",
			Explanation: fmt.Sprintf("Unrecognized operation result code: %d", code),
		}
	}
}

// DecodeCreateAccountResultCode decodes CreateAccount operation specific codes
func DecodeCreateAccountResultCode(code xdr.CreateAccountResultCode) OperationResultCodeInfo {
	switch code {
	case xdr.CreateAccountResultCodeCreateAccountSuccess:
		return OperationResultCodeInfo{
			Code:        "create_account_success",
			Description: "Account Created",
			Explanation: "Account was successfully created",
		}
	case xdr.CreateAccountResultCodeCreateAccountMalformed:
		return OperationResultCodeInfo{
			Code:        "create_account_malformed",
			Description: "Malformed Request",
			Explanation: "Invalid destination account ID",
		}
	case xdr.CreateAccountResultCodeCreateAccountUnderfunded:
		return OperationResultCodeInfo{
			Code:        "create_account_underfunded",
			Description: "Insufficient Funds",
			Explanation: "Source account doesn't have enough XLM to create the account and maintain minimum balance",
		}
	case xdr.CreateAccountResultCodeCreateAccountLowReserve:
		return OperationResultCodeInfo{
			Code:        "create_account_low_reserve",
			Description: "Low Reserve",
			Explanation: "Starting balance is less than the minimum reserve (currently 1 XLM)",
		}
	case xdr.CreateAccountResultCodeCreateAccountAlreadyExist:
		return OperationResultCodeInfo{
			Code:        "create_account_already_exist",
			Description: "Account Already Exists",
			Explanation: "Destination account already exists on the network",
		}
	default:
		return OperationResultCodeInfo{
			Code:        fmt.Sprintf("create_account_unknown_%d", code),
			Description: "Unknown Error",
			Explanation: fmt.Sprintf("Unrecognized create account result code: %d", code),
		}
	}
}

// DecodePaymentResultCode decodes Payment operation specific codes
func DecodePaymentResultCode(code xdr.PaymentResultCode) OperationResultCodeInfo {
	switch code {
	case xdr.PaymentResultCodePaymentSuccess:
		return OperationResultCodeInfo{
			Code:        "payment_success",
			Description: "Payment Successful",
			Explanation: "Payment was successfully completed",
		}
	case xdr.PaymentResultCodePaymentMalformed:
		return OperationResultCodeInfo{
			Code:        "payment_malformed",
			Description: "Malformed Request",
			Explanation: "Invalid destination or asset",
		}
	case xdr.PaymentResultCodePaymentUnderfunded:
		return OperationResultCodeInfo{
			Code:        "payment_underfunded",
			Description: "Insufficient Funds",
			Explanation: "Source account doesn't have enough of the asset to send",
		}
	case xdr.PaymentResultCodePaymentSrcNoTrust:
		return OperationResultCodeInfo{
			Code:        "payment_src_no_trust",
			Description: "Source No Trustline",
			Explanation: "Source account doesn't have a trustline for this asset",
		}
	case xdr.PaymentResultCodePaymentSrcNotAuthorized:
		return OperationResultCodeInfo{
			Code:        "payment_src_not_authorized",
			Description: "Source Not Authorized",
			Explanation: "Source account is not authorized to send this asset",
		}
	case xdr.PaymentResultCodePaymentNoDestination:
		return OperationResultCodeInfo{
			Code:        "payment_no_destination",
			Description: "Destination Not Found",
			Explanation: "Destination account does not exist",
		}
	case xdr.PaymentResultCodePaymentNoTrust:
		return OperationResultCodeInfo{
			Code:        "payment_no_trust",
			Description: "No Trustline",
			Explanation: "Destination account doesn't have a trustline for this asset",
		}
	case xdr.PaymentResultCodePaymentNotAuthorized:
		return OperationResultCodeInfo{
			Code:        "payment_not_authorized",
			Description: "Not Authorized",
			Explanation: "Destination account is not authorized to receive this asset",
		}
	case xdr.PaymentResultCodePaymentLineFull:
		return OperationResultCodeInfo{
			Code:        "payment_line_full",
			Description: "Trustline Full",
			Explanation: "Destination trustline limit would be exceeded",
		}
	case xdr.PaymentResultCodePaymentNoIssuer:
		return OperationResultCodeInfo{
			Code:        "payment_no_issuer",
			Description: "Issuer Not Found",
			Explanation: "Asset issuer account does not exist",
		}
	default:
		return OperationResultCodeInfo{
			Code:        fmt.Sprintf("payment_unknown_%d", code),
			Description: "Unknown Error",
			Explanation: fmt.Sprintf("Unrecognized payment result code: %d", code),
		}
	}
}

// FormatTransactionResult formats a complete transaction result with human-readable errors
func FormatTransactionResult(result xdr.TransactionResult) string {
	txCodeInfo := DecodeTransactionResultCode(result.Result.Code)

	output := fmt.Sprintf("Transaction Result: %s\n", txCodeInfo.Description)
	output += fmt.Sprintf("Code: %s\n", txCodeInfo.Code)
	output += fmt.Sprintf("Explanation: %s\n", txCodeInfo.Explanation)

	// If transaction failed, show operation results
	if result.Result.Code == xdr.TransactionResultCodeTxFailed {
		if results := result.Result.Results; results != nil && len(*results) > 0 {
			output += "\nOperation Results:\n"
			for i, opResult := range *results {
				opCodeInfo := DecodeOperationResultCode(opResult.Code)
				output += fmt.Sprintf("  Operation %d: %s (%s)\n", i, opCodeInfo.Description, opCodeInfo.Code)
				if opCodeInfo.Code != "op_inner" {
					output += fmt.Sprintf("    %s\n", opCodeInfo.Explanation)
				}
			}
		}
	}

	return output
}
