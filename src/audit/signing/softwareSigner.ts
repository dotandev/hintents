import { sign, createPublicKey } from 'crypto';
import type { AuditSigner, PublicKey, Signature } from './types';

/**
 * Default signer that uses a local Ed25519 private key (PKCS#8 PEM).
 */
export class SoftwareEd25519Signer implements AuditSigner {
  constructor(private readonly privateKeyPem: string) {}

  async sign(payload: Uint8Array): Promise<Signature> {
    try {
      return sign(null, Buffer.from(payload), this.privateKeyPem);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`software signing failed: ${msg}`);
    }
  }

  async public_key(): Promise<PublicKey> {
    try {
      const pub = createPublicKey(this.privateKeyPem);
      return pub.export({ type: 'spki', format: 'pem' }).toString();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`software public key derivation failed: ${msg}`);
    }
  }
}
