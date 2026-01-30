## Description
This PR implements RPC authentication support, allowing the CLI to pass API keys or bearer tokens to Horizon/RPC providers that require authentication.

## Related Issue
Closes #64

## Changes Made

### Custom HTTP RoundTripper
- **internal/rpc/client.go**: Added `authTransport` struct
  - Implements `http.RoundTripper` interface
  - Injects `Authorization: Bearer <token>` header into every HTTP request
  - Only adds header when token is non-empty

### Updated RPC Client
- **internal/rpc/client.go**: Modified client initialization
  - `NewClient()` now accepts optional `token` parameter
  - `NewClientWithURL()` now accepts optional `token` parameter
  - Both functions check `ERST_RPC_TOKEN` environment variable if token not provided
  - Added `createHTTPClient()` helper function

### CLI Command Updates
- **internal/cmd/debug.go**: Added `--rpc-token` flag
- **internal/cmd/generate_test.go**: Added `--rpc-token` flag

### Unit Tests
- **internal/rpc/client_test.go**: Added comprehensive authentication tests
  - `TestAuthTransport`: Verifies Bearer token header is added correctly
  - `TestAuthTransportWithoutToken`: Verifies no header added when token is empty
  - `TestNewClientWithToken`: Verifies client initialization with token
  - `TestNewClientWithoutToken`: Verifies client initialization without token

### Documentation
- **docs/CLI.md**: Updated with `--rpc-token` flag and `ERST_RPC_TOKEN` environment variable documentation

## Usage Examples

```bash
# Using environment variable
export ERST_RPC_TOKEN="your-api-key-here"
erst debug <tx-hash>

# Using CLI flag
erst debug --rpc-token "your-api-key-here" <tx-hash>

# With custom RPC URL
erst debug --rpc-url "https://custom-rpc.example.com" --rpc-token "your-api-key" <tx-hash>
```

## Security

> [!IMPORTANT]
> **Token Security**: The token value is stored in the client struct but is NEVER logged in debug output to prevent accidental exposure.

Verification:
```bash
grep -n "token" internal/rpc/client.go | grep -i "logger\|log\|print"
# Result: Only shows the comment "// stored for reference, not logged"
```

## Testing

### Automated Tests
- Run `go test ./internal/rpc/...` to verify authentication behavior
- All tests pass and verify proper header injection

## Success Criteria Met
- ✅ `ERST_RPC_TOKEN` environment variable supported
- ✅ `--rpc-token` CLI flag supported
- ✅ Custom `http.RoundTripper` injects `Authorization` header
- ✅ Token value is NEVER logged in debug logs
- ✅ Unit tests verify authentication behavior
- ✅ Documentation updated
