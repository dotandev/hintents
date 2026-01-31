// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { sign, createHash } from 'crypto';
import stringify from 'fast-json-stable-stringify';
import type { AuditSigner } from './signing/types';

// Define the structure of the execution trace
interface ExecutionTrace {
  input: Record<string, any>;
  state: Record<string, any>;
  events: any[];
  timestamp: string; // ISO string
}

interface SignedAuditLog {
  trace: ExecutionTrace;
  hash: string;
  signature: string;
  algorithm: string;
  publicKey: string;
  signer: {
    provider: string;
  };
}

export class AuditLogger {
  constructor(
    private readonly signer: AuditSigner,
    private readonly signerProvider: string = 'software'
  ) {}

  /**
   * Generates a deterministic, signed audit log.
   */
  public async generateLog(trace: ExecutionTrace): Promise<SignedAuditLog> {
    // 1. Canonicalize the data (Sort keys deterministically)
    // Without this, {a:1, b:2} and {b:2, a:1} would produce different signatures.
    const canonicalString = stringify(trace);

    // 2. Create the Hash (SHA-256 integrity check)
    // We hash the canonical string, not the raw object.
    const traceHash = createHash('sha256').update(canonicalString).digest('hex');

    // Sign the hash bytes (the verifier verifies the same bytes)
    const signatureBuffer = await this.signer.sign(Buffer.from(traceHash));
    const signatureHex = Buffer.from(signatureBuffer).toString('hex');

    const publicKeyPem = await this.signer.public_key();

    return {
      trace: trace,
      hash: traceHash,
      signature: signatureHex,
      algorithm: 'Ed25519+SHA256',
      publicKey: publicKeyPem,
      signer: {
        provider: this.signerProvider,
      },
    };
  }
}