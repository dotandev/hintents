// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { xdr } from '@stellar/stellar-sdk';

/**
 * Real transaction meta samples for testing
 * These are simplified examples - in production, use actual transaction meta from Horizon/RPC
 */

// Sample payment transaction meta (v2)
export const PAYMENT_TX_META_V2 = `AAAAAgAAAAMAAAADAAYHEQAAAAAAAAAAR...`;

// Sample create account transaction meta (v2)
export const CREATE_ACCOUNT_TX_META_V2 = `AAAAAgAAAAMAAAADAAYHEgAAAAAAAAAAR...`;

// Sample Soroban contract invocation meta (v3)
export const SOROBAN_TX_META_V3 = `AAAAAwAAAAMAAAADAAYHEwAAAAAAAAAAR...`;

// Sample complex multi-operation transaction meta (v2)
export const COMPLEX_TX_META_V2 = `AAAAAgAAAAMAAAADAAYHFAAAAAAAAAAAR...`;

/**
 * Build a valid TransactionMeta V3 base64 XDR string from LedgerEntryChange items.
 */
export function buildConfigSettingTxMetaV3(
    txChangesBefore: xdr.LedgerEntryChange[],
    operationChanges: xdr.LedgerEntryChange[] = []
): string {
    const operations =
        operationChanges.length > 0
            ? [new xdr.OperationMeta({ changes: operationChanges })]
            : [];

    const meta = new xdr.TransactionMeta(
        3,
        new xdr.TransactionMetaV3({
            ext: new xdr.ExtensionPoint(0),
            txChangesBefore,
            operations,
            txChangesAfter: [],
            sorobanMeta: null,
        })
    );

    return meta.toXDR('base64');
}

/**
 * Build a LedgerEntry wrapping a ConfigSettingEntry.
 */
export function buildConfigSettingLedgerEntry(
    configSetting: xdr.ConfigSettingEntry,
    lastModifiedLedgerSeq = 12345
): xdr.LedgerEntry {
    return new xdr.LedgerEntry({
        lastModifiedLedgerSeq,
        data: xdr.LedgerEntryData.configSetting(configSetting),
        ext: new xdr.LedgerEntryExt(0),
    });
}

// Valid TransactionMeta V3 fixture reading ConfigSettingEntry (contractMaxSizeBytes = 65536)
export const CONFIG_SETTING_TX_META_V3 = buildConfigSettingTxMetaV3([
    xdr.LedgerEntryChange.ledgerEntryState(
        buildConfigSettingLedgerEntry(
            xdr.ConfigSettingEntry.configSettingContractMaxSizeBytes(65536)
        )
    ),
]);
