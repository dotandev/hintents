// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

export type Signature = Buffer;
export type PublicKey = string; // PEM (SPKI)

/**
 * A single certificate in the hardware attestation chain.
 * Ordered from leaf (signing key attestation) to root (manufacturer CA).
 */
export interface AttestationCertificate {
  /** PEM-encoded X.509 certificate */
  pem: string;
  /** Human-readable subject (e.g. "CN=YubiKey PIV Attestation") */
  subject: string;
  /** Human-readable issuer */
  issuer: string;
  /** Serial number (hex) */
  serial: string;
}

/**
 * Full attestation chain from an HSM or hardware token.
 */
export interface HardwareAttestation {
  /** Ordered certificate chain: [leaf, ..., root] */
  certificates: AttestationCertificate[];
  /** Token manufacturer or model (e.g. "YubiKey 5", "SoftHSM") */
  token_info: string;
  /** Whether the private key is marked as non-exportable on the token */
  key_non_exportable: boolean;
  /** ISO 8601 timestamp of when the attestation was retrieved */
  retrieved_at: string;
}

export interface AuditSigner {
  /**
   * Signs an arbitrary payload.
   * Implementations should throw an Error with a clear message on failure.
   */
  sign(payload: Uint8Array): Promise<Signature>;

  /**
   * Returns the public key corresponding to the signing key.
   * For Ed25519 this should be SPKI PEM.
   */
  public_key(): Promise<PublicKey>;

  /**
   * Returns the hardware attestation chain if the signer is backed by an HSM.
   * Software signers return undefined.
   */
  attestation_chain?(): Promise<HardwareAttestation | undefined>;
}
