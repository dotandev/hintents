// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { verify, createHash, X509Certificate } from 'crypto';
import stringify from 'fast-json-stable-stringify';
import type { HardwareAttestation } from './signing/types';

export interface VerificationResult {
  /** Overall pass/fail */
  valid: boolean;
  /** Hash integrity check passed */
  hash_valid: boolean;
  /** Signature verification passed */
  signature_valid: boolean;
  /** Hardware attestation verification result (if present) */
  attestation?: AttestationVerification;
}

export interface AttestationVerification {
  /** Whether the attestation chain was present */
  present: boolean;
  /** Whether the certificate chain validates (each cert signed by the next) */
  chain_valid: boolean;
  /** Whether the signing key is marked as non-exportable */
  key_non_exportable: boolean;
  /** Token identification string */
  token_info: string;
  /** Number of certificates in the chain */
  chain_length: number;
  /** Any issues encountered */
  issues: string[];
}

/**
 * Verifies a signed audit log, including hardware attestation if present.
 *
 * @param auditLog  The full signed audit JSON object
 * @param publicKeyPEM  Optional external public key to verify against
 * @returns boolean for backward compatibility (true = valid)
 */
export const verifyAuditLog = (
  auditLog: any,
  publicKeyPEM?: string
): boolean => {
  const result = verifyAuditLogDetailed(auditLog, publicKeyPEM);
  return result.valid;
};

/**
 * Detailed verification that returns granular results, including
 * hardware attestation chain validation.
 */
export const verifyAuditLogDetailed = (
  auditLog: any,
  publicKeyPEM?: string
): VerificationResult => {
  const { trace, hash, signature, hardware_attestation } = auditLog;

  // 1. Re-calculate the deterministic string
  // Must match the hashing logic in AuditLogger:
  // if attestation is present, include it in the hash input.
  const hashInput = hardware_attestation
    ? { trace, hardware_attestation }
    : { trace };
  const canonicalString = stringify(hashInput);

  // 2. Re-calculate the hash
  const recalculatedHash = createHash('sha256').update(canonicalString).digest('hex');

  const hashValid = recalculatedHash === hash;
  if (!hashValid) {
    return {
      valid: false,
      hash_valid: false,
      signature_valid: false,
      attestation: hardware_attestation
        ? buildAttestationResult(hardware_attestation, ['skipped: hash mismatch'])
        : undefined,
    };
  }

  // 3. Verify signature
  const keyToUse = publicKeyPEM ?? auditLog.publicKey;
  if (!keyToUse) {
    return {
      valid: false,
      hash_valid: true,
      signature_valid: false,
    };
  }

  let signatureValid: boolean;
  try {
    signatureValid = verify(
      null,
      Buffer.from(hash),
      keyToUse,
      Buffer.from(signature, 'hex')
    );
  } catch {
    signatureValid = false;
  }

  // 4. Verify attestation chain if present
  let attestationResult: AttestationVerification | undefined;
  if (hardware_attestation) {
    attestationResult = verifyAttestationChain(hardware_attestation);
  }

  const attestationOk = !attestationResult || attestationResult.chain_valid;

  return {
    valid: hashValid && signatureValid && attestationOk,
    hash_valid: hashValid,
    signature_valid: signatureValid,
    attestation: attestationResult,
  };
};

function verifyAttestationChain(attestation: HardwareAttestation): AttestationVerification {
  const issues: string[] = [];
  const certs = attestation.certificates;

  if (!certs || certs.length === 0) {
    return {
      present: false,
      chain_valid: false,
      key_non_exportable: attestation.key_non_exportable,
      token_info: attestation.token_info,
      chain_length: 0,
      issues: ['no certificates in attestation chain'],
    };
  }

  // Validate each certificate can be parsed
  const parsed: X509Certificate[] = [];
  for (let i = 0; i < certs.length; i++) {
    try {
      parsed.push(new X509Certificate(certs[i].pem));
    } catch (e) {
      issues.push(`certificate[${i}]: failed to parse: ${e instanceof Error ? e.message : String(e)}`);
    }
  }

  // Validate chain: each cert should be issued by the next
  let chainValid = parsed.length === certs.length;
  for (let i = 0; i < parsed.length - 1; i++) {
    try {
      const issuedBy = parsed[i].checkIssued(parsed[i + 1]);
      if (!issuedBy) {
        issues.push(`certificate[${i}] is not issued by certificate[${i + 1}]`);
        chainValid = false;
      }
    } catch (e) {
      issues.push(`chain validation error at index ${i}: ${e instanceof Error ? e.message : String(e)}`);
      chainValid = false;
    }
  }

  // Warn if key is exportable (this defeats the purpose of HSM attestation)
  if (!attestation.key_non_exportable) {
    issues.push('private key is not marked as non-exportable on the token');
  }

  return {
    present: true,
    chain_valid: chainValid,
    key_non_exportable: attestation.key_non_exportable,
    token_info: attestation.token_info,
    chain_length: certs.length,
    issues,
  };
}

function buildAttestationResult(
  attestation: HardwareAttestation,
  extraIssues: string[]
): AttestationVerification {
  return {
    present: true,
    chain_valid: false,
    key_non_exportable: attestation.key_non_exportable,
    token_info: attestation.token_info,
    chain_length: attestation.certificates?.length ?? 0,
    issues: extraIssues,
  };
}