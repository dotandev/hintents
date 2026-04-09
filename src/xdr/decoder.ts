// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { Readable } from 'stream';
// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { xdr } from '@stellar/stellar-sdk';
import { Buffer } from './buffer-shim';
import * as crypto from 'crypto';

export enum TransactionMetaVersion {
    V1 = 1,
    V2 = 2,
    V3 = 3,
}

export class XDRDecoder {
    /**
     * Streaming decode for large batches of LedgerEntry XDRs.
     * Accepts a Readable stream or Buffer containing concatenated base64 XDRs.
     * Yields each LedgerEntry as it is decoded, reducing peak memory usage.
     */
    static async *streamLedgerEntries(
        input: any,
        decodeFn: (buf: Buffer) => any
    ): AsyncGenerator<any, void, unknown> {
        let stream: Readable;
        // Robust type check for Buffer (works for both Node and polyfill)
        const isBuffer = (val: any) => val && typeof val === 'object' && typeof val.length === 'number' && typeof val.toString === 'function' && !val.readable;
        if (isBuffer(input)) {
            stream = Readable.from(input.toString().split('\n'));
        } else if (input && typeof input.read === 'function') {
            stream = input;
        } else {
            // Fallback: treat as string
            stream = Readable.from(String(input).split('\n'));
        }

        for await (const chunk of stream) {
            const line = chunk.toString().trim();
            if (!line) continue;
            try {
                const buffer = Buffer.from(line, 'base64');
                const entry = decodeFn(buffer);
                yield entry;
            } catch (error: any) {
                // Optionally log or handle decode errors per entry
                continue;
            }
        }
    }
    /**
     * Decode TransactionMeta from base64 XDR
     */
    static decodeTransactionMeta(base64Xdr: string): xdr.TransactionMeta {
        try {
            const buffer = Buffer.from(base64Xdr, 'base64');
            return xdr.TransactionMeta.fromXDR(buffer);
        } catch (error: any) {
            throw new Error(`Failed to decode TransactionMeta XDR: ${error.message}`);
        }
    }

    /**
     * Detect TransactionMeta version
     */
    static getMetaVersion(meta: xdr.TransactionMeta): TransactionMetaVersion {
        switch (meta.switch()) {
            case 0:
                return TransactionMetaVersion.V1;
            case 1:
                return TransactionMetaVersion.V2;
            case 2:
            case 3:
                return TransactionMetaVersion.V3;
            default:
                throw new Error(`Unknown TransactionMeta version: ${meta.switch()}`);
        }
    }

    /**
     * Get meta version as string for logging
     */
    static getMetaVersionString(version: TransactionMetaVersion): string {
        return `v${version}`;
    }


    /**
     * Decode LedgerKey from XDR
     */
    static decodeLedgerKey(ledgerKey: xdr.LedgerKey): string {
        return ledgerKey.toXDR('base64');
    }

    /**
     * Get LedgerKey type
     */
    static getLedgerKeyType(ledgerKey: xdr.LedgerKey): xdr.LedgerEntryType {
        return ledgerKey.switch();
    }

    /**
     * Hash LedgerKey for deduplication
     */
    static hashLedgerKey(ledgerKey: xdr.LedgerKey): string {
        const xdrBuffer = ledgerKey.toXDR();
        return crypto.createHash('sha256').update(xdrBuffer).digest('hex');
    }

    /**
     * Get LedgerEntryType name as string
     */
    static getLedgerEntryTypeName(type: xdr.LedgerEntryType): string {
        const typeMap: Record<number, string> = {
            0: 'ACCOUNT',
            1: 'TRUSTLINE',
            2: 'OFFER',
            3: 'DATA',
            4: 'CLAIMABLE_BALANCE',
            5: 'LIQUIDITY_POOL',
            6: 'CONTRACT_DATA',
            7: 'CONTRACT_CODE',
            8: 'CONFIG_SETTING',
            9: 'TTL',
        };
        return typeMap[type.value] || 'UNKNOWN';
    }

    /**
     * Validate base64 XDR string
     */
    static isValidBase64(str: string): boolean {
        try {
            Buffer.from(str, 'base64');
            return true;
        } catch {
            return false;
        }
    }
}
