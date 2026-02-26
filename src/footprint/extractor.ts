// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { xdr } from '@stellar/stellar-sdk';
import { XDRDecoder, TransactionMetaVersion } from '../xdr/decoder';
import { LedgerKey, FootprintResult } from '../xdr/types';

export class FootprintExtractor {
    /**
     * Extract footprint from TransactionMeta
     */
    static extractFootprint(metaXdr: string): FootprintResult {
        const meta = XDRDecoder.decodeTransactionMeta(metaXdr);
        const version = XDRDecoder.getMetaVersion(meta);

        console.log(`Extracting footprint from TransactionMeta ${XDRDecoder.getMetaVersionString(version)}...`);

        let allKeysWithType: Array<{ key: LedgerKey; isReadOnly: boolean }> = [];

        switch (version) {
            case TransactionMetaVersion.V1:
                allKeysWithType = this.extractFromMetaV1(meta.v1());
                break;
            case TransactionMetaVersion.V2:
                allKeysWithType = this.extractFromMetaV2(meta.v2());
                break;
            case TransactionMetaVersion.V3:
                allKeysWithType = this.extractFromMetaV3(meta.v3());
                break;
            default:
                throw new Error(`Unsupported meta version: ${version}`);
        }

        // Build a hash → isReadOnly map in O(n); first-seen semantics are
        // preserved because we skip hashes already recorded.
        const readOnlyByHash = new Map<string, boolean>();
        for (const { key, isReadOnly } of allKeysWithType) {
            if (key.hash && !readOnlyByHash.has(key.hash)) {
                readOnlyByHash.set(key.hash, isReadOnly);
            }
        }

        const allKeys = allKeysWithType.map(k => k.key);
        const deduplicated = this.deduplicateKeys(allKeys);

        // O(n) single-pass classification using the pre-built Map — avoids the
        // O(n²) Array.find() that caused UI freezes on >100 MB trace footprints.
        const readOnly = deduplicated.filter(key => readOnlyByHash.get(key.hash) === true);
        const readWrite = deduplicated.filter(key => readOnlyByHash.get(key.hash) !== true);

        return {
            readOnly,
            readWrite,
            all: deduplicated,
        };
    }

    /**
     * Extract from TransactionMeta V1
     */
    private static extractFromMetaV1(meta: xdr.TransactionMetaV1): Array<{ key: LedgerKey; isReadOnly: boolean }> {
        const keys: Array<{ key: LedgerKey; isReadOnly: boolean }> = [];

        const operations = meta.operations();

        for (let i = 0; i < operations.length; i++) {
            const operation = operations[i];
            const changes = operation.changes();

            for (const change of changes) {
                const ledgerKeys = this.extractFromLedgerEntryChange(change);
                keys.push(...ledgerKeys);
            }
        }

        return keys;
    }

    /**
     * Extract from TransactionMeta V2
     */
    private static extractFromMetaV2(meta: xdr.TransactionMetaV2): Array<{ key: LedgerKey; isReadOnly: boolean }> {
        const keys: Array<{ key: LedgerKey; isReadOnly: boolean }> = [];

        const changesBefore = meta.txChangesBefore();
        for (const change of changesBefore) {
            const ledgerKeys = this.extractFromLedgerEntryChange(change);
            keys.push(...ledgerKeys);
        }

        const operations = meta.operations();
        for (const operation of operations) {
            const changes = operation.changes();
            for (const change of changes) {
                const ledgerKeys = this.extractFromLedgerEntryChange(change);
                keys.push(...ledgerKeys);
            }
        }

        const changesAfter = meta.txChangesAfter();
        for (const change of changesAfter) {
            const ledgerKeys = this.extractFromLedgerEntryChange(change);
            keys.push(...ledgerKeys);
        }

        return keys;
    }

    /**
     * Extract from TransactionMeta V3 (Soroban)
     */
    private static extractFromMetaV3(meta: xdr.TransactionMetaV3): Array<{ key: LedgerKey; isReadOnly: boolean }> {
        const keys: Array<{ key: LedgerKey; isReadOnly: boolean }> = [];

        const changesBefore = meta.txChangesBefore();
        for (const change of changesBefore) {
            const ledgerKeys = this.extractFromLedgerEntryChange(change);
            keys.push(...ledgerKeys);
        }

        const operations = meta.operations();
        for (const operation of operations) {
            const changes = operation.changes();
            for (const change of changes) {
                const ledgerKeys = this.extractFromLedgerEntryChange(change);
                keys.push(...ledgerKeys);
            }
        }

        const changesAfter = meta.txChangesAfter();
        for (const change of changesAfter) {
            const ledgerKeys = this.extractFromLedgerEntryChange(change);
            keys.push(...ledgerKeys);
        }

        const sorobanMeta = meta.sorobanMeta();
        if (sorobanMeta) {
            console.log('Extracting Soroban footprint from meta v3...');
            const sorobanKeys = this.extractFromSorobanMeta(sorobanMeta);
            keys.push(...sorobanKeys);
        }

        return keys;
    }

    /**
     * Extract from SorobanTransactionMeta
     */
    private static extractFromSorobanMeta(sorobanMeta: xdr.SorobanTransactionMeta): Array<{ key: LedgerKey; isReadOnly: boolean }> {
        const keys: Array<{ key: LedgerKey; isReadOnly: boolean }> = [];

        // TODO: Extract contract data/code keys from Soroban events
        // For now, the contract state changes are already captured in the
        // txChangesBefore/After and operation changes above

        return keys;
    }

    /**
     * Extract LedgerKey from LedgerEntryChange
     */
    private static extractFromLedgerEntryChange(change: xdr.LedgerEntryChange): Array<{ key: LedgerKey; isReadOnly: boolean }> {
        const keys: Array<{ key: LedgerKey; isReadOnly: boolean }> = [];

        const changeType = change.switch().name;

        switch (changeType) {
            case 'ledgerEntryCreated':
                const created = change.created();
                if (created) {
                    const key = this.ledgerEntryToKey(created);
                    if (key) keys.push({ key, isReadOnly: false });
                }
                break;

            case 'ledgerEntryUpdated':
                const updated = change.updated();
                if (updated) {
                    const key = this.ledgerEntryToKey(updated);
                    if (key) keys.push({ key, isReadOnly: false });
                }
                break;

            case 'ledgerEntryRemoved':
                const removedKey = change.removed();
                if (removedKey) {
                    keys.push({
                        key: {
                            type: removedKey.switch(),
                            key: XDRDecoder.decodeLedgerKey(removedKey),
                            hash: XDRDecoder.hashLedgerKey(removedKey),
                        },
                        isReadOnly: false,
                    });
                }
                break;

            case 'ledgerEntryState':
                const state = change.state();
                if (state) {
                    const key = this.ledgerEntryToKey(state);
                    if (key) keys.push({ key, isReadOnly: true });
                }
                break;

            default:
                console.warn(`Unknown ledger entry change type: ${changeType}`);
        }

        return keys;
    }

    /**
     * Convert LedgerEntry to LedgerKey
     */
    private static ledgerEntryToKey(entry: xdr.LedgerEntry): LedgerKey | null {
        const data = entry.data();

        let ledgerKey: xdr.LedgerKey | null = null;

        switch (data.switch().name) {
            case 'account':
                const account = data.account();
                ledgerKey = xdr.LedgerKey.account(
                    new xdr.LedgerKeyAccount({
                        accountId: account.accountId(),
                    })
                );
                break;

            case 'trustline':
                const trustline = data.trustLine();
                ledgerKey = xdr.LedgerKey.trustline(
                    new xdr.LedgerKeyTrustLine({
                        accountId: trustline.accountId(),
                        asset: trustline.asset(),
                    })
                );
                break;

            case 'offer':
                const offer = data.offer();
                ledgerKey = xdr.LedgerKey.offer(
                    new xdr.LedgerKeyOffer({
                        sellerId: offer.sellerId(),
                        offerId: offer.offerId(),
                    })
                );
                break;

            case 'data':
                const dataEntry = data.data();
                ledgerKey = xdr.LedgerKey.data(
                    new xdr.LedgerKeyData({
                        accountId: dataEntry.accountId(),
                        dataName: dataEntry.dataName(),
                    })
                );
                break;

            case 'claimableBalance':
                const cb = data.claimableBalance();
                ledgerKey = xdr.LedgerKey.claimableBalance(
                    new xdr.LedgerKeyClaimableBalance({
                        balanceId: cb.balanceId(),
                    })
                );
                break;

            case 'liquidityPool':
                const lp = data.liquidityPool();
                ledgerKey = xdr.LedgerKey.liquidityPool(
                    new xdr.LedgerKeyLiquidityPool({
                        liquidityPoolId: lp.liquidityPoolId(),
                    })
                );
                break;

            case 'contractData':
                const contractData = data.contractData();
                ledgerKey = xdr.LedgerKey.contractData(
                    new xdr.LedgerKeyContractData({
                        contract: contractData.contract(),
                        key: contractData.key(),
                        durability: contractData.durability(),
                    })
                );
                break;

            case 'contractCode':
                const contractCode = data.contractCode();
                ledgerKey = xdr.LedgerKey.contractCode(
                    new xdr.LedgerKeyContractCode({
                        hash: contractCode.hash(),
                    })
                );
                break;

            case 'configSetting':
                // TODO: Add support for config setting entries in future protocol versions
                console.warn('ConfigSetting entries not yet fully supported');
                return null;

            case 'ttl':
                const ttl = data.ttl();
                ledgerKey = xdr.LedgerKey.ttl(
                    new xdr.LedgerKeyTtl({
                        keyHash: ttl.keyHash(),
                    })
                );
                break;

            default:
                console.warn(`Unknown ledger entry type: ${data.switch().name}`);
                return null;
        }

        if (!ledgerKey) return null;

        return {
            type: ledgerKey.switch(),
            key: XDRDecoder.decodeLedgerKey(ledgerKey),
            hash: XDRDecoder.hashLedgerKey(ledgerKey),
        };
    }

    /**
     * Deduplicate LedgerKeys using hash
     */
    private static deduplicateKeys(keys: LedgerKey[]): LedgerKey[] {
        const seen = new Set<string>();
        const unique: LedgerKey[] = [];

        for (const key of keys) {
            if (key.hash && key.hash.length > 0 && !seen.has(key.hash)) {
                seen.add(key.hash);
                unique.push(key);
            }
        }

        console.log(`Deduplicated ${keys.length} keys to ${unique.length} unique keys`);

        return unique;
    }

    /**
     * Categorize keys by type
     */
    static categorizeKeys(keys: LedgerKey[]): Map<xdr.LedgerEntryType, LedgerKey[]> {
        const categorized = new Map<xdr.LedgerEntryType, LedgerKey[]>();

        for (const key of keys) {
            if (!categorized.has(key.type)) {
                categorized.set(key.type, []);
            }
            categorized.get(key.type)!.push(key);
        }

        return categorized;
    }
}
