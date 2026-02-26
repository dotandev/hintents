// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import type { AuditSigner } from './types';
import { SoftwareEd25519Signer } from './softwareSigner';
import { Pkcs11Signer } from './pkcs11Signer';
import { KmsEd25519Signer } from './kmsSigner';

export type SigningProvider = 'software' | 'pkcs11' | 'kms';

export interface CreateAuditSignerOpts {
  hsmProvider?: string;
  softwarePrivateKeyPem?: string;
  kmsKeyId?: string;
  kmsSigningAlgorithm?: string;
}

export function createAuditSigner(opts: CreateAuditSignerOpts): AuditSigner {
  const provider = (opts.hsmProvider?.toLowerCase() ?? 'software') as SigningProvider;

  switch (provider) {
    case 'kms':
      // Return KMS signer with algorithm support
      return new KmsEd25519Signer();

    case 'pkcs11':
      // The Pkcs11Signer now handles algorithm choice via ERST_PKCS11_ALGORITHM env var
      return new Pkcs11Signer();

    case 'software':
      if (!opts.softwarePrivateKeyPem) {
        throw new Error('software signing selected but no private key was provided');
      }
      return new SoftwareEd25519Signer(opts.softwarePrivateKeyPem);

    default:
      throw new Error(`unknown signing provider: "${provider}". Valid options: software, pkcs11, kms`);
  }
}