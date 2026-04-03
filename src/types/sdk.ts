// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

export type PublicKeyPEM = Brand<string, 'PublicKeyPEM'>;
// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

// src/types/sdk.ts
// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

/**
 * Branded Type Utility
 * Prevents accidental assignment of identical primitive types.
 */
type Brand<K, T> = K & { __brand: T };

/** * Stricter SDK Types 
 * These ensure that a ContractID isn't swapped for a TransactionID 
 */
export type ContractID = Brand<string, 'ContractID'>;
export type TransactionID = Brand<string, 'TransactionID'>;
export type KeyID = Brand<string, 'KeyID'>;

/**
 * SDK Configuration Interface
 * Enforces strict property requirements for SDK initialization.
 */
export interface SDKConfig {
  readonly network: 'mainnet' | 'testnet' | 'futurenet';
  readonly rpcUrl: string;
  readonly timeoutMs: number;
}