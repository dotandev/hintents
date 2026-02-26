# Copyright (c) Hintents Authors.
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

# Copyright (c) 2026 dotandev
# SPDX-License-Identifier: MIT OR Apache-2.0

TX_HASH="a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef"
RPC_URL="https://horizon-testnet.stellar.org"

echo "=== BUILDING ==="
npm run build

echo ""
echo "=== STANDARD MODE ==="
node dist/index.js debug $TX_HASH --rpc $RPC_URL

echo ""
echo ""
echo "=== VERBOSE MODE ==="
node dist/index.js debug $TX_HASH --rpc $RPC_URL --verbose

echo ""
echo ""
echo "=== SIDE BY SIDE COMPARISON ==="
echo "Standard output (left) vs Verbose output (right)"
diff -y <(node dist/index.js debug $TX_HASH --rpc $RPC_URL 2>&1) <(node dist/index.js debug $TX_HASH --rpc $RPC_URL --verbose 2>&1) || true
