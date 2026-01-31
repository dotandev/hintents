// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { URL } from 'url';

export interface RPCConfig {
    urls: string[];
    timeout: number;
    retries: number;
    retryDelay: number;
    circuitBreakerThreshold: number;
    circuitBreakerTimeout: number;
    maxRedirects: number;
    headers?: Record<string, string>;
}

export class RPCConfigParser {
    /**
     * Parse RPC URLs from comma-separated string or array
     */
    static parseUrls(input: string | string[]): string[] {
        const urls: string[] = [];

        if (Array.isArray(input)) {
            urls.push(...input);
        } else if (typeof input === 'string') {
            // Split by comma and trim whitespace
            urls.push(...input.split(',').map(url => url.trim()));
        }

        // Validate each URL
        const validUrls = urls.filter(url => this.isValidUrl(url));

        if (validUrls.length === 0) {
            throw new Error('No valid RPC URLs provided');
        }

        return validUrls;
    }

    /**
     * Validate URL format
     */
    static isValidUrl(urlString: string): boolean {
        try {
            const url = new URL(urlString);
            return url.protocol === 'http:' || url.protocol === 'https:';
        } catch {
            console.warn(`⚠️  Invalid RPC URL skipped: ${urlString}`);
            return false;
        }
    }

    /**
     * Load RPC configuration from environment and CLI args
     */
    static loadConfig(options: {
        rpc?: string | string[];
        timeout?: number;
        retries?: number;
    }): RPCConfig {
        // Get URLs from CLI args or environment variable
        const urlInput = options.rpc || process.env.STELLAR_RPC_URLS;

        if (!urlInput) {
            throw new Error('No RPC URLs configured. Use --rpc flag or STELLAR_RPC_URLS env variable');
        }

        const urls = this.parseUrls(urlInput);

        return {
            urls,
            timeout: options.timeout || 30000, // 30 seconds
            retries: options.retries || 3,
            retryDelay: 1000, // 1 second
            circuitBreakerThreshold: 5,
            circuitBreakerTimeout: 60000, // 1 minute
            maxRedirects: 5,
        };
    }
}
