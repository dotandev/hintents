// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { RPCConfigParser } from '../rpc-config';
import { UrlsValidator, NumericValidator } from '../validators';

describe('RPC config validators', () => {
    it('throws when urls missing', () => {
        const cfg = { urls: [], timeout: 1000, retries: 1, retryDelay: 100, circuitBreakerThreshold: 1, circuitBreakerTimeout: 1000, maxRedirects: 0 } as any;
        expect(() => new UrlsValidator().validate(cfg)).toThrow('No RPC URLs configured');
    });

    it('throws when numeric fields have wrong types', () => {
        const cfg = { urls: ['https://rpc.com'], timeout: '30000', retries: '3', retryDelay: '1000', circuitBreakerThreshold: '5', circuitBreakerTimeout: '60000', maxRedirects: '5' } as any;
        expect(() => new NumericValidator().validate(cfg)).toThrow('timeout must be a positive integer');
    });

    it('throws when numeric fields are out of bounds', () => {
        const cfg = { urls: ['https://rpc.com'], timeout: -1, retries: -2, retryDelay: 0, circuitBreakerThreshold: 0, circuitBreakerTimeout: 0, maxRedirects: -1 } as any;
        expect(() => new NumericValidator().validate(cfg)).toThrow('timeout must be a positive integer');
    });

    it('loadConfig runs validators and returns config', () => {
        const config = RPCConfigParser.loadConfig({ rpc: 'https://rpc.example' });
        expect(config.urls).toEqual(['https://rpc.example']);
        expect(config.timeout).toBe(30000);
    });
});
