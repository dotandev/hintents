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

        let allKeys: LedgerKey[] = [];

        switch (version) {
            case TransactionMetaVersion.V1:
                allKeys = this.extractFromMetaV1(meta.v1());
                break;
            case TransactionMetaVersion.V2:
                allKeys = this.extractFromMetaV2(meta.v2());
                break;
            case TransactionMetaVersion.V3:
                allKeys = this.extractFromMetaV3(meta.v3());
                break;
            default:
                throw new Error(`Unsupported meta version: ${version}`);
        }

        const deduplicated = this.deduplicateKeys(allKeys);

        return {
            readOnly: [],
            readWrite: deduplicated,
            all: deduplicated,
        };
    }

    /**
     * Extract from TransactionMeta V1
     */
    private static extractFromMetaV1(meta: xdr.TransactionMetaV1): LedgerKey[] {
        const keys: LedgerKey[] = [];

        const operations = meta.operations();

        for (const operation of operations) {
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
    private static extractFromMetaV2(meta: xdr.TransactionMetaV2): LedgerKey[] {
        const keys: LedgerKey[] = [];

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
    private static extractFromMetaV3(meta: xdr.TransactionMetaV3): LedgerKey[] {
        const keys: LedgerKey[] = [];

        return keys;
    }

    /**
     * Extract LedgerKey from LedgerEntryChange
     */
    private static extractFromLedgerEntryChange(change: xdr.LedgerEntryChange): LedgerKey[] {
        const keys: LedgerKey[] = [];

        switch (change.switch().name) {
            case 'ledgerEntryCreated':
                const created = change.created();
                if (created) {
                    const key = this.ledgerEntryToKey(created);
                    if (key) keys.push(key);
                }
                break;

            case 'ledgerEntryUpdated':
                const updated = change.updated();
                if (updated) {
                    const key = this.ledgerEntryToKey(updated);
                    if (key) keys.push(key);
                }
                break;

            case 'ledgerEntryRemoved':
                const removedKey = change.removed();
                if (removedKey) {
                    keys.push({
                        type: removedKey.switch(),
                        key: XDRDecoder.decodeLedgerKey(removedKey),
                        hash: XDRDecoder.hashLedgerKey(removedKey),
                    });
                }
                break;

            case 'ledgerEntryState':
                const state = change.state();
                if (state) {
                    const key = this.ledgerEntryToKey(state);
                    if (key) keys.push(key);
                }
                break;
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

            case 'datum':
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
                const configSetting = data.configSetting();
                ledgerKey = xdr.LedgerKey.configSetting(
                    new xdr.LedgerKeyConfigSetting({
                        configSettingId: configSetting.configSettingId(),
                    })
                );
                break;

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
            if (!seen.has(key.hash)) {
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
