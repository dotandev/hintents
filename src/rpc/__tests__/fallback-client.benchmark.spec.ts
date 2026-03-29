// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { FallbackRPCClient } from '../fallback-client';
import { RPCConfig } from '../../config/rpc-config';
import axios from 'axios';
import MockAdapter from 'axios-mock-adapter';

describe('FallbackRPCClient Benchmarks', () => {
    let client: FallbackRPCClient;
    let mock: MockAdapter;

    const config: RPCConfig = {
        urls: ['https://rpc1.test.com'],
        timeout: 5000,
        retries: 0,
        retryDelay: 10,
        circuitBreakerThreshold: 3,
        circuitBreakerTimeout: 10000,
        maxRedirects: 5,
    };

    beforeEach(() => {
        mock = new MockAdapter(axios);
        client = new FallbackRPCClient(config);
    });

    afterEach(() => {
        mock.restore();
    });

    describe('Performance benchmarks', () => {
        it('should measure single request latency', async () => {
            mock.onPost('https://rpc1.test.com/test').reply(200, { success: true });

            const iterations = 100;
            const start = performance.now();
            
            for (let i = 0; i < iterations; i++) {
                await client.request('/test', {});
            }
            
            const end = performance.now();
            const avgTime = (end - start) / iterations;
            
            console.log(`Average single request: ${avgTime.toFixed(2)}ms`);
            expect(avgTime).toBeLessThan(50);
        });

        it('should benchmark batch request performance', async () => {
            mock.onPost('https://rpc1.test.com/rpc').reply(200, [
                { jsonrpc: '2.0', id: 1, result: { success: true } },
                { jsonrpc: '2.0', id: 2, result: { success: true } },
                { jsonrpc: '2.0', id: 3, result: { success: true } },
            ]);

            const requests = [
                { id: 1, method: 'getHealth', params: {} },
                { id: 2, method: 'getLatestLedger', params: {} },
                { id: 3, method: 'getNetwork', params: {} },
            ];

            const iterations = 50;
            const start = performance.now();
            
            for (let i = 0; i < iterations; i++) {
                await client.batchRequest(requests);
            }
            
            const end = performance.now();
            const avgTime = (end - start) / iterations;
            const throughput = (requests.length / avgTime) * 1000;
            
            console.log(`Batch request avg: ${avgTime.toFixed(2)}ms`);
            console.log(`Batch throughput: ${throughput.toFixed(2)} req/sec`);
            expect(avgTime).toBeLessThan(100);
        });

        it('should benchmark parallel requests', async () => {
            const endpoints = ['https://rpc1.test.com', 'https://rpc2.test.com'];
            mock.onPost(/\/test/).reply(200, { success: true });

            const requests = Array(10).fill(null).map((_, i) => ({
                path: `/test/${i}`,
                options: { method: 'POST' as const, data: { index: i } }
            }));

            const start = performance.now();
            await client.parallelRequests(requests, 5);
            const end = performance.now();
            const duration = end - start;
            const throughput = (requests.length / duration) * 1000;
            
            console.log(`Parallel (10 reqs, concurrency 5): ${duration.toFixed(2)}ms`);
            console.log(`Parallel throughput: ${throughput.toFixed(2)} req/sec`);
            expect(duration).toBeLessThan(5000);
        });

        it('should benchmark chunking performance', async () => {
            const wasmPaths = Array(1000).fill(null).map((_, i) => `/path/to/contract${i}.wasm`);
            
            const start = performance.now();
            
            const chunks = client['chunkStringSlice'](wasmPaths, 64);
            
            const end = performance.now();
            const time = end - start;
            
            console.log(`Chunking 1000 paths: ${time.toFixed(2)}ms, chunks: ${chunks.length}`);
            expect(chunks.length).toBe(16);
            expect(time).toBeLessThan(10);
        });

        it('should measure health status retrieval performance', async () => {
            const iterations = 1000;
            
            const start = performance.now();
            for (let i = 0; i < iterations; i++) {
                client.getHealthStatus();
            }
            const end = performance.now();
            const avgTime = (end - start) / iterations;
            
            console.log(`getHealthStatus avg: ${avgTime.toFixed(4)}ms`);
            expect(avgTime).toBeLessThan(1);
        });

        it('should measure circuit breaker performance', async () => {
            mock.onPost('https://rpc1.test.com/fail').networkError();
            mock.onPost('https://rpc1.test.com/success').reply(200, { ok: true });

            for (let i = 0; i < 3; i++) {
                try { await client.request('/fail', {}); } catch {}
            }

            const start = performance.now();
            try { await client.request('/success', {}); } catch {}
            const end = performance.now();

            console.log(`Circuit breaker check: ${end - start}ms`);
        });
    });

    describe('Memory benchmarks', () => {
        it('should not leak memory on repeated requests', async () => {
            mock.onPost('https://rpc1.test.com/test').reply(200, { data: 'x'.repeat(1000) });

            const initialMemory = process.memoryUsage().heapUsed;
            
            for (let i = 0; i < 500; i++) {
                await client.request('/test', {});
                client.getHealthStatus();
            }
            
            const finalMemory = process.memoryUsage().heapUsed;
            const memoryIncrease = (finalMemory - initialMemory) / 1024 / 1024;
            
            console.log(`Memory increase: ${memoryIncrease.toFixed(2)}MB`);
            expect(memoryIncrease).toBeLessThan(50);
        });

        it('should handle large batch requests efficiently', async () => {
            const largeResponse = Array(100).fill(null).map((_, i) => ({
                jsonrpc: '2.0', id: i, result: { value: i }
            }));
            mock.onPost('https://rpc1.test.com/rpc').reply(200, largeResponse);

            const requests = Array(100).fill(null).map((_, i) => ({
                id: i, method: 'getHealth', params: {}
            }));

            const start = performance.now();
            await client.batchRequest(requests);
            const end = performance.now();

            console.log(`100-request batch: ${end - start}ms`);
            expect(end - start).toBeLessThan(5000);
        });
    });

    describe('Type-safe method benchmarks', () => {
        it('should benchmark getHealth type-safe method', async () => {
            mock.onPost('https://rpc1.test.com/').reply(200, {
                jsonrpc: '2.0',
                id: 1,
                result: { status: 'healthy' }
            });

            const iterations = 100;
            const start = performance.now();
            
            for (let i = 0; i < iterations; i++) {
                await client.getHealth();
            }
            
            const end = performance.now();
            const avgTime = (end - start) / iterations;
            
            console.log(`getHealth() avg: ${avgTime.toFixed(2)}ms`);
            expect(avgTime).toBeLessThan(50);
        });

        it('should benchmark getTransaction type-safe method', async () => {
            mock.onPost('https://rpc1.test.com/').reply(200, {
                jsonrpc: '2.0',
                id: 1,
                result: { hash: 'abc123', successful: true }
            });

            const start = performance.now();
            await client.getTransaction('abc123');
            const end = performance.now();

            console.log(`getTransaction() avg: ${end - start}ms`);
            expect(end - start).toBeLessThan(50);
        });
    });
});
