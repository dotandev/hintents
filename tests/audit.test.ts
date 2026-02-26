// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { verify } from 'crypto';
import { AuditLogger } from '../src/audit/AuditLogger';
import { verifyAuditLog, verifyAuditLogDetailed } from '../src/audit/AuditVerifier';
import { MockAuditSigner } from '../src/audit/signing/mockSigner';

describe('audit signing (signer-agnostic)', () => {
  test('signs an audit payload and verifies using returned public key', async () => {
    const signer = new MockAuditSigner();
    const logger = new AuditLogger(signer, 'mock');

    const traceData = {
      input: { amount: 100, currency: 'USD', user_id: 'u_123' },
      state: { balance_before: 500, balance_after: 400 },
      events: ['INIT_TRANSFER', 'DEBIT_ACCOUNT', 'FEE_CALC'],
      timestamp: new Date().toISOString(),
    };

    const secureLog = await logger.generateLog(traceData as any);

    // Verifier should work without separately supplying a key (uses embedded publicKey)
    expect(verifyAuditLog(secureLog)).toBe(true);

    // Also verify explicitly using Node crypto verify, to ensure public_key() is consistent
    const ok = verify(
      null,
      Buffer.from(secureLog.hash),
      secureLog.publicKey,
      Buffer.from(secureLog.signature, 'hex')
    );
    expect(ok).toBe(true);
  });

  test('detects tampering', async () => {
    const signer = new MockAuditSigner();
    const logger = new AuditLogger(signer, 'mock');

    const traceData = {
      input: { amount: 100 },
      state: { balance_before: 500, balance_after: 400 },
      events: ['INIT_TRANSFER'],
      timestamp: new Date().toISOString(),
    };

    const secureLog: any = await logger.generateLog(traceData as any);
    secureLog.trace.input.amount = 999999;

    expect(verifyAuditLog(secureLog)).toBe(false);
  });
});

describe('audit with hardware attestation', () => {
  test('embeds attestation chain when signer provides it', async () => {
    const signer = new MockAuditSigner({ withAttestation: true });
    const logger = new AuditLogger(signer, 'hsm-mock');

    const traceData = {
      input: { amount: 50 },
      state: { balance_before: 200, balance_after: 150 },
      events: ['TRANSFER'],
      timestamp: new Date().toISOString(),
    };

    const secureLog = await logger.generateLog(traceData as any);

    // Attestation should be present
    expect(secureLog.hardware_attestation).toBeDefined();
    expect(secureLog.hardware_attestation!.certificates.length).toBe(2);
    expect(secureLog.hardware_attestation!.key_non_exportable).toBe(true);
    expect(secureLog.hardware_attestation!.token_info).toContain('MockHSM');

    // Hash and signature should be valid even though mock certs are not real X.509
    const result = verifyAuditLogDetailed(secureLog);
    expect(result.hash_valid).toBe(true);
    expect(result.signature_valid).toBe(true);
    expect(result.attestation).toBeDefined();
    expect(result.attestation!.present).toBe(true);
    // chain_valid may be false for mock certs (not real X.509), which is expected
  });

  test('does not include attestation for software signer', async () => {
    const signer = new MockAuditSigner(); // no attestation
    const logger = new AuditLogger(signer, 'software');

    const traceData = {
      input: { x: 1 },
      state: {},
      events: [],
      timestamp: new Date().toISOString(),
    };

    const secureLog = await logger.generateLog(traceData as any);
    expect(secureLog.hardware_attestation).toBeUndefined();
    expect(verifyAuditLog(secureLog)).toBe(true);
  });

  test('detects tampering with attestation data', async () => {
    const signer = new MockAuditSigner({ withAttestation: true });
    const logger = new AuditLogger(signer, 'hsm-mock');

    const traceData = {
      input: { amount: 100 },
      state: {},
      events: [],
      timestamp: new Date().toISOString(),
    };

    const secureLog: any = await logger.generateLog(traceData as any);

    // Tamper with attestation
    secureLog.hardware_attestation.key_non_exportable = false;

    expect(verifyAuditLog(secureLog)).toBe(false);
  });

  test('detects stripped attestation', async () => {
    const signer = new MockAuditSigner({ withAttestation: true });
    const logger = new AuditLogger(signer, 'hsm-mock');

    const traceData = {
      input: { amount: 100 },
      state: {},
      events: [],
      timestamp: new Date().toISOString(),
    };

    const secureLog: any = await logger.generateLog(traceData as any);

    // Remove attestation entirely
    delete secureLog.hardware_attestation;

    // Hash will mismatch because attestation was included in the hash
    expect(verifyAuditLog(secureLog)).toBe(false);
  });

  test('detailed verification returns attestation info', async () => {
    const signer = new MockAuditSigner({ withAttestation: true });
    const logger = new AuditLogger(signer, 'hsm-mock');

    const traceData = {
      input: { y: 2 },
      state: {},
      events: [],
      timestamp: new Date().toISOString(),
    };

    const secureLog = await logger.generateLog(traceData as any);
    const result = verifyAuditLogDetailed(secureLog);

    expect(result.hash_valid).toBe(true);
    expect(result.signature_valid).toBe(true);
    expect(result.attestation).toBeDefined();
    expect(result.attestation!.present).toBe(true);
    expect(result.attestation!.key_non_exportable).toBe(true);
    expect(result.attestation!.chain_length).toBe(2);
  });
});
