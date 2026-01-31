import { generateKeyPairSync, sign as nodeSign } from 'crypto';
import type { AuditSigner, PublicKey, Signature } from './types';

/**
 * In-memory signer used for unit tests; does not require a real HSM.
 */
export class MockAuditSigner implements AuditSigner {
  private readonly privateKeyPem: string;
  private readonly publicKeyPem: string;

  constructor() {
    const { publicKey, privateKey } = generateKeyPairSync('ed25519', {
      publicKeyEncoding: { type: 'spki', format: 'pem' },
      privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
    });
    this.privateKeyPem = privateKey;
    this.publicKeyPem = publicKey;
  }

  async sign(payload: Uint8Array): Promise<Signature> {
    return nodeSign(null, Buffer.from(payload), this.privateKeyPem);
  }

  async public_key(): Promise<PublicKey> {
    return this.publicKeyPem;
  }
}
