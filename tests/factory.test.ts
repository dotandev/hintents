// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { createAuditSigner } from '../src/audit/signing/factory';
import { KmsEd25519Signer } from '../src/audit/signing/kmsSigner';

describe('Audit signer factory', () => {
  const mockKeyId = 'arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012';
  const mockPublicKeyPem = `-----BEGIN PUBLIC KEY-----
MFMwEwYHKoZIzj0CAQYIKoZIzj0DAQcDOgAEWdp8vGtXxyGkftJoJphBnwvlvVfc
6xwvSMu00nWXrF5bUegdisSGSF3567890123456789abcdefg
-----END PUBLIC KEY-----`;

  beforeEach(() => {
    process.env.ERST_KMS_KEY_ID = mockKeyId;
    process.env.ERST_KMS_PUBLIC_KEY_PEM = mockPublicKeyPem;
  });

  afterEach(() => {
    delete process.env.ERST_KMS_KEY_ID;
    delete process.env.ERST_KMS_PUBLIC_KEY_PEM;
  });

  test('creates KMS signer when provider is kms', () => {
    const signer = createAuditSigner({ hsmProvider: 'kms' });
    expect(signer).toBeInstanceOf(KmsEd25519Signer);
  });

  test('respects case-insensitive provider selection', () => {
    const signer = createAuditSigner({ hsmProvider: 'KMS' });
    expect(signer).toBeInstanceOf(KmsEd25519Signer);
  });

  test('defaults to software signer when no provider specified', () => {
    const privateKey = process.env.TEST_PRIVATE_KEY_PEM || 'test-key-placeholder';
    const signer = createAuditSigner({ softwarePrivateKeyPem: privateKey });
    expect(signer).toBeDefined();
  });
});
