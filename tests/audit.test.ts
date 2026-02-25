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
