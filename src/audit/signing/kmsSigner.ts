// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import type { AuditSigner, PublicKey, Signature } from "./types";

/**
 * AWS KMS-backed signer for audit trail signing.
 *
 * Uses an asymmetric KMS key (ECC_NIST_P256 or RSA) to sign payloads natively
 * via the KMS Sign API, without routing through a PKCS#11 abstraction layer.
 *
 * Required environment variables:
 *   ERST_KMS_KEY_ID       - KMS key ID or ARN of the signing key
 *   AWS_REGION            - AWS region where the key resides
 *
 * Optional environment variables:
 *   ERST_KMS_SIGNING_ALGORITHM - KMS signing algorithm (default: ECDSA_SHA_256)
 *                                Supported values:
 *                                  RSASSA_PSS_SHA_256 | RSASSA_PSS_SHA_384 | RSASSA_PSS_SHA_512
 *                                  RSASSA_PKCS1_V1_5_SHA_256 | RSASSA_PKCS1_V1_5_SHA_384 | RSASSA_PKCS1_V1_5_SHA_512
 *                                  ECDSA_SHA_256 | ECDSA_SHA_384 | ECDSA_SHA_512
 *
 * AWS credentials are resolved via the standard credential provider chain
 * (environment variables, shared credentials file, EC2/ECS instance metadata, etc.).
 *
 * Note: KMS does not support Ed25519 keys for asymmetric signing as of 2026.
 * If your policy requires Ed25519, use the PKCS#11 or software signer instead.
 */
export class KmsSigner implements AuditSigner {
  private readonly keyId: string;
  private readonly signingAlgorithm: string;
  private readonly region: string;

  // Lazy-loaded KMS client. Loaded once on first use.
  private client: any | undefined;

  constructor(opts?: {
    keyId?: string;
    signingAlgorithm?: string;
    region?: string;
  }) {
    const keyId = opts?.keyId ?? process.env.ERST_KMS_KEY_ID;
    if (!keyId) {
      throw new Error("KMS signer: ERST_KMS_KEY_ID is required");
    }

    const region = opts?.region ?? process.env.AWS_REGION;
    if (!region) {
      throw new Error("KMS signer: AWS_REGION is required");
    }

    this.keyId = keyId;
    this.region = region;
    this.signingAlgorithm =
      opts?.signingAlgorithm ??
      process.env.ERST_KMS_SIGNING_ALGORITHM ??
      "ECDSA_SHA_256";
  }

  /**
   * Signs the payload using AWS KMS Sign API.
   * The payload should be the pre-computed digest bytes (e.g. SHA-256 hash).
   */
  async sign(payload: Uint8Array): Promise<Signature> {
    const { KMSClient, SignCommand } = this.loadKmsModule();

    const client = this.getClient(KMSClient);

    const command = new SignCommand({
      KeyId: this.keyId,
      Message: Buffer.from(payload),
      MessageType: "DIGEST",
      SigningAlgorithm: this.signingAlgorithm,
    });

    let response: any;
    try {
      response = await client.send(command);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`KMS signing failed: ${msg}`);
    }

    if (!response.Signature) {
      throw new Error(
        "KMS signing failed: response contained no Signature field",
      );
    }

    return Buffer.from(response.Signature);
  }

  /**
   * Returns the public key corresponding to the KMS signing key as DER-encoded
   * bytes, base64-encoded, wrapped in a SPKI PEM envelope.
   *
   * KMS returns the raw DER-encoded SubjectPublicKeyInfo (SPKI) bytes, which
   * is exactly what SPKI PEM encapsulates.
   */
  async public_key(): Promise<PublicKey> {
    const { KMSClient, GetPublicKeyCommand } = this.loadKmsModule();

    const client = this.getClient(KMSClient);

    const command = new GetPublicKeyCommand({ KeyId: this.keyId });

    let response: any;
    try {
      response = await client.send(command);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`KMS GetPublicKey failed: ${msg}`);
    }

    if (!response.PublicKey) {
      throw new Error(
        "KMS GetPublicKey: response contained no PublicKey field",
      );
    }

    const der = Buffer.from(response.PublicKey);
    const b64 = der
      .toString("base64")
      .replace(/(.{64})/g, "$1\n")
      .trimEnd();
    return `-----BEGIN PUBLIC KEY-----\n${b64}\n-----END PUBLIC KEY-----\n`;
  }

  // ---- private helpers ----

  private getClient(KMSClient: any): any {
    if (!this.client) {
      this.client = new KMSClient({ region: this.region });
    }
    return this.client;
  }

  /**
   * Lazily requires @aws-sdk/client-kms so that users without the optional
   * dependency do not see errors unless they actually select the kms provider.
   */
  private loadKmsModule(): {
    KMSClient: any;
    SignCommand: any;
    GetPublicKeyCommand: any;
  } {
    try {
      // eslint-disable-next-line no-eval
      const mod = eval("require")("@aws-sdk/client-kms");
      return {
        KMSClient: mod.KMSClient,
        SignCommand: mod.SignCommand,
        GetPublicKeyCommand: mod.GetPublicKeyCommand,
      };
    } catch {
      throw new Error(
        "kms provider selected but optional dependency `@aws-sdk/client-kms` is not installed. " +
          "Add it to your dependencies: npm install @aws-sdk/client-kms",
      );
    }
  }
}
// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

import type { AuditSigner, PublicKey, Signature } from "./types";

/**
 * AWS KMS-backed signer for audit trail signing.
 *
 * Uses an asymmetric KMS key (ECC_NIST_P256 or RSA) to sign payloads natively
 * via the KMS Sign API, without routing through a PKCS#11 abstraction layer.
 *
 * Required environment variables:
 *   ERST_KMS_KEY_ID       - KMS key ID or ARN of the signing key
 *   AWS_REGION            - AWS region where the key resides
 *
 * Optional environment variables:
 *   ERST_KMS_SIGNING_ALGORITHM - KMS signing algorithm (default: ECDSA_SHA_256)
 *                                Supported values:
 *                                  RSASSA_PSS_SHA_256 | RSASSA_PSS_SHA_384 | RSASSA_PSS_SHA_512
 *                                  RSASSA_PKCS1_V1_5_SHA_256 | RSASSA_PKCS1_V1_5_SHA_384 | RSASSA_PKCS1_V1_5_SHA_512
 *                                  ECDSA_SHA_256 | ECDSA_SHA_384 | ECDSA_SHA_512
 *
 * AWS credentials are resolved via the standard credential provider chain
 * (environment variables, shared credentials file, EC2/ECS instance metadata, etc.).
 *
 * Note: KMS does not support Ed25519 keys for asymmetric signing as of 2026.
 * If your policy requires Ed25519, use the PKCS#11 or software signer instead.
 */
export class KmsSigner implements AuditSigner {
  private readonly keyId: string;
  private readonly signingAlgorithm: string;
  private readonly region: string;

  // Lazy-loaded KMS client. Loaded once on first use.
  private client: any | undefined;

  constructor(opts?: {
    keyId?: string;
    signingAlgorithm?: string;
    region?: string;
  }) {
    const keyId = opts?.keyId ?? process.env.ERST_KMS_KEY_ID;
    if (!keyId) {
      throw new Error("KMS signer: ERST_KMS_KEY_ID is required");
    }

    const region = opts?.region ?? process.env.AWS_REGION;
    if (!region) {
      throw new Error("KMS signer: AWS_REGION is required");
    }

    this.keyId = keyId;
    this.region = region;
    this.signingAlgorithm =
      opts?.signingAlgorithm ??
      process.env.ERST_KMS_SIGNING_ALGORITHM ??
      "ECDSA_SHA_256";
  }

  /**
   * Signs the payload using AWS KMS Sign API.
   * The payload should be the pre-computed digest bytes (e.g. SHA-256 hash).
   */
  async sign(payload: Uint8Array): Promise<Signature> {
    const { KMSClient, SignCommand } = this.loadKmsModule();

    const client = this.getClient(KMSClient);

    const command = new SignCommand({
      KeyId: this.keyId,
      Message: Buffer.from(payload),
      MessageType: "DIGEST",
      SigningAlgorithm: this.signingAlgorithm,
    });

    let response: any;
    try {
      response = await client.send(command);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`KMS signing failed: ${msg}`);
    }

    if (!response.Signature) {
      throw new Error(
        "KMS signing failed: response contained no Signature field",
      );
    }

    return Buffer.from(response.Signature);
  }

  /**
   * Returns the public key corresponding to the KMS signing key as DER-encoded
   * bytes, base64-encoded, wrapped in a SPKI PEM envelope.
   *
   * KMS returns the raw DER-encoded SubjectPublicKeyInfo (SPKI) bytes, which
   * is exactly what SPKI PEM encapsulates.
   */
  async public_key(): Promise<PublicKey> {
    const { KMSClient, GetPublicKeyCommand } = this.loadKmsModule();

    const client = this.getClient(KMSClient);

    const command = new GetPublicKeyCommand({ KeyId: this.keyId });

    let response: any;
    try {
      response = await client.send(command);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`KMS GetPublicKey failed: ${msg}`);
    }

    if (!response.PublicKey) {
      throw new Error(
        "KMS GetPublicKey: response contained no PublicKey field",
      );
    }

    const der = Buffer.from(response.PublicKey);
    const b64 = der
      .toString("base64")
      .replace(/(.{64})/g, "$1\n")
      .trimEnd();
    return `-----BEGIN PUBLIC KEY-----\n${b64}\n-----END PUBLIC KEY-----\n`;
  }

  // ---- private helpers ----

  private getClient(KMSClient: any): any {
    if (!this.client) {
      this.client = new KMSClient({ region: this.region });
    }
    return this.client;
  }

  /**
   * Lazily requires @aws-sdk/client-kms so that users without the optional
   * dependency do not see errors unless they actually select the kms provider.
   */
  private loadKmsModule(): {
    KMSClient: any;
    SignCommand: any;
    GetPublicKeyCommand: any;
  } {
    try {
      // eslint-disable-next-line no-eval
      const mod = eval("require")("@aws-sdk/client-kms");
      return {
        KMSClient: mod.KMSClient,
        SignCommand: mod.SignCommand,
        GetPublicKeyCommand: mod.GetPublicKeyCommand,
      };
    } catch {
      throw new Error(
        "kms provider selected but optional dependency `@aws-sdk/client-kms` is not installed. " +
          "Add it to your dependencies: npm install @aws-sdk/client-kms",
      );
    }
  }
}
