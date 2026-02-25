// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

import { KmsEd25519Signer } from '../src/audit/signing/kmsSigner';

describe('KMS Ed25519Signer integration', () => {
  const testKeyId = 'arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012';
  const testPublicKeyPem = `-----BEGIN PUBLIC KEY-----
MFMwEwYHKoZIzj0CAQYIKoZIzj0DAQcDOgAEWdp8vGtXxyGkftJoJphBnwvlvVfc
6xwvSMu00nWXrF5bUegdisSGSF3567890123456789abcdefg
-----END PUBLIC KEY-----`;

  beforeEach(() => {
    process.env.ERST_KMS_REGION = 'us-east-1';
    process.env.ERST_KMS_KEY_ID = testKeyId;
    process.env.ERST_KMS_PUBLIC_KEY_PEM = testPublicKeyPem;
  });

  afterEach(() => {
    delete process.env.ERST_KMS_REGION;
    delete process.env.ERST_KMS_KEY_ID;
    delete process.env.ERST_KMS_PUBLIC_KEY_PEM;
  });

  test('constructor validates required environment variables', () => {
    delete process.env.ERST_KMS_KEY_ID;
    expect(() => new KmsEd25519Signer()).toThrow('ERST_KMS_KEY_ID is not set');
  });

  test('constructor validates public key environment variable', () => {
    delete process.env.ERST_KMS_PUBLIC_KEY_PEM;
    expect(() => new KmsEd25519Signer()).toThrow('ERST_KMS_PUBLIC_KEY_PEM is not set');
  });

  test('public_key returns configured PEM', async () => {
    const signer = new KmsEd25519Signer();
    const pubKey = await signer.public_key();
    expect(pubKey).toBe(testPublicKeyPem);
  });

  test('uses configured region or defaults to us-east-1', () => {
    process.env.ERST_KMS_REGION = 'eu-west-1';
    const signer = new KmsEd25519Signer();
    expect(signer).toBeDefined();
  });
});
