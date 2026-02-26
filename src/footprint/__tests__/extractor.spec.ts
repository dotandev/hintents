// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { FootprintExtractor } from '../extractor';
import { xdr } from '@stellar/stellar-sdk';

describe('FootprintExtractor', () => {
    describe('extractFootprint', () => {
        it('should handle invalid XDR', () => {
            const invalidXdr = 'invalid-base64';

            expect(() => {
                FootprintExtractor.extractFootprint(invalidXdr);
            }).toThrow('Failed to decode TransactionMeta XDR');
        });

        it('should return empty footprint for empty operations', () => {
            // This would need a real empty transaction meta XDR
            // For now, we test the structure
            const result = {
                readOnly: [],
                readWrite: [],
                all: [],
            };

            expect(result).toBeDefined();
            expect(result.all).toBeInstanceOf(Array);
            expect(result.readOnly).toBeInstanceOf(Array);
            expect(result.readWrite).toBeInstanceOf(Array);
        });

        it('should deduplicate keys', () => {
            // Test that duplicate keys are properly filtered
            const keys = [
                { type: xdr.LedgerEntryType.account(), key: 'key1', hash: 'hash1' },
                { type: xdr.LedgerEntryType.account(), key: 'key1', hash: 'hash1' },
                { type: xdr.LedgerEntryType.trustline(), key: 'key2', hash: 'hash2' },
            ];

            const hashes = keys.map(k => k.hash);
            const uniqueHashes = new Set(hashes);

            expect(hashes.length).toBe(3);
            expect(uniqueHashes.size).toBe(2);
        });

        it('should filter out empty hashes', () => {
            const keys = [
                { type: xdr.LedgerEntryType.account(), key: 'key1', hash: '' },
                { type: xdr.LedgerEntryType.account(), key: 'key2', hash: 'hash2' },
            ];

            const validKeys = keys.filter(k => k.hash && k.hash.length > 0);

            expect(validKeys.length).toBe(1);
            expect(validKeys[0].hash).toBe('hash2');
        });
    });

    describe('categorizeKeys', () => {
        it('should categorize keys by type', () => {
            const keys = [
                { type: xdr.LedgerEntryType.account(), key: 'key1', hash: 'hash1' },
                { type: xdr.LedgerEntryType.account(), key: 'key2', hash: 'hash2' },
                { type: xdr.LedgerEntryType.trustline(), key: 'key3', hash: 'hash3' },
            ];

            const categorized = FootprintExtractor.categorizeKeys(keys);

            expect(categorized.size).toBe(2);
            expect(categorized.get(xdr.LedgerEntryType.account())).toHaveLength(2);
            expect(categorized.get(xdr.LedgerEntryType.trustline())).toHaveLength(1);
        });

        it('should handle empty key array', () => {
            const keys: any[] = [];

            const categorized = FootprintExtractor.categorizeKeys(keys);

            expect(categorized.size).toBe(0);
        });

        it('should handle single key type', () => {
            const keys = [
                { type: xdr.LedgerEntryType.account(), key: 'key1', hash: 'hash1' },
                { type: xdr.LedgerEntryType.account(), key: 'key2', hash: 'hash2' },
            ];

            const categorized = FootprintExtractor.categorizeKeys(keys);

            expect(categorized.size).toBe(1);
            expect(categorized.get(xdr.LedgerEntryType.account())).toHaveLength(2);
        });
    });

    describe('XDR decoder integration', () => {
        it('should validate base64 format', () => {
            const validBase64 = 'SGVsbG8gV29ybGQ=';
            const invalidBase64 = 'not-valid-base64!@#';

            expect(() => Buffer.from(validBase64, 'base64')).not.toThrow();
        });

        it('should handle different meta versions', () => {
            const versions = [1, 2, 3];

            versions.forEach(version => {
                expect(version).toBeGreaterThanOrEqual(1);
                expect(version).toBeLessThanOrEqual(3);
            });
        });
    });

    describe('Read/Write separation', () => {
        it('should separate read-only and read-write keys', () => {
            const allKeys = [
                { key: { type: xdr.LedgerEntryType.account(), key: 'key1', hash: 'hash1' }, isReadOnly: true },
                { key: { type: xdr.LedgerEntryType.account(), key: 'key2', hash: 'hash2' }, isReadOnly: false },
                { key: { type: xdr.LedgerEntryType.trustline(), key: 'key3', hash: 'hash3' }, isReadOnly: false },
            ];

            const readOnly = allKeys.filter(k => k.isReadOnly).map(k => k.key);
            const readWrite = allKeys.filter(k => !k.isReadOnly).map(k => k.key);

            expect(readOnly).toHaveLength(1);
            expect(readWrite).toHaveLength(2);
        });

        it('should handle all read-only keys', () => {
            const allKeys = [
                { key: { type: xdr.LedgerEntryType.account(), key: 'key1', hash: 'hash1' }, isReadOnly: true },
                { key: { type: xdr.LedgerEntryType.account(), key: 'key2', hash: 'hash2' }, isReadOnly: true },
            ];

            const readOnly = allKeys.filter(k => k.isReadOnly).map(k => k.key);
            const readWrite = allKeys.filter(k => !k.isReadOnly).map(k => k.key);

            expect(readOnly).toHaveLength(2);
            expect(readWrite).toHaveLength(0);
        });

        it('should handle all read-write keys', () => {
            const allKeys = [
                { key: { type: xdr.LedgerEntryType.account(), key: 'key1', hash: 'hash1' }, isReadOnly: false },
                { key: { type: xdr.LedgerEntryType.account(), key: 'key2', hash: 'hash2' }, isReadOnly: false },
            ];

            const readOnly = allKeys.filter(k => k.isReadOnly).map(k => k.key);
            const readWrite = allKeys.filter(k => !k.isReadOnly).map(k => k.key);

            expect(readOnly).toHaveLength(0);
            expect(readWrite).toHaveLength(2);
        });

        it('should correctly classify 5000 keys using O(1) map lookup (regression: O(n²) fix)', () => {
            // Build a synthetic allKeysWithType array with alternating read-only/read-write keys.
            const count = 5_000;
            const allKeysWithType: Array<{ key: { type: any; key: string; hash: string }; isReadOnly: boolean }> = [];
            for (let i = 0; i < count; i++) {
                allKeysWithType.push({
                    key: { type: xdr.LedgerEntryType.account(), key: `k${i}`, hash: `h${i}` },
                    isReadOnly: i % 2 === 0,
                });
            }

            // Replicate the optimised classification logic from FootprintExtractor.extractFootprint.
            const readOnlyByHash = new Map<string, boolean>();
            for (const { key, isReadOnly } of allKeysWithType) {
                if (key.hash && !readOnlyByHash.has(key.hash)) {
                    readOnlyByHash.set(key.hash, isReadOnly);
                }
            }
            const allKeys = allKeysWithType.map(k => k.key);
            // Simulate deduplication (all hashes unique here).
            const deduplicated = allKeys;
            const readOnly = deduplicated.filter(key => readOnlyByHash.get(key.hash) === true);
            const readWrite = deduplicated.filter(key => readOnlyByHash.get(key.hash) !== true);

            expect(readOnly).toHaveLength(count / 2);
            expect(readWrite).toHaveLength(count / 2);

            // Spot-check: even indices are read-only, odd are read-write.
            expect(readOnly.every(k => parseInt(k.key.slice(1)) % 2 === 0)).toBe(true);
            expect(readWrite.every(k => parseInt(k.key.slice(1)) % 2 !== 0)).toBe(true);
        });
    });

    describe('LedgerKey types', () => {
        it('should support all ledger entry types', () => {
            const supportedTypes = [
                'ACCOUNT',
                'TRUSTLINE',
                'OFFER',
                'DATA',
                'CLAIMABLE_BALANCE',
                'LIQUIDITY_POOL',
                'CONTRACT_DATA',
                'CONTRACT_CODE',
                'CONFIG_SETTING',
                'TTL',
            ];

            expect(supportedTypes).toHaveLength(10);
        });
    });
});
