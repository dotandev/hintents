import { verify } from 'crypto';
import { AuditLogger } from '../src/audit/AuditLogger';
import { verifyAuditLog } from '../src/audit/AuditVerifier';
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
// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

// Change these back to end with .ts
import { generateAuditKeys } from '../src/utils/cryptoUtils.ts';
import { AuditLogger } from '../src/audit/AuditLogger.ts';
import { verifyAuditLog } from '../src/audit/AuditVerifier.ts';

const runTest = () => {
  console.log('--- Starting Audit Log Test ---');

  // 1. Setup Keys (In prod, these come from Vault/Env)
  const { publicKey, privateKey } = generateAuditKeys();

  // 2. Mock Trace Data
  const traceData = {
    input: { amount: 100, currency: 'USD', user_id: 'u_123' },
    state: { balance_before: 500, balance_after: 400 },
    events: ['INIT_TRANSFER', 'DEBIT_ACCOUNT', 'FEE_CALC'],
    timestamp: new Date().toISOString()
  };

  // 3. Generate Log
  const logger = new AuditLogger(privateKey);
  const secureLog = logger.generateLog(traceData);

  console.log('Generated Log:', JSON.stringify(secureLog, null, 2));

  // 4. Verify Log
  const isValid = verifyAuditLog(secureLog, publicKey);
  
  if (isValid) {
    console.log('✅ SUCCESS: Signature and Hash verified.');
  } else {
    console.error('❌ FAILED: Invalid log.');
    process.exit(1);
  }

  // 5. Tamper Test (Optional but recommended)
  console.log('\n--- Tampering Data ---');
  secureLog.trace.input.amount = 999999; // Hacker changes amount
  const isTamperValid = verifyAuditLog(secureLog, publicKey);
  
  if (!isTamperValid) {
    console.log('✅ SUCCESS: Tampered data detected.');
  } else {
    console.error('❌ FAILED: Tampered data was accepted.');
  }
};

runTest();
