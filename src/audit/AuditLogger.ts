// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { createHash } from 'crypto';
import stringify from 'fast-json-stable-stringify';
import type { AuditSigner, HardwareAttestation } from './signing/types';

// Define the structure of the execution trace
export interface ExecutionTrace {
  input: Record<string, any>;
  state: Record<string, any>;
  events: any[];
  timestamp: string; // ISO string
}

export interface SignedAuditLog {
  trace: ExecutionTrace;
  hash: string;
  signature: string;
  algorithm: string;
  publicKey: string;
  signer: {
    provider: string;
    label?: string;
  };
  hardware_attestation?: HardwareAttestation;
}

export interface SignatureEntry {
  signature: string;
  publicKey: string;
  signer: {
    provider: string;
    label?: string;
  };
}

export interface MultiSignedAuditLog {
  trace: ExecutionTrace;
  hash: string;
  algorithm: string;
  signatures: SignatureEntry[];
}

export interface MultiSignerInput {
  signer: AuditSigner;
  provider: string;
  label?: string;
}

export class AuditLogger {
  constructor(
    private readonly signer: AuditSigner,
    private readonly signerProvider: string = 'software'
  ) { }

  /**
   * Generates a deterministic, signed audit log.
   *
   * When the signer is HSM-backed and provides an attestation chain,
   * the chain is embedded in the output under `hardware_attestation`.
   * The attestation data is included in the hash to prevent it from
   * being stripped or swapped after signing.
   */
  public async generateLog(trace: ExecutionTrace): Promise<SignedAuditLog> {
    // 1. Retrieve attestation chain (if available) before hashing
    let attestation: HardwareAttestation | undefined;
    if (typeof this.signer.attestation_chain === 'function') {
      attestation = await this.signer.attestation_chain();
    }

    // 2. Canonicalize the data (Sort keys deterministically)
    // Without this, {a:1, b:2} and {b:2, a:1} would produce different signatures.
    //
    // We include the attestation in the canonical payload so that
    // removing the attestation from the JSON would invalidate the signature.
    const hashInput = attestation
      ? { trace, hardware_attestation: attestation }
      : { trace };
    const canonicalString = stringify(hashInput);

    // 3. Create the Hash (SHA-256 integrity check)
    // We hash the canonical string, not the raw object.
    const traceHash = createHash('sha256').update(canonicalString).digest('hex');

    // Sign the hash bytes (the verifier verifies the same bytes)
    const signatureBuffer = await this.signer.sign(Buffer.from(traceHash));
    const signatureHex = Buffer.from(signatureBuffer).toString('hex');

    const publicKeyPem = await this.signer.public_key();

    const result: SignedAuditLog = {
      trace: trace,
      hash: traceHash,
      signature: signatureHex,
      algorithm: 'Ed25519+SHA256',
      publicKey: publicKeyPem,
      signer: {
        provider: this.signerProvider,
      },
    };

    if (attestation) {
      result.hardware_attestation = attestation;
    }

    return result;
  }

  /**
   * Generates a deterministic audit log with multiple signatures.
   */
  public async generateMultiLog(
    trace: ExecutionTrace,
    signers: MultiSignerInput[]
  ): Promise<MultiSignedAuditLog> {
    const canonicalString = stringify(trace);
    const traceHash = createHash('sha256').update(canonicalString).digest('hex');

    const signatures: SignatureEntry[] = [];
    for (const entry of signers) {
      const signatureBuffer = await entry.signer.sign(Buffer.from(traceHash));
      const signatureHex = Buffer.from(signatureBuffer).toString('hex');
      const publicKeyPem = await entry.signer.public_key();

      signatures.push({
        signature: signatureHex,
        publicKey: publicKeyPem,
        signer: {
          provider: entry.provider,
          label: entry.label,
        },
      });
    }

    return {
      trace,
      hash: traceHash,
      algorithm: 'Ed25519+SHA256',
      signatures,
    };
  }
}
