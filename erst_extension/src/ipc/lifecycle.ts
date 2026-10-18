// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

/**
 * IPC Connection Lifecycle Management
 *
 * This module provides retry logic for establishing connections to the ERST
 * simulator's IPC socket during the DAP Attach and Launch sequences. It
 * implements exponential backoff with jitter to handle the race condition
 * where the DAP request arrives before the simulator has finished binding
 * its socket.
 *
 * # The Race Condition
 *
 * When debugging rapidly:
 * 1. Extension sends `launch` or `attach` request to DAP adapter
 * 2. DAP adapter immediately tries to connect to simulator IPC socket
 * 3. Simulator is still starting up / binding socket
 * 4. Connection fails with ECONNREFUSED
 *
 * # The Solution
 *
 * This module provides `connectWithRetry()` which:
 * 1. Attempts immediate connection
 * 2. On failure, waits with exponential backoff: 10ms, 20ms, 40ms, ...
 * 3. Retries up to a configurable number of times
 * 4. Adds random jitter (┬▒25%) to prevent thundering herd
 * 5. Returns when connection succeeds or retries exhausted
 */

import * as net from 'net';

/**
 * Configuration for connection retry behavior
 */
export interface ConnectionRetryConfig {
    /** Maximum number of retry attempts (including initial attempt) */
    maxAttempts?: number;
    
    /** Initial retry delay in milliseconds */
    initialDelayMs?: number;
    
    /** Maximum retry delay in milliseconds (caps exponential backoff) */
    maxDelayMs?: number;
    
    /** Multiplier for exponential backoff (e.g., 2 = doubling delay each time) */
    backoffMultiplier?: number;
    
    /** Jitter factor as a percentage (e.g., 0.25 = ┬▒25% variance) */
    jitterFactor?: number;
}

/**
 * Default configuration values
 */
const DEFAULT_CONFIG: Required<ConnectionRetryConfig> = {
    maxAttempts: 15,           // ~3 seconds total with exponential backoff
    initialDelayMs: 10,        // Start with 10ms
    maxDelayMs: 500,           // Cap at 500ms
    backoffMultiplier: 2,      // Double the delay each retry
    jitterFactor: 0.25,        // ┬▒25% jitter
};

/**
 * Attempts to connect to the IPC socket with exponential backoff retry logic.
 *
 * This function handles the race condition where a DAP Attach/Launch request
 * arrives before the simulator has finished binding its IPC socket. It retries
 * with exponential backoff and jitter to eventually connect or fail gracefully.
 *
 * @param host - The host to connect to (e.g., '127.0.0.1')
 * @param port - The port to connect to (e.g., 8080)
 * @param config - Optional retry configuration
 * @returns A promise that resolves to the connected socket or rejects with error
 *
 * @example
 * ```typescript
 * const socket = await connectWithRetry('127.0.0.1', 8080);
 * // Use socket...
 * socket.end();
 * ```
 */
export async function connectWithRetry(
    host: string,
    port: number,
    config?: ConnectionRetryConfig
): Promise<net.Socket> {
    const finalConfig = { ...DEFAULT_CONFIG, ...config };
    let lastError: Error | null = null;

    for (let attempt = 0; attempt < finalConfig.maxAttempts; attempt++) {
        try {
            return await attemptConnection(host, port);
        } catch (err: any) {
            lastError = err;

            // Don't retry if we've exhausted attempts
            if (attempt === finalConfig.maxAttempts - 1) {
                break;
            }

            // Calculate delay with exponential backoff and jitter
            const delay = calculateBackoffDelay(
                attempt,
                finalConfig.initialDelayMs,
                finalConfig.maxDelayMs,
                finalConfig.backoffMultiplier,
                finalConfig.jitterFactor
            );

            // Wait before retrying
            await sleep(delay);
        }
    }

    // All retries failed
    throw new Error(
        `Failed to connect to ERST simulator at ${host}:${port} after ${finalConfig.maxAttempts} attempts: ${lastError?.message || 'Unknown error'}`
    );
}

/**
 * Attempts a single connection to the IPC socket (no retry).
 *
 * @param host - The host to connect to
 * @param port - The port to connect to
 * @returns Promise that resolves to socket on success
 */
function attemptConnection(host: string, port: number): Promise<net.Socket> {
    return new Promise((resolve, reject) => {
        const socket = net.createConnection({ host, port });

        const onConnect = () => {
            cleanup();
            resolve(socket);
        };

        const onError = (err: Error) => {
            cleanup();
            reject(err);
        };

        const timeout = setTimeout(() => {
            cleanup();
            reject(new Error('Connection timeout'));
        }, 5000); // 5 second timeout per attempt

        const cleanup = () => {
            socket.removeListener('connect', onConnect);
            socket.removeListener('error', onError);
            clearTimeout(timeout);
        };

        socket.on('connect', onConnect);
        socket.on('error', onError);
    });
}

/**
 * Calculates backoff delay with exponential growth and jitter.
 *
 * Formula: delayMs = min(initialDelay * (multiplier ^ attemptNum), maxDelay) * (1 ┬▒ jitter)
 *
 * @param attemptNum - The attempt number (0-based)
 * @param initialDelayMs - The initial delay in milliseconds
 * @param maxDelayMs - The maximum delay in milliseconds
 * @param backoffMultiplier - The exponential backoff multiplier
 * @param jitterFactor - The jitter as a decimal (e.g., 0.25 for ┬▒25%)
 * @returns The calculated delay in milliseconds
 */
function calculateBackoffDelay(
    attemptNum: number,
    initialDelayMs: number,
    maxDelayMs: number,
    backoffMultiplier: number,
    jitterFactor: number
): number {
    // Exponential backoff: base * multiplier^attempt
    const exponentialDelay = initialDelayMs * Math.pow(backoffMultiplier, attemptNum);

    // Cap at maximum delay
    const cappedDelay = Math.min(exponentialDelay, maxDelayMs);

    // Add jitter: ┬▒jitterFactor * 100%
    const jitterRange = cappedDelay * jitterFactor;
    const jitter = (Math.random() - 0.5) * 2 * jitterRange;

    return Math.max(1, Math.round(cappedDelay + jitter));
}

/**
 * Sleep for the specified number of milliseconds.
 *
 * @param ms - The number of milliseconds to sleep
 * @returns A promise that resolves after the delay
 */
function sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Tests a connection without holding it open.
 *
 * Useful for health checks or connection validation.
 *
 * @param host - The host to connect to
 * @param port - The port to connect to
 * @param timeoutMs - Maximum time to wait for connection
 * @returns Promise that resolves to true if connection succeeds, false otherwise
 */
export async function testConnection(
    host: string,
    port: number,
    timeoutMs: number = 1000
): Promise<boolean> {
    try {
        const socket = await Promise.race([
            attemptConnection(host, port),
            new Promise<net.Socket>((_, reject) =>
                setTimeout(() => reject(new Error('Test timeout')), timeoutMs)
            ),
        ]);
        socket.end();
        return true;
    } catch {
        return false;
    }
}
