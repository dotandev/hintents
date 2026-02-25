// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import axios, { AxiosInstance, AxiosError } from 'axios';
import { RPCConfig } from '../config/rpc-config';
import { getLogger, LogCategory } from '../utils/logger';

interface RPCEndpoint {
    url: string;
    healthy: boolean;
    failureCount: number;
    lastFailure: number | null;
    circuitOpen: boolean;
    totalRequests: number;
    totalSuccess: number;
    totalFailure: number;
    averageDuration: number;
}

export class FallbackRPCClient {
    private endpoints: RPCEndpoint[];
    private currentIndex: number = 0;
    private config: RPCConfig;
    private clients: Map<string, AxiosInstance> = new Map();

    constructor(config: RPCConfig) {
        const logger = getLogger();
        this.config = config;
        this.endpoints = config.urls.map(url => ({
            url,
            healthy: true,
            failureCount: 0,
            lastFailure: null,
            circuitOpen: false,
            totalRequests: 0,
            totalSuccess: 0,
            totalFailure: 0,
            averageDuration: 0,
        }));

        // Initialize axios clients for each endpoint
        this.endpoints.forEach(endpoint => {
            this.clients.set(endpoint.url, axios.create({
                baseURL: endpoint.url,
                timeout: config.timeout,
                headers: {
                    'Content-Type': 'application/json',
                    ...(config.headers || {}),
                },
            }));
        });

        logger.verbose(LogCategory.RPC, `RPC client initialized with ${this.endpoints.length} endpoint(s)`);

        // Fixed: No longer leaking verbose info into standard mode
        this.endpoints.forEach((ep, idx) => {
            logger.verboseIndent(LogCategory.RPC, `[${idx + 1}] ${ep.url}`);
        });
    }

    /**
     * Make RPC request with automatic fallback
     */
    async request<T = any>(path: string, options: { method?: 'GET' | 'POST', data?: any } = {}): Promise<T> {
        const logger = getLogger();
        const method = options.method || 'POST';
        const data = options.data;
        const startTime = Date.now();
        let lastError: Error | null = null;

        // Try each endpoint in order
        for (let attempt = 0; attempt < this.endpoints.length; attempt++) {
            const endpoint = this.getNextHealthyEndpoint();

            if (!endpoint) {
                throw new Error('All RPC endpoints are unavailable');
            }

            try {
                endpoint.totalRequests++;

                // Verbose logging for request
                logger.verbose(LogCategory.RPC, `→ ${method} ${path}`);
                logger.verboseIndent(LogCategory.RPC, `Endpoint: ${endpoint.url}`);

                const requestStartTime = Date.now();
                const client = this.clients.get(endpoint.url)!;

                // Verbose: Payload size
                const requestSize = data ? JSON.stringify(data).length : 0;
                logger.verboseIndent(LogCategory.RPC, `${method} request to ${path}`);
                logger.verboseIndent(LogCategory.RPC, `Request size: ${logger.formatBytes(requestSize)}`);

                const response = await this.executeWithRetry(client, method, path, data);

                const duration = Date.now() - requestStartTime;
                this.updateMetrics(endpoint, duration, true);

                // Success! Mark endpoint as healthy and reset to primary
                this.markSuccess(endpoint);
                this.currentIndex = 0; // Return to primary

                const responseSize = response.data ? JSON.stringify(response.data).length : 0;
                logger.verbose(LogCategory.RPC, `← Response received (${duration}ms)`);
                logger.verboseIndent(LogCategory.RPC, `Status: ${response.status} ${response.statusText}`);
                logger.verboseIndent(LogCategory.RPC, `Response size: ${logger.formatBytes(responseSize)}`);

                return response.data;

            } catch (error) {
                lastError = error as Error;
                const duration = Date.now() - startTime;
                this.updateMetrics(endpoint, duration, false);

                // Determine if this is a retryable error
                if (this.isRetryableError(error)) {
                    logger.warn(`RPC request failed: ${endpoint.url}`);

                    // Verbose error details
                    if (axios.isAxiosError(error)) {
                        logger.verbose(LogCategory.ERROR, `Request error: ${error.message}`);
                        if (error.code) logger.verboseIndent(LogCategory.ERROR, `Code: ${error.code}`);
                    }

                    // Mark endpoint as failed
                    this.markFailure(endpoint);

                    // Continue to next endpoint in fallback list
                    continue;
                } else {
                    this.markFailure(endpoint);
                    throw error;
                }
            }
        }

        // All endpoints failed
        const totalDuration = Date.now() - startTime;
        logger.error(`All RPC endpoints failed after ${totalDuration}ms`);
        throw new Error(`All RPC endpoints failed: ${lastError?.message}`);
    }

    /**
     * Update performance metrics for an endpoint
     */
    private updateMetrics(endpoint: RPCEndpoint, duration: number, success: boolean): void {
        const logger = getLogger();
        if (success) {
            endpoint.totalSuccess++;
        } else {
            endpoint.totalFailure++;
        }

        // Running average calculation
        const count = endpoint.totalSuccess + endpoint.totalFailure;
        endpoint.averageDuration = (endpoint.averageDuration * (count - 1) + duration) / count;

        logger.verbose(LogCategory.PERF, `Metrics updated for ${endpoint.url}`);
        logger.verboseIndent(LogCategory.PERF, `Avg duration: ${Math.round(endpoint.averageDuration)}ms`);
    }

    /**
     * Execute request with local retries and exponential backoff
     */
    private async executeWithRetry(client: AxiosInstance, method: 'GET' | 'POST', path: string, data: any): Promise<any> {
        const logger = getLogger();
        let lastError: any;

        for (let attempt = 0; attempt < this.config.retries; attempt++) {
            try {
                if (method === 'GET') {
                    return await client.get(path);
                }
                return await client.post(path, data);
            } catch (error) {
                lastError = error;

                if (attempt < this.config.retries - 1 && this.isRetryableError(error)) {
                    const delay = this.config.retryDelay * Math.pow(2, attempt);
                    logger.verbose(LogCategory.RPC, `Retrying in ${delay}ms... (Attempt ${attempt + 1}/${this.config.retries})`);
                    await new Promise(resolve => setTimeout(resolve, delay));
                } else {
                    throw error;
                }
            }
        }

        throw lastError;
    }

    /**
     * Get next healthy endpoint
     */
    private getNextHealthyEndpoint(): RPCEndpoint | null {
        const now = Date.now();

        // Check circuit breakers and reset if timeout passed
        this.endpoints.forEach(endpoint => {
            if (endpoint.circuitOpen && endpoint.lastFailure) {
                if (now - endpoint.lastFailure > this.config.circuitBreakerTimeout) {
                    console.log(`🔄 Circuit breaker reset for: ${endpoint.url}`);
                    endpoint.circuitOpen = false;
                    endpoint.failureCount = 0;
                }
            }
        });

        // Find next healthy endpoint
        for (let i = 0; i < this.endpoints.length; i++) {
            const index = (this.currentIndex + i) % this.endpoints.length;
            const endpoint = this.endpoints[index];

            if (!endpoint.circuitOpen) {
                this.currentIndex = (index + 1) % this.endpoints.length;
                return endpoint;
            }
        }

        return null;
    }

    /**
     * Mark endpoint as successful
     */
    private markSuccess(endpoint: RPCEndpoint): void {
        endpoint.healthy = true;
        endpoint.failureCount = 0;
        endpoint.circuitOpen = false;
    }

    /**
     * Mark endpoint as failed
     */
    private markFailure(endpoint: RPCEndpoint): void {
        endpoint.healthy = false;
        endpoint.failureCount++;
        endpoint.lastFailure = Date.now();

        // Open circuit breaker if threshold exceeded
        if (endpoint.failureCount >= this.config.circuitBreakerThreshold) {
            console.warn(`[READY] Circuit breaker opened for: ${endpoint.url}`);
            endpoint.circuitOpen = true;
        }
    }

    /**
     * Determine if error is retryable
     */
    private isRetryableError(error: any): boolean {
        // Handle axios errors
        if (axios.isAxiosError(error)) {
            const axiosError = error as AxiosError;

            // Network errors or timeout
            if (!axiosError.response) {
                return true; // No response usually means network/timeout issue
            }

            // Explicit codes
            const retryableCodes = [
                'ECONNREFUSED', 'ENOTFOUND', 'ETIMEDOUT', 'ECONNRESET',
                'ECONNABORTED', 'ERR_NETWORK'
            ];

            if (axiosError.code && retryableCodes.includes(axiosError.code)) {
                return true;
            }

            // HTTP 5xx errors (server errors)
            if (axiosError.response.status >= 500) {
                return true;
            }

            // HTTP 429 (rate limit)
            if (axiosError.response.status === 429) {
                return true;
            }
        }

        // Generic network error check (for mock adapter or non-axios wrapped errors)
        const message = (error as Error)?.message?.toLowerCase() || '';
        if (message.includes('network error') || message.includes('timeout')) {
            return true;
        }

        return false;
    }

    /**
     * Get health status of all endpoints
     */
    getHealthStatus(): Array<{
        url: string;
        healthy: boolean;
        failureCount: number;
        circuitOpen: boolean;
        metrics: {
            totalRequests: number;
            totalSuccess: number;
            totalFailure: number;
            averageDuration: number;
        };
    }> {
        return this.endpoints.map(ep => ({
            url: ep.url,
            healthy: ep.healthy,
            failureCount: ep.failureCount,
            circuitOpen: ep.circuitOpen,
            metrics: {
                totalRequests: ep.totalRequests,
                totalSuccess: ep.totalSuccess,
                totalFailure: ep.totalFailure,
                averageDuration: Math.round(ep.averageDuration),
            },
        }));
    }

    /**
     * Perform health check on all endpoints
     */
    async performHealthChecks(): Promise<void> {
        console.log('[HEALTH] Performing health checks on all RPC endpoints...');

        const checks = this.endpoints.map(async (endpoint) => {
            try {
                const client = this.clients.get(endpoint.url)!;
                const response = await client.post('', {
                    jsonrpc: '2.0',
                    id: 1,
                    method: 'getHealth'
                }, { timeout: 5000 });

                if (response.data && response.data.result && response.data.result.status === 'healthy') {
                    this.markSuccess(endpoint);
                    console.log(`    ${endpoint.url} (healthy)`);
                } else {
                    this.markFailure(endpoint);
                    console.log(`   [FAIL] ${endpoint.url} (invalid response)`);
                }
            } catch (error) {
                this.markFailure(endpoint);
                console.log(`   [FAIL] ${endpoint.url}`);
            }
        });

        await Promise.allSettled(checks);
    }
}
