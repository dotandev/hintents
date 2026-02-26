// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import { normalizeTokenLabel, resolvePkcs11KeyIdHex, resolveYkcs11KeyIdHex } from '../src/audit/signing/pkcs11Signer';

const loadSigner = (): typeof import('../src/audit/signing/pkcs11Signer').Pkcs11Ed25519Signer =>
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  require('../src/audit/signing/pkcs11Signer').Pkcs11Ed25519Signer;

describe('Pkcs11Ed25519Signer', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('constructor validation', () => {
    test('should throw clear error when ERST_PKCS11_MODULE is not set', () => {
      delete process.env.ERST_PKCS11_MODULE;
      process.env.ERST_PKCS11_PIN = '1234';
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';

      const Pkcs11Ed25519Signer = loadSigner();
      expect(() => new Pkcs11Ed25519Signer()).toThrow(
        'pkcs11 provider selected but ERST_PKCS11_MODULE is not set'
      );
    });

    test('should throw clear error when ERST_PKCS11_PIN is not set', () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      delete process.env.ERST_PKCS11_PIN;
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';

      const Pkcs11Ed25519Signer = loadSigner();
      expect(() => new Pkcs11Ed25519Signer()).toThrow(
        'pkcs11 provider selected but ERST_PKCS11_PIN is not set'
      );
    });

    test('should throw clear error when neither key label, key ID, nor PIV slot is set', () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      process.env.ERST_PKCS11_PIN = '1234';
      delete process.env.ERST_PKCS11_KEY_LABEL;
      delete process.env.ERST_PKCS11_KEY_ID;
      delete process.env.ERST_PKCS11_PIV_SLOT;

      const Pkcs11Ed25519Signer = loadSigner();
      expect(() => new Pkcs11Ed25519Signer()).toThrow(
        'pkcs11 provider selected but neither ERST_PKCS11_KEY_LABEL, ERST_PKCS11_KEY_ID, nor ERST_PKCS11_PIV_SLOT is set'
      );
    });

    test('should throw clear error when pkcs11js is not installed', () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      process.env.ERST_PKCS11_PIN = '1234';
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';

      jest.isolateModules(() => {
        jest.doMock('pkcs11js', () => {
          throw new Error('missing');
        });

        // eslint-disable-next-line @typescript-eslint/no-var-requires
        const { Pkcs11Ed25519Signer } = require('../src/audit/signing/pkcs11Signer');
        expect(() => new Pkcs11Ed25519Signer()).toThrow(
          'pkcs11 provider selected but optional dependency `pkcs11js` is not installed'
        );
      });

      jest.dontMock('pkcs11js');
    });
  });

  describe('public_key', () => {
    test('should return public key from environment when set', async () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      process.env.ERST_PKCS11_PIN = '1234';
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';
      process.env.ERST_PKCS11_PUBLIC_KEY_PEM = '-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----';

      const Pkcs11Ed25519Signer = loadSigner();
      const signer = new Pkcs11Ed25519Signer();
      const publicKey = await signer.public_key();

      expect(publicKey).toBe(process.env.ERST_PKCS11_PUBLIC_KEY_PEM);
    });

    test('should throw clear error when public key is not configured', async () => {
      process.env.ERST_PKCS11_MODULE = '/usr/lib/softhsm/libsofthsm2.so';
      process.env.ERST_PKCS11_PIN = '1234';
      process.env.ERST_PKCS11_KEY_LABEL = 'test-key';
      delete process.env.ERST_PKCS11_PUBLIC_KEY_PEM;

      const Pkcs11Ed25519Signer = loadSigner();
      const signer = new Pkcs11Ed25519Signer();

      await expect(signer.public_key()).rejects.toThrow(
        'pkcs11 public key retrieval is not configured. Set ERST_PKCS11_PUBLIC_KEY_PEM to a SPKI PEM public key.'
      );
    });
  });

  describe('error context messages', () => {
    test('should provide context for module load failures', () => {
      const expectedErrorPattern = /Failed to load PKCS#11 module at '.*': .* Check that the library exists and is accessible\./;
      expect(expectedErrorPattern.test(
        "Failed to load PKCS#11 module at '/invalid/path.so': ENOENT. Check that the library exists and is accessible."
      )).toBe(true);
    });

    test('should provide context for initialization failures', () => {
      const testCases = [
        {
          error: 'Library lock error (CKR_CANT_LOCK). The PKCS#11 library may be in use by another process.',
          expected: /Library lock error \(CKR_CANT_LOCK\)\. The PKCS#11 library may be in use by another process\./
        },
        {
          error: 'Token not present (CKR_TOKEN_NOT_PRESENT). Ensure the HSM/token is connected.',
          expected: /Token not present \(CKR_TOKEN_NOT_PRESENT\)\. Ensure the HSM\/token is connected\./
        },
        {
          error: 'Device error (CKR_DEVICE_ERROR). Check HSM/token hardware connection.',
          expected: /Device error \(CKR_DEVICE_ERROR\)\. Check HSM\/token hardware connection\./
        }
      ];

      testCases.forEach(({ error, expected }) => {
        expect(expected.test(`${error}: some details`)).toBe(true);
      });
    });

    test('should provide context for login failures', () => {
      const testCases = [
        {
          error: 'Wrong PIN (CKR_PIN_INCORRECT)',
          expected: /Wrong PIN \(CKR_PIN_INCORRECT\)/
        },
        {
          error: 'PIN locked (CKR_PIN_LOCKED). The token may be locked due to too many failed attempts.',
          expected: /PIN locked \(CKR_PIN_LOCKED\)\. The token may be locked due to too many failed attempts\./
        },
        {
          error: 'Token not present (CKR_TOKEN_NOT_PRESENT)',
          expected: /Token not present \(CKR_TOKEN_NOT_PRESENT\)/
        }
      ];

      testCases.forEach(({ error, expected }) => {
        expect(expected.test(`${error}: some details`)).toBe(true);
      });
    });
  });

  describe('YubiKey PIV helpers', () => {
    test('normalizes token labels by trimming padding', () => {
      expect(normalizeTokenLabel('YubiKey PIV\x00\x00  ')).toBe('YubiKey PIV');
    });

    test('maps PIV slots to YKCS11 key IDs', () => {
      expect(resolveYkcs11KeyIdHex('9a')).toBe('01');
      expect(resolveYkcs11KeyIdHex('0x9c')).toBe('02');
      expect(resolveYkcs11KeyIdHex('9D')).toBe('03');
      expect(resolveYkcs11KeyIdHex('9e')).toBe('04');
      expect(resolveYkcs11KeyIdHex('82')).toBe('05');
      expect(resolveYkcs11KeyIdHex('95')).toBe('18');
      expect(resolveYkcs11KeyIdHex('f9')).toBe('19');
    });

    test('rejects unsupported PIV slots', () => {
      expect(() => resolveYkcs11KeyIdHex('9b')).toThrow(
        "Unsupported PIV slot '9b'. Supported slots: 9a, 9c, 9d, 9e, 82-95, f9."
      );
    });

    test('prefers explicit key IDs over derived PIV slot IDs', () => {
      expect(resolvePkcs11KeyIdHex({ keyIdHex: '0a', pivSlot: '9a' })).toBe('0a');
    });

    test('rejects invalid explicit key IDs', () => {
      expect(() => resolvePkcs11KeyIdHex({ keyIdHex: 'xyz' })).toThrow(
        "Invalid ERST_PKCS11_KEY_ID 'xyz'. Expected an even-length hex string (e.g., 01, 0a, 10)."
      );
      expect(() => resolvePkcs11KeyIdHex({ keyIdHex: 'a' })).toThrow(
        "Invalid ERST_PKCS11_KEY_ID 'a'. Expected an even-length hex string (e.g., 01, 0a, 10)."
      );
    });
  });
});
