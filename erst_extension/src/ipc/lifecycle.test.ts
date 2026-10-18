// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

/**
 * Tests for IPC Connection Lifecycle Management
 *
 * Tests the retry mechanism with exponential backoff for handling
 * the race condition between DAP Attach/Launch and simulator startup.
 */

import * as assert from 'assert';
import * as net from 'net';
import { connectWithRetry, testConnection } from './lifecycle';

describe('IPC Lifecycle - Connection Retry', () => {
    /**
     * Helper to create a server that accepts connections after a delay
     */
    function createDelayedServer(delayMs: number, port: number): Promise<net.Server> {
        return new Promise((resolve) => {
            setTimeout(() => {
                const server = net.createServer((socket) => {
                    socket.write('OK');
                    socket.end();
                });
                server.listen(port, '127.0.0.1', () => {
                    resolve(server);
                });
            }, delayMs);
        });
    }

    /**
     * Helper to create a server that rejects all connections
     */
    function createRejectingServer(port: number): net.Server {
        const server = net.createServer((socket) => {
            socket.destroy();
        });
        server.listen(port, '127.0.0.1');
        return server;
    }

    describe('connectWithRetry', () => {
        it('should connect immediately when server is ready', async () => {
            const port = 9001;
            const server = net.createServer((socket) => {
                socket.end();
            });

            await new Promise<void>((resolve) => {
                server.listen(port, '127.0.0.1', () => resolve());
            });

            try {
                const socket = await connectWithRetry('127.0.0.1', port, {
                    maxAttempts: 1,
                });
                socket.end();
                assert.ok(true, 'Connected successfully on first attempt');
            } finally {
                server.close();
            }
        });

        it('should retry and connect after server startup delay', async () => {
            const port = 9002;
            const startTime = Date.now();

            // Server starts after 50ms
            const serverPromise = createDelayedServer(50, port);

            try {
                const socket = await connectWithRetry('127.0.0.1', port, {
                    maxAttempts: 10,
                    initialDelayMs: 10,
                    maxDelayMs: 100,
                    backoffMultiplier: 2,
                });
                const elapsed = Date.now() - startTime;
                socket.end();

                // Should have taken at least 50ms (the server delay)
                assert.ok(
                    elapsed >= 40,
                    `Should wait for server startup, took ${elapsed}ms`
                );
            } finally {
                const server = await serverPromise;
                server.close();
            }
        }).timeout(5000);

        it('should fail after exhausting all retries', async () => {
            const port = 9003;

            try {
                await connectWithRetry('127.0.0.1', port, {
                    maxAttempts: 3,
                    initialDelayMs: 5,
                    maxDelayMs: 20,
                    backoffMultiplier: 2,
                });
                assert.fail('Should have thrown an error');
            } catch (err: any) {
                assert.ok(
                    err.message.includes('Failed to connect'),
                    `Error message should mention connection failure: ${err.message}`
                );
                assert.ok(
                    err.message.includes('3 attempts'),
                    `Error message should mention attempt count`
                );
            }
        }).timeout(5000);

        it('should use custom retry configuration', async () => {
            const port = 9004;
            const startTime = Date.now();

            // Server starts after 30ms
            const serverPromise = createDelayedServer(30, port);

            try {
                const socket = await connectWithRetry('127.0.0.1', port, {
                    maxAttempts: 20,
                    initialDelayMs: 1,
                    maxDelayMs: 50,
                    backoffMultiplier: 1.5, // Slower backoff
                    jitterFactor: 0.1, // Less jitter
                });
                const elapsed = Date.now() - startTime;
                socket.end();

                // Should complete with custom config
                assert.ok(
                    elapsed >= 20,
                    `Should wait for server startup, took ${elapsed}ms`
                );
            } finally {
                const server = await serverPromise;
                server.close();
            }
        }).timeout(5000);

        it('should handle multiple consecutive connections', async () => {
            const port = 9005;
            const server = net.createServer((socket) => {
                socket.end();
            });

            await new Promise<void>((resolve) => {
                server.listen(port, '127.0.0.1', () => resolve());
            });

            try {
                // Try multiple connections
                for (let i = 0; i < 3; i++) {
                    const socket = await connectWithRetry('127.0.0.1', port, {
                        maxAttempts: 3,
                    });
                    socket.end();
                }
                assert.ok(true, 'Multiple connections succeeded');
            } finally {
                server.close();
            }
        });

        it('should handle jitter in backoff calculation', async () => {
            // This test verifies that jitter is applied without causing errors
            const port = 9006;
            const delays: number[] = [];

            // Connect to a server that starts after 100ms
            const serverPromise = createDelayedServer(100, port);

            try {
                const socket = await connectWithRetry('127.0.0.1', port, {
                    maxAttempts: 15,
                    initialDelayMs: 10,
                    maxDelayMs: 200,
                    backoffMultiplier: 1.5,
                    jitterFactor: 0.3, // ┬▒30% jitter
                });
                socket.end();
                assert.ok(true, 'Connected with jitter applied');
            } finally {
                const server = await serverPromise;
                server.close();
            }
        }).timeout(5000);

        it('should respect maxDelayMs cap in backoff', async () => {
            // Verify that exponential backoff doesn't exceed maxDelayMs
            const port = 9007;
            const startTime = Date.now();

            const serverPromise = createDelayedServer(50, port);

            try {
                const socket = await connectWithRetry('127.0.0.1', port, {
                    maxAttempts: 20,
                    initialDelayMs: 10,
                    maxDelayMs: 30, // Should cap backoff at 30ms
                    backoffMultiplier: 10, // Aggressive multiplier
                    jitterFactor: 0,
                });
                const elapsed = Date.now() - startTime;
                socket.end();

                // Total time should be reasonable (roughly: 50ms server delay + few retries with capped delays)
                // Without the cap, delays would be: 10, 100, 1000, ... which would take too long
                assert.ok(
                    elapsed < 2000,
                    `Should respect maxDelayMs cap, took ${elapsed}ms`
                );
            } finally {
                const server = await serverPromise;
                server.close();
            }
        }).timeout(5000);
    });

    describe('testConnection', () => {
        it('should return true for successful connection', async () => {
            const port = 9008;
            const server = net.createServer();

            await new Promise<void>((resolve) => {
                server.listen(port, '127.0.0.1', () => resolve());
            });

            try {
                const result = await testConnection('127.0.0.1', port);
                assert.strictEqual(result, true, 'Should return true for successful connection');
            } finally {
                server.close();
            }
        });

        it('should return false for failed connection', async () => {
            const port = 9009; // Likely no server on this port
            const result = await testConnection('127.0.0.1', port, 100);
            assert.strictEqual(result, false, 'Should return false for failed connection');
        });

        it('should respect timeout', async () => {
            const port = 9010;
            const startTime = Date.now();

            const result = await testConnection('127.0.0.1', port, 50);
            const elapsed = Date.now() - startTime;

            assert.strictEqual(result, false, 'Should return false on timeout');
            assert.ok(
                elapsed >= 40 && elapsed < 200,
                `Should timeout around 50ms, took ${elapsed}ms`
            );
        });
    });

    describe('Backoff Calculation', () => {
        // These tests are integration tests that verify the retry logic
        // works correctly with realistic scenarios

        it('should handle rapid retries efficiently', async () => {
            const port = 9011;
            const startTime = Date.now();

            // Server starts after 20ms
            const serverPromise = createDelayedServer(20, port);

            try {
                const socket = await connectWithRetry('127.0.0.1', port, {
                    maxAttempts: 50,
                    initialDelayMs: 5,
                    maxDelayMs: 100,
                    backoffMultiplier: 1.5,
                    jitterFactor: 0.1,
                });
                const elapsed = Date.now() - startTime;
                socket.end();

                // Should connect much faster than exhausting all retries
                assert.ok(
                    elapsed < 500,
                    `Should connect efficiently, took ${elapsed}ms`
                );
            } finally {
                const server = await serverPromise;
                server.close();
            }
        }).timeout(5000);

        it('should handle worst-case scenario (long startup time)', async () => {
            const port = 9012;
            const startTime = Date.now();

            // Server starts after 200ms
            const serverPromise = createDelayedServer(200, port);

            try {
                const socket = await connectWithRetry('127.0.0.1', port, {
                    maxAttempts: 30,
                    initialDelayMs: 20,
                    maxDelayMs: 300,
                    backoffMultiplier: 2,
                    jitterFactor: 0.2,
                });
                const elapsed = Date.now() - startTime;
                socket.end();

                // Should successfully connect despite long startup
                assert.ok(
                    elapsed >= 180,
                    `Should wait for server, took ${elapsed}ms`
                );
                assert.ok(
                    elapsed < 2000,
                    `Should connect without excessive delay, took ${elapsed}ms`
                );
            } finally {
                const server = await serverPromise;
                server.close();
            }
        }).timeout(5000);
    });
});
