# XDR Result Code Decoder

## Overview

The XDR Result Code Decoder translates binary Stellar XDR error codes into human-readable error messages with detailed explanations. This eliminates the need for developers to manually look up error codes like `tx_failed (-1)` or `op_underfunded`.

## Features

-  **Transaction-Level Error Codes**: Comprehensive mapping of all `TransactionResultCode` values
-  **Operation-Level Error Codes**: Detailed mappings for operation-specific errors
-  **Human-Readable Messages**: Clear descriptions and explanations for each error
-  **Actionable Guidance**: Explanations include suggestions on how to fix the issue

## Usage

### Basic Usage

```go
import "github.com/dotandev/hintents/internal/decoder"

// Decode a transaction result code
txCodeInfo := decoder.DecodeTransactionResultCode(xdr.TransactionResultCodeTxInsufficientBalance)
fmt.Printf("Error: %s (%s)\n", txCodeInfo.Description, txCodeInfo.Code)
fmt.Printf("Explanation: %s\n", txCodeInfo.Explanation)

// Output:
// Error: Insufficient Balance (tx_insufficient_balance)
// Explanation: Fee would bring account below minimum reserve. Account needs more XLM
```

### Decode from XDR String

```go
// Decode a base64-encoded TransactionResult XDR
output, err := decoder.DecodeResultXDR(resultXDRBase64)
if err != nil {
    log.Fatal(err)
}
fmt.Println(output)
```

### Format Complete Transaction Result

```go
// Format a complete transaction result with all operation results
formatted := decoder.FormatTransactionResult(transactionResult)
fmt.Println(formatted)

// Output:
// Transaction Result: Transaction Failed
// Code: tx_failed
// Explanation: One or more operations failed. Check individual operation results for details
//
// Operation Results:
//   Operation 0: Insufficient Funds (payment_underfunded)
//     Source account doesn't have enough of the asset to send
```

## Supported Error Codes

### Transaction-Level Codes

| Code | Description | Common Cause |
|------|-------------|--------------|
| `tx_success` | Transaction Successful | All operations completed successfully |
| `tx_failed` | Transaction Failed | One or more operations failed |
| `tx_insufficient_balance` | Insufficient Balance | Fee would bring account below minimum reserve |
| `tx_bad_seq` | Bad Sequence Number | Sequence number doesn't match account |
| `tx_bad_auth` | Bad Authentication | Invalid or missing signatures |
| `tx_insufficient_fee` | Insufficient Fee | Fee is too low |
| `tx_no_account` | Source Account Not Found | Account doesn't exist |
| `tx_too_early` | Transaction Too Early | Before minTime |
| `tx_too_late` | Transaction Too Late | After maxTime (expired) |
| `tx_malformed` | Malformed Transaction | Invalid parameters |
| `tx_soroban_invalid` | Soroban Invalid | Soroban-specific validation failed |

### Operation-Level Codes

| Code | Description | Common Cause |
|------|-------------|--------------|
| `op_bad_auth` | Bad Authentication | Missing signatures for operation |
| `op_no_account` | Source Account Not Found | Operation source doesn't exist |
| `op_too_many_subentries` | Too Many Subentries | Account has 1000+ subentries |
| `op_exceeded_work_limit` | Exceeded Work Limit | Computational limit exceeded |

### Payment Operation Codes

| Code | Description | Common Cause |
|------|-------------|--------------|
| `payment_underfunded` | Insufficient Funds | Not enough of the asset |
| `payment_no_trust` | No Trustline | Destination lacks trustline |
| `payment_not_authorized` | Not Authorized | Asset authorization required |
| `payment_line_full` | Trustline Full | Would exceed trustline limit |
| `payment_no_issuer` | Issuer Not Found | Asset issuer doesn't exist |

### Create Account Operation Codes

| Code | Description | Common Cause |
|------|-------------|--------------|
| `create_account_underfunded` | Insufficient Funds | Not enough XLM to create account |
| `create_account_already_exist` | Account Already Exists | Destination already exists |
| `create_account_low_reserve` | Low Reserve | Starting balance < 1 XLM |

## Integration with CLI

The decoder is integrated into the `erst debug` command to automatically display human-readable errors:

```bash
$ erst debug abc123...def789

Transaction Result: Insufficient Balance (tx_insufficient_balance)
Explanation: Fee would bring account below minimum reserve. Account needs more XLM

[INFO] Tip: Add more XLM to your account to cover the transaction fee and maintain the minimum balance.
```

## Architecture

### Package Structure

```
internal/decoder/
├── result_codes.go       # Main decoder implementation
├── result_codes_test.go  # Comprehensive test suite
├── examples.go           # Usage examples
└── README.md            # This file
```

### Key Functions

- `DecodeTransactionResultCode(code)` - Decode transaction-level codes
- `DecodeOperationResultCode(code)` - Decode operation-level codes
- `DecodePaymentResultCode(code)` - Decode payment-specific codes
- `DecodeCreateAccountResultCode(code)` - Decode create account codes
- `FormatTransactionResult(result)` - Format complete transaction result
- `DecodeResultXDR(xdrString)` - Decode from base64 XDR string

## Testing

Run the test suite:

```bash
go test ./internal/decoder/...
```

Expected output:
```
PASS
ok      github.com/dotandev/hintents/internal/decoder    0.123s
```

## Future Enhancements

- [ ] Add more operation-specific decoders (ManageOffer, ChangeTrust, etc.)
- [ ] Include links to Stellar documentation for each error
- [ ] Add suggested fixes/remediation steps
- [ ] Support for Soroban-specific error codes
- [ ] Localization support for multiple languages

## References

- [Stellar XDR Documentation](https://developers.stellar.org/docs/encyclopedia/xdr)
- [Transaction Result Codes](https://developers.stellar.org/docs/encyclopedia/error-codes)
- [Stellar Go SDK](https://github.com/stellar/go)

## Contributing

When adding new error code mappings:

1. Add the mapping to the appropriate function in `result_codes.go`
2. Include a clear description and actionable explanation
3. Add test cases in `result_codes_test.go`
4. Update this README with the new codes
5. Run tests to ensure everything works

## License

Copyright 2025 Erst Users  
SPDX-License-Identifier: Apache-2.0
