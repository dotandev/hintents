import type { AuditSigner } from './types';
import { SoftwareEd25519Signer } from './softwareSigner';
import { Pkcs11Ed25519Signer } from './pkcs11Signer';
import { KmsEd25519Signer } from './kmsSigner';

export type HsmProvider = 'pkcs11' | 'software' | 'kms';

export function createAuditSigner(opts: {
  hsmProvider?: string;
  softwarePrivateKeyPem?: string;
}): AuditSigner {
  const provider = (opts.hsmProvider?.toLowerCase() ?? 'software') as HsmProvider;

  if (provider === 'kms') {
    return new KmsEd25519Signer();
  }

  if (provider === 'pkcs11') {
    return new Pkcs11Ed25519Signer();
  }

  if (!opts.softwarePrivateKeyPem) {
    throw new Error('software signing selected but no private key was provided');
  }

  return new SoftwareEd25519Signer(opts.softwarePrivateKeyPem);
}
