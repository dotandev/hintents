# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0


# Test script for JSON-RPC daemon

PORT=${1:-8080}
HOST="localhost:$PORT"

echo "Testing ERST JSON-RPC daemon on $HOST"

# Test health endpoint
echo "1. Testing health endpoint..."
curl -s "http://$HOST/health" | jq .

echo -e "\n2. Testing debug_transaction endpoint..."
curl -s -X POST "http://$HOST/rpc" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "DebugTransaction",
    "params": {"hash": "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"},
    "id": 1
  }' | jq .

echo -e "\n3. Testing get_trace endpoint..."
curl -s -X POST "http://$HOST/rpc" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "GetTrace", 
    "params": {"hash": "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"},
    "id": 2
  }' | jq .

echo -e "\n4. Testing with authentication (if enabled)..."
curl -s -X POST "http://$HOST/rpc" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer secret123" \
  -d '{
    "jsonrpc": "2.0",
    "method": "DebugTransaction",
    "params": {"hash": "test-hash"},
    "id": 3
  }' | jq .

echo -e "\nDone!"
