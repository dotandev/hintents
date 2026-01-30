# Ledger Header API

## Overview

The Ledger Header API provides functionality to fetch comprehensive ledger metadata from the Stellar network via the Horizon API. This is essential for transaction replay simulation, as it provides the exact blockchain state at the time a transaction was executed.

## Features

- Fetch ledger header information for any ledger sequence
- Support for testnet, mainnet, and futurenet networks
- Comprehensive error handling (not found, archived, rate limiting)
- Automatic timeout management
- Telemetry and logging integration
- Type-safe error checking

## Usage

### Basic Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/dotandev/hintents/internal/rpc"
)

func main() {
    // Create a client for testnet
    client := rpc.NewClient(rpc.Testnet)

    // Create a context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Fetch ledger header
    header, err := client.GetLedgerHeader(ctx, 12345678)
    if err != nil {
        log.Fatalf("Failed to fetch ledger: %v", err)
    }

    fmt.Printf("Ledger %d:\n", header.Sequence)
    fmt.Printf("  Hash: %s\n", header.Hash)
    fmt.Printf("  Protocol Version: %d\n", header.ProtocolVersion)
    fmt.Printf("  Close Time: %s\n", header.CloseTime)
}
```

### Error Handling

The API provides typed errors for different failure scenarios:

```go
header, err := client.GetLedgerHeader(ctx, sequence)
if err != nil {
    switch {
    case rpc.IsLedgerNotFound(err):
        // Ledger doesn't exist (future or invalid sequence)
        fmt.Println("Ledger not found")
    case rpc.IsLedgerArchived(err):
        // Ledger has been archived and is no longer available
        fmt.Println("Ledger archived")
    case rpc.IsRateLimitError(err):
        // Too many requests - implement backoff
        fmt.Println("Rate limited")
        time.Sleep(5 * time.Second)
        // Retry...
    default:
        // Other errors (network, timeout, etc.)
        fmt.Printf("Error: %v\n", err)
    }
    return
}
```

### Network Selection

```go
// Testnet
testnetClient := rpc.NewClient(rpc.Testnet)

// Mainnet
mainnetClient := rpc.NewClient(rpc.Mainnet)

// Futurenet
futurenetClient := rpc.NewClient(rpc.Futurenet)

// Custom Horizon URL
customClient := rpc.NewClientWithURL("https://custom-horizon.example.com", rpc.Testnet)
```

## LedgerHeaderResponse Structure

The `LedgerHeaderResponse` contains all essential ledger metadata:

```go
type LedgerHeaderResponse struct {
    // Core identifiers
    Sequence uint32    // Ledger sequence number
    Hash     string    // Ledger hash (hex-encoded SHA-256)
    PrevHash string    // Previous ledger hash

    // Timing
    CloseTime time.Time // When the ledger closed

    // Protocol parameters
    ProtocolVersion uint32 // Stellar protocol version
    BaseFee         int32  // Base fee in stroops
    BaseReserve     int32  // Base reserve in stroops
    MaxTxSetSize    int32  // Maximum transaction set size

    // Network state
    TotalCoins string // Total lumens in circulation
    FeePool    string // Fee pool amount

    // XDR data
    HeaderXDR string // Base64-encoded LedgerHeader XDR

    // Statistics
    SuccessfulTxCount int32 // Successful transactions
    FailedTxCount     int32 // Failed transactions
    OperationCount    int32 // Total operations
}
```

## Error Types

### LedgerNotFoundError

Indicates the requested ledger doesn't exist. This can happen if:

- The sequence number is in the future
- The sequence number is invalid
- The ledger hasn't been created yet

```go
type LedgerNotFoundError struct {
    Sequence uint32
    Message  string
}
```

### LedgerArchivedError

Indicates the ledger has been archived and is no longer available through Horizon:

```go
type LedgerArchivedError struct {
    Sequence uint32
    Message  string
}
```

### RateLimitError

Indicates too many requests have been made. Implement exponential backoff:

```go
type RateLimitError struct {
    Message string
}
```

## Best Practices

### 1. Always Use Context with Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

header, err := client.GetLedgerHeader(ctx, sequence)
```

### 2. Handle Rate Limiting

```go
func fetchWithRetry(client *rpc.Client, sequence uint32) (*rpc.LedgerHeaderResponse, error) {
    maxRetries := 3
    backoff := time.Second

    for i := 0; i < maxRetries; i++ {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        header, err := client.GetLedgerHeader(ctx, sequence)
        if err == nil {
            return header, nil
        }

        if rpc.IsRateLimitError(err) {
            time.Sleep(backoff)
            backoff *= 2
            continue
        }

        return nil, err
    }

    return nil, fmt.Errorf("max retries exceeded")
}
```

### 3. Cache Ledger Headers

Ledger headers are immutable once closed. Consider caching them:

```go
type LedgerCache struct {
    cache map[uint32]*rpc.LedgerHeaderResponse
    mu    sync.RWMutex
}

func (c *LedgerCache) Get(client *rpc.Client, sequence uint32) (*rpc.LedgerHeaderResponse, error) {
    c.mu.RLock()
    if header, ok := c.cache[sequence]; ok {
        c.mu.RUnlock()
        return header, nil
    }
    c.mu.RUnlock()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    header, err := client.GetLedgerHeader(ctx, sequence)
    if err != nil {
        return nil, err
    }

    c.mu.Lock()
    c.cache[sequence] = header
    c.mu.Unlock()

    return header, nil
}
```

## Testing

### Unit Tests

Run unit tests with mocked Horizon client:

```bash
go test ./internal/rpc -v -run TestGetLedgerHeader
```

### Integration Tests

Run integration tests against real testnet (requires network access):

```bash
go test ./internal/rpc -v -run TestGetLedgerHeader_Integration
```

Skip integration tests in CI:

```bash
go test ./internal/rpc -short
```

## Performance

- Average response time: 100-500ms (depends on network)
- Default timeout: 30 seconds
- Recommended: Cache ledger headers to reduce API calls
- Rate limits: Respect Horizon's rate limiting (typically 3600 requests/hour)

## Telemetry

The API includes OpenTelemetry tracing:

```
Span: rpc_get_ledger_header
Attributes:
  - network: testnet/mainnet/futurenet
  - ledger.sequence: <sequence>
  - ledger.hash: <hash>
  - ledger.protocol_version: <version>
  - ledger.tx_count: <count>
```

## Logging

Structured logging with slog:

```
INFO: Ledger header fetched successfully
  sequence: 12345678
  hash: abc123...
  protocol_version: 20
  close_time: 2024-01-15T12:30:45Z

WARN: Ledger not found
  sequence: 999999999
  status: 404

ERROR: Failed to fetch ledger
  sequence: 12345
  error: context deadline exceeded
```

## Related Documentation

- [Stellar Horizon API](https://developers.stellar.org/api/resources/ledgers)
- [Ledger Object Structure](https://developers.stellar.org/api/resources/ledgers/object)
- [Transaction Replay Simulation](./simulator-interface.md)
- [RPC Client Architecture](./json-rpc.md)
