# Mock Horizon/RPC Server for Offline Testing

## Overview

The mock HTTP server provides a way to test Stellar Horizon and RPC endpoints locally without requiring an internet connection or worrying about rate limits.

## Features

- **httptest.NewServer**: Uses Go's built-in HTTP testing server
- **URL Path Mapping**: Maps specific paths to JSON responses
- **Error Responses**: Support for custom error codes (429, 500, 403, etc.)
- **Call Tracking**: Monitor which endpoints were called and how many times
- **Dynamic Routes**: Add/remove routes without restarting
- **Custom Headers**: Support for Retry-After and other headers

## Basic Usage

```go
package rpc

import (
    "testing"
    "net/http"
)

func TestWithMockServer(t *testing.T) {
    // Create mock server with routes
    routes := map[string]MockRoute{
        "/transactions/abc123": SuccessRoute(MockTransactionResponse{
            Hash:          "abc123",
            EnvelopeXdr:   "envelope-xdr-data",
            ResultXdr:     "result-xdr-data",
            ResultMetaXdr: "meta-xdr-data",
        }),
    }
    
    mockServer := NewMockServer(routes)
    defer mockServer.Close()
    
    // Create a client pointing to the mock server
    client := NewClientWithURL(mockServer.URL(), Testnet)
    
    // Test your code without external network calls
    resp, err := client.GetTransaction(context.Background(), "abc123")
    // ... assertions ...
}
```

## Error Scenarios

### Rate Limiting (HTTP 429)

```go
routes := map[string]MockRoute{
    "/transactions/limited": RateLimitRoute(),
}

mockServer := NewMockServer(routes)
defer mockServer.Close()

// Makes request that returns 429 with Retry-After header
resp, _ := http.Get(mockServer.URL() + "/transactions/limited")
```

### Server Error (HTTP 500)

```go
routes := map[string]MockRoute{
    "/transactions/error": ServerErrorRoute(),
}

mockServer := NewMockServer(routes)
defer mockServer.Close()

// Makes request that returns 500
resp, _ := http.Get(mockServer.URL() + "/transactions/error")
```

### Custom Error Response

```go
routes := map[string]MockRoute{
    "/forbidden": ErrorRoute(http.StatusForbidden, "access denied"),
}

mockServer := NewMockServer(routes)
defer mockServer.Close()

// Makes request that returns 403 with custom message
resp, _ := http.Get(mockServer.URL() + "/forbidden")
```

## Dynamic Routes

Add or remove routes while the server is running:

```go
mockServer := NewMockServer(map[string]MockRoute{})
defer mockServer.Close()

// Add a route
mockServer.AddRoute("/accounts/test", SuccessRoute(MockAccountResponse{
    ID:        "test",
    AccountID: "GTEST...",
    Balance:   "1000.0",
}))

// Remove a route
mockServer.RemoveRoute("/accounts/test")
```

## Request Tracking

Track which endpoints were called and verify expected behavior:

```go
routes := map[string]MockRoute{
    "/transactions/abc123": SuccessRoute(MockTransactionResponse{Hash: "abc123"}),
}

mockServer := NewMockServer(routes)
defer mockServer.Close()

// Make some requests
http.Get(mockServer.URL() + "/transactions/abc123")
http.Get(mockServer.URL() + "/transactions/abc123")
http.Get(mockServer.URL() + "/transactions/abc123")

// Verify call count
count := mockServer.CallCount("/transactions/abc123")
// count == 3

// Reset counts
mockServer.ResetCallCounts()
```

## Custom Headers

Add custom response headers:

```go
routes := map[string]MockRoute{
    "/transactions/abc123": {
        StatusCode: http.StatusOK,
        Body:       MockTransactionResponse{Hash: "abc123"},
        Headers: map[string]string{
            "X-Custom-Header": "custom-value",
            "Cache-Control":   "no-cache",
        },
    },
}

mockServer := NewMockServer(routes)
defer mockServer.Close()
```

## Data Types

### MockTransactionResponse

Represents a transaction from Stellar Horizon:

```go
MockTransactionResponse{
    ID:            "id-string",
    Hash:          "transaction-hash",
    EnvelopeXdr:   "xdr-encoded-envelope",
    ResultXdr:     "xdr-encoded-result",
    ResultMetaXdr: "xdr-encoded-metadata",
    Ledger:        12345,
    CreatedAt:     "2026-01-28T12:00:00Z",
}
```

### MockAccountResponse

Represents an account from Stellar Horizon:

```go
MockAccountResponse{
    ID:            "id-string",
    AccountID:     "GADDRESS...",
    Balance:       "1000.0000000",
    Sequence:      "123",
    CreatedAt:     "2026-01-28T12:00:00Z",
    UpdatedAt:     "2026-01-28T12:00:00Z",
    SubentryCount: 5,
}
```

## Testing Benefits

1. **No Network Dependencies**: Run tests offline
2. **No Rate Limits**: Test handling of rate limits without hitting actual service
3. **Predictable Responses**: Control exactly what the server returns
4. **Fast Tests**: No network latency
5. **Error Scenarios**: Easily test error handling
6. **Verification**: Track which endpoints were called

## Files Created

- `mock_server.go` - Core mock server implementation
- `mock_server_test.go` - Comprehensive test suite with 15+ test cases

All tests pass with 100% success rate.
