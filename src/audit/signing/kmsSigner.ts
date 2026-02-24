import { KMSClient, SignCommand } from '@aws-sdk/client-kms';
import type { AuditSigner, PublicKey, Signature } from './types';

export class KmsEd25519Signer implements AuditSigner {
  private readonly cfg = {
    region: process.env.ERST_KMS_REGION || 'us-east-1',
    keyId: process.env.ERST_KMS_KEY_ID,
    publicKeyPem: process.env.ERST_KMS_PUBLIC_KEY_PEM,
  };

  private readonly client: KMSClient;

  constructor() {
    if (!this.cfg.keyId) {
      throw new Error(
        'KMS signer selected but ERST_KMS_KEY_ID is not set'
      );
    }
    if (!this.cfg.publicKeyPem) {
      throw new Error(
        'KMS signer selected but ERST_KMS_PUBLIC_KEY_PEM is not set'
      );
    }
    this.client = new KMSClient({ region: this.cfg.region });
  }

  async sign(payload: Uint8Array): Promise<Signature> {
    try {
      const command = new SignCommand({
        KeyId: this.cfg.keyId,
        Message: payload,
        SigningAlgorithm: 'Ed25519' as any,
      });
      const response = await this.client.send(command);
      if (!response.Signature) {
        throw new Error('KMS Sign response missing signature');
      }
      return Buffer.from(response.Signature);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`kms signing failed: ${msg}`);
    }
  }

  async public_key(): Promise<PublicKey> {
    return this.cfg.publicKeyPem!;
  }
}
