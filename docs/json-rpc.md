# JSON-RPC API Documentation

The ERST daemon exposes a JSON-RPC 2.0 server for remote debugging capabilities.

## Starting the Daemon

```bash
# Basic usage
./erst daemon --port 8080

# With authentication
./erst daemon --port 8080 --auth-token secret123

# With tracing enabled
./erst daemon --port 8080 --tracing --otlp-url http://localhost:4318

# Custom network
./erst daemon --port 8080 --network testnet
```

## Endpoints

### Health Check
```
GET /health
```

Returns server health status.

### JSON-RPC Endpoint
```
POST /rpc
Content-Type: application/json
```

## RPC Methods

### debug_transaction

Debug a failed Stellar transaction.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "DebugTransaction",
  "params": {
    "hash": "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
  },
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "hash": "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab",
    "network": "mainnet",
    "envelope_size": 1024,
    "status": "success"
  },
  "id": 1
}
```

### get_trace

Get execution traces for a transaction.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "GetTrace",
  "params": {
    "hash": "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
  },
  "id": 2
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "hash": "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab",
    "traces": [
      {
        "span_id": "debug_transaction",
        "operation": "fetch_transaction", 
        "duration": "150ms",
        "status": "success"
      }
    ]
  },
  "id": 2
}
```

## Authentication

When `--auth-token` is provided, all RPC requests must include authentication:

```bash
# Bearer token format
curl -H "Authorization: Bearer secret123" ...

# Direct token format  
curl -H "Authorization: secret123" ...
```

## Error Responses

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32603,
    "message": "failed to fetch transaction: horizon error: \"Resource Missing\""
  },
  "id": 1
}
```

## Testing

Use the provided test script:

```bash
./test/rpc_test.sh 8080
```

Or test manually with curl:

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "DebugTransaction",
    "params": {"hash": "your-tx-hash"},
    "id": 1
  }'
```

## Integration Examples

### Python Client
```python
import requests

def debug_transaction(hash, host="localhost:8080", token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    
    payload = {
        "jsonrpc": "2.0",
        "method": "DebugTransaction", 
        "params": {"hash": hash},
        "id": 1
    }
    
    response = requests.post(f"http://{host}/rpc", 
                           json=payload, headers=headers)
    return response.json()
```

### JavaScript Client
```javascript
async function debugTransaction(hash, host = "localhost:8080", token = null) {
  const headers = {"Content-Type": "application/json"};
  if (token) headers["Authorization"] = `Bearer ${token}`;
  
  const response = await fetch(`http://${host}/rpc`, {
    method: "POST",
    headers,
    body: JSON.stringify({
      jsonrpc: "2.0",
      method: "DebugTransaction",
      params: {hash},
      id: 1
    })
  });
  
  return response.json();
}
```
