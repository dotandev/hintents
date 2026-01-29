import { sign, createHash } from 'crypto';
import stringify from 'fast-json-stable-stringify';

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
}

export class AuditLogger {
  private privateKey: string;

  constructor(privateKeyPEM: string) {
    this.privateKey = privateKeyPEM;
  }

  /**
   * Generates a deterministic, signed audit log.
   */
  public generateLog(trace: ExecutionTrace): SignedAuditLog {
    // 1. Canonicalize the data (Sort keys deterministically)
    // Without this, {a:1, b:2} and {b:2, a:1} would produce different signatures.
    const canonicalString = stringify(trace);

    // 2. Create the Hash (SHA-256 integrity check)
    // We hash the canonical string, not the raw object.
    const traceHash = createHash('sha256').update(canonicalString).digest('hex');

    // 3. Sign the Hash (Proof of Authenticity)
    // Using Ed25519 via the crypto module
    const signatureBuffer = sign(
      null, 
      Buffer.from(traceHash), 
      this.privateKey
    );
    
    const signatureHex = signatureBuffer.toString('hex');

    // 4. Return the complete package
    return {
      trace: trace,
      hash: traceHash,
      signature: signatureHex,
      algorithm: 'Ed25519+SHA256'
    };
  }
}