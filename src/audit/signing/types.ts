export type Signature = Buffer;
export type PublicKey = string; // PEM (SPKI)

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
}
