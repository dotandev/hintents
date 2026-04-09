// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { ContractID } from './sdk';

type PublicKeyPEM = string & { readonly __brand: 'PublicKeyPEM' };

/**
 * Validates a raw string as a ContractID.
 */
export function isContractID(id: string): id is ContractID {
  // Enforces stricter format: must start with 'C' and be 56 chars
  return /^C[A-Z0-9]{55}$/.test(id);
}

/**
 * Assertion guard to enforce strict linting at the entry point of operations.
 */
export function assertContractID(id: string): asserts id is ContractID {
  if (!isContractID(id)) {
    throw new Error(`Invalid ContractID: ${id}. Strict linting rules require a 56-character 'C' address.`);
  }
}

/**
 * Validates if a string is a valid SPKI PEM Public Key.
 */
export function isPublicKeyPEM(key: string): key is PublicKeyPEM {
  return key.startsWith('-----BEGIN PUBLIC KEY-----') && key.endsWith('-----END PUBLIC KEY-----\n');
}