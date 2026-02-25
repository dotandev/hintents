import type { AuditSigner, PublicKey, Signature } from './types';
import { HsmRateLimiter } from './rateLimiter';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const lazyRequire = (name: string): any => {
  // Using eval('require') keeps this file compatible with CommonJS builds and avoids TS/webpack rewriting.
  // eslint-disable-next-line no-eval
  return eval('require')(name);
};

/**
 * PKCS#11-backed signer.
 *
 * Configuration is done via environment variables.
 *
 * Required env (typical):
 * - ERST_PKCS11_MODULE  : path to PKCS#11 module (e.g. /usr/lib/softhsm/libsofthsm2.so)
 * - ERST_PKCS11_TOKEN_LABEL or ERST_PKCS11_SLOT
 * - ERST_PKCS11_PIN
 * - ERST_PKCS11_KEY_LABEL or ERST_PKCS11_KEY_ID
 *
 * Notes:
 * - This implementation uses the optional dependency `pkcs11js`.
 * - We intentionally do not hardcode any secrets.
 */
export class Pkcs11Ed25519Signer implements AuditSigner {
  private readonly cfg = {
    module: process.env.ERST_PKCS11_MODULE,
    tokenLabel: process.env.ERST_PKCS11_TOKEN_LABEL,
    slot: process.env.ERST_PKCS11_SLOT,
    pin: process.env.ERST_PKCS11_PIN,
    keyLabel: process.env.ERST_PKCS11_KEY_LABEL,
    keyIdHex: process.env.ERST_PKCS11_KEY_ID,
    publicKeyPem: process.env.ERST_PKCS11_PUBLIC_KEY_PEM,
  };

  private pkcs11: any | undefined;

  constructor() {
    // Lazy-load so users/tests without PKCS#11 don't break unless provider is selected.
    try {
      this.pkcs11 = lazyRequire('pkcs11js');
    } catch {
      throw new Error(
        'pkcs11 provider selected but optional dependency `pkcs11js` is not installed (add it to dependencies to enable PKCS#11)'
      );
    }

    if (!this.cfg.module) {
      throw new Error('pkcs11 provider selected but ERST_PKCS11_MODULE is not set');
    }
    if (!this.cfg.pin) {
      throw new Error('pkcs11 provider selected but ERST_PKCS11_PIN is not set');
    }
    if (!this.cfg.keyLabel && !this.cfg.keyIdHex) {
      throw new Error('pkcs11 provider selected but neither ERST_PKCS11_KEY_LABEL nor ERST_PKCS11_KEY_ID is set');
    }
  }

  async public_key(): Promise<PublicKey> {
    // We allow providing the public key via env/config to avoid a brittle, token-specific extraction flow.
    // If not provided, we attempt to read it from the token.
    if (this.cfg.publicKeyPem) return this.cfg.publicKeyPem;

    const msg =
      'pkcs11 public key retrieval is not configured. Set ERST_PKCS11_PUBLIC_KEY_PEM to a SPKI PEM public key.';
    throw new Error(msg);
  }

  async sign(payload: Uint8Array): Promise<Signature> {
    // Guard against HSM abuse during loops
    await HsmRateLimiter.checkAndRecordCall();

    // Minimal skeleton that surfaces errors clearly.
    // Implementing full PKCS#11 key discovery + Ed25519 mechanisms depends on token capabilities.
    // We keep this as a real provider module but require ERST_PKCS11_PUBLIC_KEY_PEM for verification.

    const pkcs11 = this.pkcs11;
    if (!pkcs11) throw new Error('pkcs11 internal error: module not loaded');

    // This is a deliberately conservative implementation: open session, login, locate key, sign.
    // If your token does not support Ed25519 sign via CKM_EDDSA, it will error with a clear message.

    const lib = new pkcs11.PKCS11();
    try {
      lib.load(this.cfg.module);
      lib.C_Initialize();

      const slots = lib.C_GetSlotList(true);
      if (!slots || slots.length === 0) throw new Error('no PKCS#11 slots with tokens found');

      // Choose slot
      const slot = this.cfg.slot ? slots[Number(this.cfg.slot)] : slots[0];
      if (slot === undefined) throw new Error('configured ERST_PKCS11_SLOT did not resolve to a valid slot');

      const session = lib.C_OpenSession(slot, pkcs11.CKF_SERIAL_SESSION | pkcs11.CKF_RW_SESSION);
      try {
        lib.C_Login(session, 1 /* CKU_USER */, this.cfg.pin);

        // Locate private key by label or id
        const template: any[] = [{ type: pkcs11.CKA_CLASS, value: pkcs11.CKO_PRIVATE_KEY }];
        if (this.cfg.keyLabel) template.push({ type: pkcs11.CKA_LABEL, value: this.cfg.keyLabel });
        if (this.cfg.keyIdHex) template.push({ type: pkcs11.CKA_ID, value: Buffer.from(this.cfg.keyIdHex, 'hex') });

        lib.C_FindObjectsInit(session, template);
        const keys = lib.C_FindObjects(session, 1);
        lib.C_FindObjectsFinal(session);

        const key = keys?.[0];
        if (!key) throw new Error('private key not found on token (check ERST_PKCS11_KEY_LABEL / ERST_PKCS11_KEY_ID)');

        // Attempt EdDSA sign (CKM_EDDSA). Some tokens use different mechanisms.
        const mechanism = { mechanism: (pkcs11 as any).CKM_EDDSA ?? 0x00001050 };

        try {
          lib.C_SignInit(session, mechanism, key);
          const sig = lib.C_Sign(session, Buffer.from(payload));
          return Buffer.from(sig);
        } catch (e) {
          const msg = e instanceof Error ? e.message : String(e);
          throw new Error(`pkcs11 signing failed: ${msg}`);
        }
      } finally {
        try {
          lib.C_CloseSession(session);
        } catch {
          // ignore
        }
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`pkcs11 error: ${msg}`);
    } finally {
      try {
        lib.C_Finalize();
      } catch {
        // ignore
      }
    }
  }
}
