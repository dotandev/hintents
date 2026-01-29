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