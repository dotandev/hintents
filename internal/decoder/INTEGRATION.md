# Integration Example: Using Decoder in CLI

## How to integrate the decoder into the `erst debug` command

When the CLI receives a failed transaction result, it should use the decoder to show human-readable errors instead of raw codes.

### Before (Raw XDR codes):
```
Transaction failed with code: -1
Operation 0 failed with code: -2
```

### After (Human-readable):
```
Transaction Result: Transaction Failed (tx_failed)
Explanation: One or more operations failed. Check individual operation results for details

Operation Results:
  Operation 0: Insufficient Funds (payment_underfunded)
    Source account doesn't have enough of the asset to send

[INFO] Tip: Check your account balance and ensure you have enough of the asset to complete this payment.
```

## Integration Code Example

```go
// In your debug command (e.g., internal/cmd/debug.go or wherever you process results)

import (
    "github.com/dotandev/hintents/internal/decoder"
    "github.com/stellar/go/xdr"
)

func displayTransactionResult(resultXDR string) error {
    // Decode the XDR
    data, err := base64.StdEncoding.DecodeString(resultXDR)
    if err != nil {
        return err
    }
    
    var result xdr.TransactionResult
    if err := xdr.SafeUnmarshal(data, &result); err != nil {
        return err
    }
    
    // Use the decoder to format human-readable output
    formatted := decoder.FormatTransactionResult(result)
    fmt.Println(formatted)
    
    // Optionally add helpful tips based on the error
    if result.Result.Code == xdr.TransactionResultCodeTxInsufficientBalance {
        fmt.Println("\n[INFO] Tip: Add more XLM to your account to cover fees and maintain minimum balance.")
    }
    
    return nil
}
```

## Quick Test

You can test the decoder right now:

```bash
# From the project root
go run internal/decoder/examples.go
```

This will show example output for common error scenarios.
