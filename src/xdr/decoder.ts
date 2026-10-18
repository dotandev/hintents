// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { Readable } from 'stream';
// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { xdr } from '@stellar/stellar-sdk';
import { Buffer } from './buffer-shim';
import * as crypto from 'crypto';

const decodedBufferPool: Buffer[] = [];
const MAX_POOLED_BUFFER_SIZE = 256 * 1024;

function acquireDecodedBase64(value: string): Buffer {
    const decodedLength = Math.floor((value.length * 3) / 4);
    const reusable = decodedBufferPool.pop();
    const output = reusable && reusable.length >= decodedLength
        ? reusable
        : Buffer.from(new Uint8Array(decodedLength));
    const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
    const lookup = (char: string) => alphabet.indexOf(char);
    let outputOffset = 0;
    for (let index = 0; index < value.length; index += 4) {
        const a = lookup(value[index]);
        const b = lookup(value[index + 1]);
        const c = value[index + 2] === '=' ? 0 : lookup(value[index + 2]);
        const d = value[index + 3] === '=' ? 0 : lookup(value[index + 3]);
        if (a < 0 || b < 0 || c < 0 || d < 0) throw new Error('Invalid base64 input');
        output[outputOffset++] = (a << 2) | (b >> 4);
        if (value[index + 2] !== '=') output[outputOffset++] = ((b & 15) << 4) | (c >> 2);
        if (value[index + 3] !== '=') output[outputOffset++] = ((c & 3) << 6) | d;
    }
    return output.subarray(0, outputOffset);
}

function releaseDecodedBase64(buffer: Buffer): void {
    if (buffer.buffer.byteLength <= MAX_POOLED_BUFFER_SIZE) decodedBufferPool.push(buffer);
}

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
                const buffer = acquireDecodedBase64(line);
                try {
                    yield decodeFn(buffer);
                } finally {
                    releaseDecodedBase64(buffer);
                }
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
            const buffer = acquireDecodedBase64(base64Xdr);
            try {
                return xdr.TransactionMeta.fromXDR(buffer);
            } finally {
                releaseDecodedBase64(buffer);
            }
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
