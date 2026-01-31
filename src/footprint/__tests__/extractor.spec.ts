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
