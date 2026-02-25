// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { FallbackRPCClient } from '../fallback-client';
import { RPCConfig } from '../../config/rpc-config';
import axios from 'axios';
import MockAdapter from 'axios-mock-adapter';

describe('FallbackRPCClient', () => {
    let client: FallbackRPCClient;
    let mock: MockAdapter;

    const config: RPCConfig = {
        urls: [
            'https://rpc1.test.com',
            'https://rpc2.test.com',
            'https://rpc3.test.com',
        ],
        timeout: 5000,
        retries: 2, // Fewer retries for faster tests
        retryDelay: 10,
        circuitBreakerThreshold: 3,
        circuitBreakerTimeout: 10000,
        maxRedirects: 5,
    };

    beforeEach(() => {
        // Clear mock registry
        mock = new MockAdapter(axios);
        client = new FallbackRPCClient(config);
    });

    afterEach(() => {
        mock.restore();
    });

    describe('request with fallback', () => {
        it('should succeed with primary RPC', async () => {
            mock.onPost('https://rpc1.test.com/test').reply(200, { success: true });

            const result = await client.request('/test', {});

            expect(result).toEqual({ success: true });
        });

        it('should fallback to second RPC when first fails', async () => {
            mock.onPost('https://rpc1.test.com/test').networkError();
            mock.onPost('https://rpc2.test.com/test').reply(200, { success: true });

            const result = await client.request('/test', {});

            expect(result).toEqual({ success: true });
        });

        it('should fallback through all RPCs until success', async () => {
            mock.onPost('https://rpc1.test.com/test').networkError();
            mock.onPost('https://rpc2.test.com/test').networkError();
            mock.onPost('https://rpc3.test.com/test').reply(200, { success: true });

            const result = await client.request('/test', {});

            expect(result).toEqual({ success: true });
        });

        it('should throw error when all RPCs fail', async () => {
            mock.onPost(/.*/).networkError();

            await expect(client.request('/test', {})).rejects.toThrow('All RPC endpoints failed');
        });

        it('should handle timeout errors (ECONNABORTED)', async () => {
            mock.onPost('https://rpc1.test.com/test').timeout();
            mock.onPost('https://rpc2.test.com/test').reply(200, { success: true });

            const result = await client.request('/test', {});

            expect(result).toEqual({ success: true });
        });

        it('should handle HTTP 500 errors as retryable', async () => {
            mock.onPost('https://rpc1.test.com/test').reply(500);
            mock.onPost('https://rpc2.test.com/test').reply(200, { success: true });

            const result = await client.request('/test', {});

            expect(result).toEqual({ success: true });
        });

        it('should not retry HTTP 400 errors', async () => {
            mock.onPost('https://rpc1.test.com/test').reply(400, { error: 'Bad request' });

            await expect(client.request('/test', {})).rejects.toThrow();
            // Verify it didn't call the second RPC
            expect(mock.history.post.length).toBe(1);
        });
    });

    describe('circuit breaker', () => {
        it('should open circuit after threshold failures', async () => {
            mock.onPost('https://rpc1.test.com/test').networkError();
            mock.onPost('https://rpc2.test.com/test').reply(200, { success: true });

            // Trigger failures to open circuit (threshold is 3)
            // Each request will try RPC1 (fail) then RPC2 (success)
            for (let i = 0; i < 3; i++) {
                await client.request('/test', {});
            }

            const status = client.getHealthStatus();
            expect(status[0].circuitOpen).toBe(true);

            // Next request should skip RPC1 entirely
            mock.onPost('https://rpc2.test.com/test').reply(200, { success: 'skipped rpc1' });
            const result = await client.request('/test', {});
            expect(result.success).toBe('skipped rpc1');

            // Check history - RPC1 should not have been called in the last request
            // Total post calls so far: 3 (from loop) * 2 (rpc1+rpc2) + 1 (last request rpc2 only) = 7
            // Wait, actually my loop does 1 rpc1 and 1 rpc2. 
            // So 6 calls in loop. 1 call after. Total 7.
            expect(mock.history.post.find(c => c.url === 'https://rpc1.test.com/test' && mock.history.post.indexOf(c) === 6)).toBeUndefined();
        });
    });

    describe('metrics', () => {
        it('should track success and failure metrics', async () => {
            mock.onPost('https://rpc1.test.com/success').reply(200);
            mock.onPost('https://rpc1.test.com/fail').networkError();
            mock.onPost('https://rpc2.test.com/fail').networkError();

            await client.request('/success', {});
            try { await client.request('/fail', {}); } catch { }

            const status = client.getHealthStatus();
            expect(status[0].metrics.totalSuccess).toBe(1);
            expect(status[0].metrics.totalFailure).toBe(1);
            expect(status[1].metrics.totalFailure).toBe(1);
        });
    });

    describe('health checks', () => {
        it('should check health of all endpoints', async () => {
            mock.onPost('https://rpc1.test.com/', { jsonrpc: '2.0', id: 1, method: 'getHealth' }).reply(200, { result: { status: 'healthy' } });
            mock.onPost('https://rpc2.test.com/', { jsonrpc: '2.0', id: 1, method: 'getHealth' }).reply(200, { result: { status: 'healthy' } });
            mock.onPost('https://rpc3.test.com/', { jsonrpc: '2.0', id: 1, method: 'getHealth' }).reply(200, { result: { status: 'unhealthy' } });

            await client.performHealthChecks();

            const status = client.getHealthStatus();
            expect(status[0].healthy).toBe(true);
            expect(status[1].healthy).toBe(true);
            expect(status[2].healthy).toBe(false);
        });
    });
});
