# RPC Fallback Configuration

## Overview

ERST supports multiple RPC endpoints with automatic failover. If the primary RPC fails, the CLI automatically tries backup URLs with exponential backoff and localized retries.

## Configuration

### CLI Flag

```bash
erst debug <tx> --rpc https://rpc1.com,https://rpc2.com,https://rpc3.com
```

### Environment Variable

```bash
export STELLAR_RPC_URLS=https://rpc1.com,https://rpc2.com,https://rpc3.com
erst debug <tx>
```

### Options

| Option | Command Flag | Default | Description |
|--------|--------------|---------|-------------|
| Timeout | `--timeout` | 30000ms | Request timeout for each attempt |
| Retries | `--retries` | 3 | Number of local retries per endpoint |

## Fallback Behavior

1. **Primary First**: Always tries the first URL in the list for a new request session.
2. **Exponential Backoff**: If a request fails, it retries locally with increasing delays ($delay = base * 2^{attempt}$).
3. **Automatic Failover**: If an endpoint exceeds its retries, the client automatically switches to the next healthy URL.
4. **Circuit Breaker**: If an endpoint fails too many times (default: 5), it is marked as "circuit open" and skipped for 60 seconds.
5. **Return to Primary**: After a successful request, the client resets to start from the primary URL for the next operation.

## Health Checks

Check status and performance metrics of all configured RPC endpoints:

```bash
erst rpc:health --rpc https://rpc1.com,https://rpc2.com
```

### Status Output Example

```
📊 RPC Endpoint Status:

  [1] ✅ https://rpc1.com
      Success Rate: 100.0% (12/12)
      Avg Duration: 245ms
      Failures: 0

  [2] ❌ https://rpc2.com [CIRCUIT OPEN]
      Success Rate: 0.0% (0/5)
      Avg Duration: 3000ms
      Failures: 5
```

## Error Handling

### Retryable Errors
- Generic Network Errors (DNS, Connection Refused)
- Axios Timeout (`ECONNABORTED`, `ETIMEDOUT`)
- HTTP 5xx Server Errors
- HTTP 429 Rate Limiting

### Non-Retryable Errors
- HTTP 4xx Errors (except 429) - These are usually client-side issues that switching RPCs won't fix.

## Troubleshooting

### All Endpoints Failing
- Check your internet connection.
- Verify the RPC URLs are reachable (use `rpc:health`).
- Check if your RPC providers require specific headers.

### Slow Failover
- Reduce the `--timeout` value to detect failures faster.
- Lower the `--retries` value to switch endpoints sooner.

### Circuit Breaker Always Open
- This indicates the RPC provider is consistently failing. Check their status page or try a different provider.
