// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { xdr } from '@stellar/stellar-sdk';

export interface LedgerKey {
    type: xdr.LedgerEntryType;
    key: string;
    hash: string;
}

export interface FootprintResult {
    readOnly: LedgerKey[];
    readWrite: LedgerKey[];
    all: LedgerKey[];
}
