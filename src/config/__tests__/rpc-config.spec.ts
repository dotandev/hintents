// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { RPCConfigParser } from '../rpc-config';

describe('RPCConfigParser', () => {
    describe('parseUrls', () => {
        it('should parse comma-separated string', () => {
            const input = 'https://rpc1.com,https://rpc2.com,https://rpc3.com';
            const result = RPCConfigParser.parseUrls(input);

            expect(result).toEqual([
                'https://rpc1.com',
                'https://rpc2.com',
                'https://rpc3.com',
            ]);
        });

        it('should handle whitespace in URLs', () => {
            const input = ' https://rpc1.com , https://rpc2.com ';
            const result = RPCConfigParser.parseUrls(input);

            expect(result).toEqual(['https://rpc1.com', 'https://rpc2.com']);
        });

        it('should filter out invalid URLs', () => {
            const input = 'https://valid.com,invalid-url,ftp://wrong.com';
            const result = RPCConfigParser.parseUrls(input);

            expect(result).toEqual(['https://valid.com']);
        });

        it('should accept array of URLs', () => {
            const input = ['https://rpc1.com', 'https://rpc2.com'];
            const result = RPCConfigParser.parseUrls(input);

            expect(result).toEqual(['https://rpc1.com', 'https://rpc2.com']);
        });

        it('should throw error if no valid URLs', () => {
            expect(() => RPCConfigParser.parseUrls('invalid')).toThrow('No valid RPC URLs');
        });
    });

    describe('isValidUrl', () => {
        it('should accept valid HTTPS URLs', () => {
            expect(RPCConfigParser.isValidUrl('https://example.com')).toBe(true);
        });

        it('should accept valid HTTP URLs', () => {
            expect(RPCConfigParser.isValidUrl('http://localhost:8080')).toBe(true);
        });

        it('should reject invalid URLs', () => {
            expect(RPCConfigParser.isValidUrl('not-a-url')).toBe(false);
            expect(RPCConfigParser.isValidUrl('ftp://wrong.com')).toBe(false);
        });
    });

    describe('loadConfig', () => {
        const originalEnv = process.env;

        beforeEach(() => {
            jest.resetModules();
            process.env = { ...originalEnv };
        });

        afterAll(() => {
            process.env = originalEnv;
        });

        it('should load from options', () => {
            const config = RPCConfigParser.loadConfig({
                rpc: 'https://rpc1.com',
                timeout: 5000,
                retries: 5
            });

            expect(config.urls).toEqual(['https://rpc1.com']);
            expect(config.timeout).toBe(5000);
            expect(config.retries).toBe(5);
        });

        it('should load from environment variable', () => {
            process.env.STELLAR_RPC_URLS = 'https://env1.com,https://env2.com';
            const config = RPCConfigParser.loadConfig({});

            expect(config.urls).toEqual(['https://env1.com', 'https://env2.com']);
        });

        it('should use default values', () => {
            const config = RPCConfigParser.loadConfig({ rpc: 'https://rpc.com' });

            expect(config.timeout).toBe(30000);
            expect(config.retries).toBe(3);
            expect(config.retryDelay).toBe(1000);
        });

        it('should throw error if no RPC URLs configured', () => {
            delete process.env.STELLAR_RPC_URLS;
            expect(() => RPCConfigParser.loadConfig({})).toThrow('No RPC URLs configured');
        });
    });
});
