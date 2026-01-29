import { verify, createHash } from 'crypto';
import stringify from 'fast-json-stable-stringify';

export const verifyAuditLog = (
  auditLog: any, 
  publicKeyPEM: string
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
  const isSignatureValid = verify(
    null,
    Buffer.from(hash),
    publicKeyPEM,
    Buffer.from(signature, 'hex')
  );

  return isSignatureValid;
};