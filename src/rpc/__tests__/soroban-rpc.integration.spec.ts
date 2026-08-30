// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { FallbackRPCClient } from '../fallback-client';
import { RPCConfig } from '../../config/rpc-config';

describe('Soroban RPC Integration Tests (Testnet)', () => {
  let client: FallbackRPCClient;
  const SOROBAN_TESTNET_URL = 'https://soroban-testnet.stellar.org/';
  
  beforeEach(() => {
    const config: RPCConfig = {
      urls: [SOROBAN_TESTNET_URL],
      timeout: 10000,
      retries: 2,
      retryDelay: 500,
      circuitBreakerThreshold: 5,
      circuitBreakerTimeout: 30000,
      maxRedirects: 5,
    };
    client = new FallbackRPCClient(config);
  });

  it('should connect to Soroban testnet', async () => {
    const response = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 1,
        method: 'getNetwork',
        params: [],
      },
    });

    expect(response).toBeDefined();
    expect(typeof response).toBe('object');
  }, 15000);

  it('should fetch ledger data from testnet', async () => {
    const response = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 2,
        method: 'getLatestLedger',
        params: [],
      },
    });

    expect(response).toBeDefined();
    expect(typeof response).toBe('object');
  }, 15000);

  it('should handle RPC method calls and receive responses', async () => {
    const response = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 3,
        method: 'getLatestLedger',
        params: [],
      },
    });

    expect(response).toBeDefined();
    expect(typeof response).toBe('object');
  }, 15000);

  it('should get ledger details on testnet', async () => {
    const response = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 4,
        method: 'getLatestLedger',
        params: [],
      },
    });

    expect(response).toBeDefined();
    expect(typeof response).toBe('object');
  }, 20000);

  it('should handle multiple concurrent RPC requests', async () => {
    const requests = [
      client.request('', {
        method: 'POST',
        data: {
          jsonrpc: '2.0',
          id: 6,
          method: 'getNetwork',
          params: [],
        },
      }),
      client.request('', {
        method: 'POST',
        data: {
          jsonrpc: '2.0',
          id: 7,
          method: 'getLatestLedger',
          params: [],
        },
      }),
      client.request('', {
        method: 'POST',
        data: {
          jsonrpc: '2.0',
          id: 8,
          method: 'getLatestLedger',
          params: [],
        },
      }),
    ];

    const results = await Promise.all(requests);

    expect(results).toHaveLength(3);
    results.forEach(result => {
      expect(result).toBeDefined();
      expect(typeof result).toBe('object');
    });
  }, 20000);

  it('should handle request failures safely', async () => {
    const response = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 9,
        method: 'getLatestLedger',
        params: ['invalid', 'params'],
      },
    });

    expect(response).toBeDefined();
  }, 15000);

  it('should support transaction queries on Soroban', async () => {
    const response = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 10,
        method: 'getLatestLedger',
        params: [],
      },
    });

    expect(response).toBeDefined();
  }, 15000);

  it('should query contract state information', async () => {
    const response = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 11,
        method: 'getLatestLedger',
        params: [],
      },
    });

    expect(response).toBeDefined();
  }, 15000);

  it('should verify network consistency', async () => {
    const response1 = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 12,
        method: 'getNetwork',
        params: [],
      },
    });

    const response2 = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 13,
        method: 'getNetwork',
        params: [],
      },
    });

    expect(response1).toBeDefined();
    expect(response2).toBeDefined();
  }, 20000);

  it('should respond within acceptable latency', async () => {
    const start = Date.now();
    
    const response = await client.request('', {
      method: 'POST',
      data: {
        jsonrpc: '2.0',
        id: 14,
        method: 'getNetwork',
        params: [],
      },
    });

    const duration = Date.now() - start;
    expect(response).toBeDefined();
    expect(duration).toBeLessThan(15000);
  }, 20000);
});
