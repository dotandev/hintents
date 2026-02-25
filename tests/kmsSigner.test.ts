// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { KmsSigner } from '../src/audit/signing/kmsSigner';
import { createAuditSigner } from '../src/audit/signing/factory';
import { AuditLogger } from '../src/audit/AuditLogger';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Builds a minimal mock for @aws-sdk/client-kms. We intercept require() so
 * the test suite runs without the optional AWS SDK installed.
 */
function buildKmsMock(overrides: {
  signatureBytes?: Buffer;
  publicKeyBytes?: Buffer;
  signError?: Error;
  publicKeyError?: Error;
} = {}) {
  const signatureBytes = overrides.signatureBytes ?? Buffer.from('fakesignature', 'utf8');
  const publicKeyBytes =
    overrides.publicKeyBytes ??
    // Minimal DER sequence prefix + 32 zero bytes to simulate an SPKI blob.
    Buffer.concat([Buffer.from([0x30, 0x2a]), Buffer.alloc(40)]);

  const send = jest.fn(async (command: any) => {
    const name: string = command.constructor?.name ?? '';

    if (name === 'SignCommand') {
      if (overrides.signError) throw overrides.signError;
      return { Signature: signatureBytes };
    }

    if (name === 'GetPublicKeyCommand') {
      if (overrides.publicKeyError) throw overrides.publicKeyError;
      return { PublicKey: publicKeyBytes };
    }

    throw new Error(`unexpected KMS command: ${name}`);
  });

  const KMSClient = jest.fn().mockImplementation(() => ({ send }));
  const SignCommand = jest.fn().mockImplementation((input: any) => {
    return { constructor: { name: 'SignCommand' }, input };
  });
  const GetPublicKeyCommand = jest.fn().mockImplementation((input: any) => {
    return { constructor: { name: 'GetPublicKeyCommand' }, input };
  });

  return { KMSClient, SignCommand, GetPublicKeyCommand, send };
}

// Stable sentinel key used to inject and remove the KMS mock from require.cache.
// Using a fixed string avoids calling require.resolve() on an absent optional module.
const KMS_MODULE_ID = '@aws-sdk/client-kms';

// eslint-disable-next-line no-eval
const _require: any = eval('require');

/**
 * Injects a mock KMS module into the require cache so that KmsSigner's lazy
 * eval('require')('@aws-sdk/client-kms') resolves to our mock without the real
 * SDK being installed.
 */
function injectKmsMock(mock: ReturnType<typeof buildKmsMock>): void {
  const mod = {
    KMSClient: mock.KMSClient,
    SignCommand: mock.SignCommand,
    GetPublicKeyCommand: mock.GetPublicKeyCommand,
  };
  _require.cache[KMS_MODULE_ID] = {
    id: KMS_MODULE_ID,
    filename: KMS_MODULE_ID,
    loaded: true,
    parent: null as any,
    children: [],
    exports: mod,
    paths: [],
  } as any;
}

function removeKmsCacheEntry(): void {
  delete _require.cache[KMS_MODULE_ID];
}

// ---------------------------------------------------------------------------
// Environment setup helpers
// ---------------------------------------------------------------------------

function setKmsEnv() {
  process.env.ERST_KMS_KEY_ID = 'arn:aws:kms:us-east-1:123456789012:key/test-key-id';
  process.env.AWS_REGION = 'us-east-1';
}

function clearKmsEnv() {
  delete process.env.ERST_KMS_KEY_ID;
  delete process.env.AWS_REGION;
  delete process.env.ERST_KMS_SIGNING_ALGORITHM;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('KmsSigner', () => {
  beforeEach(() => {
    setKmsEnv();
  });

  afterEach(() => {
    clearKmsEnv();
    removeKmsCacheEntry();
    jest.clearAllMocks();
  });

  // -------------------------------------------------------------------------
  // Constructor validation
  // -------------------------------------------------------------------------

  describe('constructor', () => {
    it('throws when ERST_KMS_KEY_ID is missing', () => {
      delete process.env.ERST_KMS_KEY_ID;
      expect(() => new KmsSigner()).toThrow('ERST_KMS_KEY_ID is required');
    });

    it('throws when AWS_REGION is missing', () => {
      delete process.env.AWS_REGION;
      expect(() => new KmsSigner()).toThrow('AWS_REGION is required');
    });

    it('accepts keyId and region from constructor options', () => {
      // Env vars are cleared; options take precedence.
      clearKmsEnv();
      expect(
        () => new KmsSigner({ keyId: 'alias/my-key', region: 'eu-west-1' })
      ).not.toThrow();
    });

    it('constructor option keyId overrides env var', () => {
      const signer = new KmsSigner({ keyId: 'alias/override', region: 'us-east-1' });
      // Access private field via cast to verify the value is stored.
      expect((signer as any).keyId).toBe('alias/override');
    });

    it('defaults signing algorithm to ECDSA_SHA_256', () => {
      const signer = new KmsSigner();
      expect((signer as any).signingAlgorithm).toBe('ECDSA_SHA_256');
    });

    it('uses ERST_KMS_SIGNING_ALGORITHM env var when set', () => {
      process.env.ERST_KMS_SIGNING_ALGORITHM = 'RSASSA_PSS_SHA_256';
      const signer = new KmsSigner();
      expect((signer as any).signingAlgorithm).toBe('RSASSA_PSS_SHA_256');
    });

    it('uses kmsSigningAlgorithm constructor option over env var', () => {
      process.env.ERST_KMS_SIGNING_ALGORITHM = 'RSASSA_PSS_SHA_256';
      const signer = new KmsSigner({ signingAlgorithm: 'ECDSA_SHA_512' });
      expect((signer as any).signingAlgorithm).toBe('ECDSA_SHA_512');
    });
  });

  // -------------------------------------------------------------------------
  // sign()
  // -------------------------------------------------------------------------

  describe('sign()', () => {
    it('returns the signature bytes from KMS as a Buffer', async () => {
      const expectedSig = Buffer.from('kms-signature-bytes');
      const mock = buildKmsMock({ signatureBytes: expectedSig });
      injectKmsMock(mock);

      const signer = new KmsSigner();
      const result = await signer.sign(Buffer.from('digest'));

      expect(Buffer.compare(result, expectedSig)).toBe(0);
    });

    it('calls KMS SignCommand with correct parameters', async () => {
      const mock = buildKmsMock();
      injectKmsMock(mock);

      const signer = new KmsSigner();
      const digest = Buffer.from('test-digest-bytes');
      await signer.sign(digest);

      expect(mock.SignCommand).toHaveBeenCalledWith({
        KeyId: process.env.ERST_KMS_KEY_ID,
        Message: digest,
        MessageType: 'DIGEST',
        SigningAlgorithm: 'ECDSA_SHA_256',
      });
    });

    it('passes the configured signing algorithm to KMS', async () => {
      process.env.ERST_KMS_SIGNING_ALGORITHM = 'ECDSA_SHA_512';
      const mock = buildKmsMock();
      injectKmsMock(mock);

      const signer = new KmsSigner();
      await signer.sign(Buffer.from('digest'));

      expect(mock.SignCommand).toHaveBeenCalledWith(
        expect.objectContaining({ SigningAlgorithm: 'ECDSA_SHA_512' })
      );
    });

    it('wraps KMS API errors with a descriptive message', async () => {
      const mock = buildKmsMock({ signError: new Error('AccessDeniedException') });
      injectKmsMock(mock);

      const signer = new KmsSigner();
      await expect(signer.sign(Buffer.from('digest'))).rejects.toThrow(
        'KMS signing failed: AccessDeniedException'
      );
    });

    it('throws when KMS response contains no Signature field', async () => {
      const mock = buildKmsMock();
      mock.send.mockResolvedValueOnce({});
      injectKmsMock(mock);

      const signer = new KmsSigner();
      await expect(signer.sign(Buffer.from('digest'))).rejects.toThrow(
        'response contained no Signature field'
      );
    });

    it('reuses the same KMS client across multiple calls', async () => {
      const mock = buildKmsMock();
      injectKmsMock(mock);

      const signer = new KmsSigner();
      await signer.sign(Buffer.from('first'));
      await signer.sign(Buffer.from('second'));

      // KMSClient constructor should only be called once.
      expect(mock.KMSClient).toHaveBeenCalledTimes(1);
    });
  });

  // -------------------------------------------------------------------------
  // public_key()
  // -------------------------------------------------------------------------

  describe('public_key()', () => {
    it('returns a PEM-wrapped public key string', async () => {
      const mock = buildKmsMock();
      injectKmsMock(mock);

      const signer = new KmsSigner();
      const pem = await signer.public_key();

      expect(pem).toMatch(/^-----BEGIN PUBLIC KEY-----/);
      expect(pem).toMatch(/-----END PUBLIC KEY-----/);
    });

    it('calls GetPublicKeyCommand with the configured key ID', async () => {
      const mock = buildKmsMock();
      injectKmsMock(mock);

      const signer = new KmsSigner();
      await signer.public_key();

      expect(mock.GetPublicKeyCommand).toHaveBeenCalledWith({
        KeyId: process.env.ERST_KMS_KEY_ID,
      });
    });

    it('wraps KMS API errors with a descriptive message', async () => {
      const mock = buildKmsMock({ publicKeyError: new Error('KeyNotFoundException') });
      injectKmsMock(mock);

      const signer = new KmsSigner();
      await expect(signer.public_key()).rejects.toThrow(
        'KMS GetPublicKey failed: KeyNotFoundException'
      );
    });

    it('throws when KMS response contains no PublicKey field', async () => {
      const mock = buildKmsMock();
      mock.send.mockResolvedValueOnce({});
      injectKmsMock(mock);

      const signer = new KmsSigner();
      await expect(signer.public_key()).rejects.toThrow(
        'response contained no PublicKey field'
      );
    });
  });

  // -------------------------------------------------------------------------
  // Missing optional dependency
  // -------------------------------------------------------------------------

  describe('missing @aws-sdk/client-kms', () => {
    it('throws a helpful error when the SDK is not installed', async () => {
      // Remove cache entry so require() will attempt a real resolution and fail.
      removeKmsCacheEntry();

      const signer = new KmsSigner();
      // Both sign() and public_key() call loadKmsModule(); test one of them.
      await expect(signer.sign(Buffer.from('x'))).rejects.toThrow(
        '@aws-sdk/client-kms'
      );
    });
  });
});

// ---------------------------------------------------------------------------
// factory integration
// ---------------------------------------------------------------------------

describe('createAuditSigner – kms provider', () => {
  beforeEach(() => setKmsEnv());
  afterEach(() => {
    clearKmsEnv();
    removeKmsCacheEntry();
    jest.clearAllMocks();
  });

  it('returns a KmsSigner when provider is "kms"', () => {
    const signer = createAuditSigner({ hsmProvider: 'kms' });
    expect(signer).toBeInstanceOf(KmsSigner);
  });

  it('is case-insensitive for the provider string', () => {
    const signer = createAuditSigner({ hsmProvider: 'KMS' });
    expect(signer).toBeInstanceOf(KmsSigner);
  });

  it('forwards kmsKeyId option to KmsSigner', () => {
    const signer = createAuditSigner({ hsmProvider: 'kms', kmsKeyId: 'alias/ci-key' });
    expect((signer as any).keyId).toBe('alias/ci-key');
  });

  it('forwards kmsSigningAlgorithm option to KmsSigner', () => {
    const signer = createAuditSigner({
      hsmProvider: 'kms',
      kmsSigningAlgorithm: 'RSASSA_PSS_SHA_384',
    });
    expect((signer as any).signingAlgorithm).toBe('RSASSA_PSS_SHA_384');
  });

  it('throws for an unknown provider string', () => {
    expect(() => createAuditSigner({ hsmProvider: 'vault' })).toThrow(
      'unknown signing provider'
    );
  });
});

// ---------------------------------------------------------------------------
// AuditLogger integration with KmsSigner
// ---------------------------------------------------------------------------

describe('AuditLogger with KmsSigner', () => {
  beforeEach(() => setKmsEnv());
  afterEach(() => {
    clearKmsEnv();
    removeKmsCacheEntry();
    jest.clearAllMocks();
  });

  it('produces a signed audit log with kms provider label', async () => {
    const sigBytes = Buffer.from('kms-sig');
    const pubKeyBytes = Buffer.concat([Buffer.from([0x30, 0x1a]), Buffer.alloc(26)]);
    const mock = buildKmsMock({ signatureBytes: sigBytes, publicKeyBytes: pubKeyBytes });
    injectKmsMock(mock);

    const signer = new KmsSigner();
    const logger = new AuditLogger(signer, 'kms');

    const trace = {
      input: { tx: 'abc123' },
      state: {},
      events: ['invoke'],
      timestamp: '2026-02-24T00:00:00.000Z',
    };

    const log = await logger.generateLog(trace as any);

    expect(log.signer.provider).toBe('kms');
    expect(log.signature).toBe(sigBytes.toString('hex'));
    expect(log.publicKey).toMatch(/-----BEGIN PUBLIC KEY-----/);
    expect(log.hash).toMatch(/^[0-9a-f]{64}$/);
    expect(log.algorithm).toBe('Ed25519+SHA256');
  });
});
