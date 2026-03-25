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

// SDK middleware types for custom injection

export interface SDKContext {
    path: string;
    method: string;
    data?: any;
    headers?: Record<string, string>;
    metadata: Record<string, unknown>;
}

export interface SDKResponse<T = any> {
    data: T;
    status: number;
    duration: number;
    endpoint: string;
    metadata: Record<string, unknown>;
}

export type NextFn<T = any> = (ctx: SDKContext) => Promise<SDKResponse<T>>;

export type SDKMiddleware<T = any> = (
    ctx: SDKContext,
    next: NextFn<T>,
) => Promise<SDKResponse<T>>;

// Composes an array of middleware into a single chain around a core handler.
export function composeMiddleware<T = any>(
    middlewares: SDKMiddleware<T>[],
    core: NextFn<T>,
): NextFn<T> {
    return middlewares.reduceRight<NextFn<T>>(
        (next, mw) => (ctx) => mw(ctx, next),
        core,
    );
}
