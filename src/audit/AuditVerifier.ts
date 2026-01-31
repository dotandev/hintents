// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { verify, createHash } from 'crypto';
import stringify from 'fast-json-stable-stringify';

export const verifyAuditLog = (
  auditLog: any,
  publicKeyPEM?: string
): boolean => {
  const { trace, hash, signature } = auditLog;

  // 1. Re-calculate the deterministic string
  const canonicalString = stringify(trace);

  // 2. Re-calculate the hash
  const recalculatedHash = createHash('sha256').update(canonicalString).digest('hex');

  // Check 1: Does the data match the hash?
  if (recalculatedHash !== hash) {
    console.error('Integrity Check Failed: Hash mismatch.');
    return false;
  }

  // Check 2: Was the hash signed by the owner of the Public Key?
  const keyToUse = publicKeyPEM ?? auditLog.publicKey;
  if (!keyToUse) {
    console.error('Verification Failed: No public key provided or embedded in audit log.');
    return false;
  }

  const isSignatureValid = verify(
    null,
    Buffer.from(hash),
    keyToUse,
    Buffer.from(signature, 'hex')
  );

  return isSignatureValid;
};