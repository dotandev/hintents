// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Cryptographic primitives and host function verification for the simulator.
//! Implements Soroban Protocol 22 `ecdsa_secp256r1_verify` (P-256) for WebAuthn / Passkeys (Issue #1981).

#![allow(dead_code)]

use p256::ecdsa::signature::hazmat::PrehashVerifier;
use p256::ecdsa::{Signature, VerifyingKey};
use thiserror::Error;

#[derive(Debug, Error, PartialEq, Eq)]
pub enum CryptoError {
    #[error("invalid public key length: {len} bytes (expected 33, 64, or 65 bytes)")]
    InvalidPublicKeyLength { len: usize },
    #[error("invalid signature length: {len} bytes (expected 64 bytes IEEE P1363 or DER encoded)")]
    InvalidSignatureLength { len: usize },
    #[error("invalid message hash length: {len} bytes (expected 32 bytes)")]
    InvalidMessageHashLength { len: usize },
    #[error("malformed public key: {0}")]
    MalformedPublicKey(String),
    #[error("malformed signature: {0}")]
    MalformedSignature(String),
}

/// Verify an ECDSA signature over the secp256r1 (NIST P-256) elliptic curve.
///
/// Implements Soroban Protocol 22 host function `ecdsa_secp256r1_verify`.
///
/// Parameters:
/// - `public_key`: 33-byte compressed SEC1, 65-byte uncompressed SEC1, or 64-byte raw (X || Y) coordinates
/// - `msg_hash`: 32-byte pre-hashed digest (SHA-256)
/// - `signature`: 64-byte IEEE P1363 (R || S) format or ASN.1 DER format
///
/// Returns:
/// - `Ok(true)` if the signature is mathematically valid for the public key and message digest
/// - `Ok(false)` if the signature format is valid but does not match the public key and digest
/// - `Err(CryptoError)` if any parameter has an invalid length or malformed encoding
pub fn ecdsa_secp256r1_verify(
    public_key: &[u8],
    msg_hash: &[u8],
    signature: &[u8],
) -> Result<bool, CryptoError> {
    // 1. Validate message hash length (SHA-256 pre-hash must be exactly 32 bytes)
    if msg_hash.len() != 32 {
        return Err(CryptoError::InvalidMessageHashLength {
            len: msg_hash.len(),
        });
    }

    // 2. Parse VerifyingKey from public key bytes
    let verifying_key = parse_secp256r1_public_key(public_key)?;

    // 3. Parse Signature
    let sig = parse_secp256r1_signature(signature)?;

    // 4. Verify pre-hashed digest against the signature
    match verifying_key.verify_prehash(msg_hash, &sig) {
        Ok(()) => Ok(true),
        Err(_) => Ok(false),
    }
}

/// Helper to parse public key bytes supporting compressed (33 bytes),
/// uncompressed (65 bytes), and raw 64-byte coordinate pairs.
fn parse_secp256r1_public_key(bytes: &[u8]) -> Result<VerifyingKey, CryptoError> {
    match bytes.len() {
        33 | 65 => VerifyingKey::from_sec1_bytes(bytes)
            .map_err(|e| CryptoError::MalformedPublicKey(e.to_string())),
        64 => {
            // Raw (X || Y) coordinates — prepend 0x04 uncompressed point indicator
            let mut sec1 = Vec::with_capacity(65);
            sec1.push(0x04);
            sec1.extend_from_slice(bytes);
            VerifyingKey::from_sec1_bytes(&sec1)
                .map_err(|e| CryptoError::MalformedPublicKey(e.to_string()))
        }
        len => Err(CryptoError::InvalidPublicKeyLength { len }),
    }
}

/// Helper to parse ECDSA signature bytes supporting 64-byte raw (R || S) and ASN.1 DER.
fn parse_secp256r1_signature(bytes: &[u8]) -> Result<Signature, CryptoError> {
    if bytes.len() == 64 {
        Signature::from_slice(bytes).map_err(|e| CryptoError::MalformedSignature(e.to_string()))
    } else {
        // Try parsing as ASN.1 DER if length is within DER range (usually 70-72 bytes)
        Signature::from_der(bytes).map_err(|e| CryptoError::MalformedSignature(e.to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use p256::ecdsa::signature::hazmat::PrehashSigner;
    use p256::ecdsa::SigningKey;
    use rand::rngs::OsRng;

    #[test]
    fn test_valid_secp256r1_signature_verification() {
        // Generate a random P-256 key pair
        let signing_key = SigningKey::random(&mut OsRng);
        let verifying_key = VerifyingKey::from(&signing_key);

        let msg_hash = [0x42u8; 32];
        let (signature, _) = signing_key.sign_prehash(&msg_hash).unwrap();
        let sig_bytes = signature.to_bytes();

        // 1. Verify with uncompressed 65-byte public key
        let pub_sec1 = verifying_key.to_encoded_point(false);
        assert_eq!(
            ecdsa_secp256r1_verify(pub_sec1.as_bytes(), &msg_hash, &sig_bytes),
            Ok(true)
        );

        // 2. Verify with compressed 33-byte public key
        let pub_compressed = verifying_key.to_encoded_point(true);
        assert_eq!(
            ecdsa_secp256r1_verify(pub_compressed.as_bytes(), &msg_hash, &sig_bytes),
            Ok(true)
        );

        // 3. Verify with raw 64-byte coordinate public key
        let raw_pub = &pub_sec1.as_bytes()[1..]; // Strip 0x04 prefix
        assert_eq!(raw_pub.len(), 64);
        assert_eq!(
            ecdsa_secp256r1_verify(raw_pub, &msg_hash, &sig_bytes),
            Ok(true)
        );
    }

    #[test]
    fn test_mismatched_signature_returns_false() {
        let signing_key = SigningKey::random(&mut OsRng);
        let verifying_key = VerifyingKey::from(&signing_key);

        let msg_hash = [0x42u8; 32];
        let different_hash = [0x99u8; 32];
        let (signature, _) = signing_key.sign_prehash(&msg_hash).unwrap();

        let pub_bytes = verifying_key.to_encoded_point(true);
        // Signature was computed for msg_hash, verifying against different_hash must return false
        assert_eq!(
            ecdsa_secp256r1_verify(pub_bytes.as_bytes(), &different_hash, &signature.to_bytes()),
            Ok(false)
        );
    }

    #[test]
    fn test_invalid_lengths_return_errors() {
        let valid_pub = [0x02; 33];
        let valid_hash = [0x00; 32];
        let valid_sig = [0x01; 64];

        // Bad hash length
        assert_eq!(
            ecdsa_secp256r1_verify(&valid_pub, &[0u8; 31], &valid_sig),
            Err(CryptoError::InvalidMessageHashLength { len: 31 })
        );

        // Bad public key length
        assert_eq!(
            ecdsa_secp256r1_verify(&[0u8; 20], &valid_hash, &valid_sig),
            Err(CryptoError::InvalidPublicKeyLength { len: 20 })
        );

        // Bad signature length/encoding
        assert!(matches!(
            ecdsa_secp256r1_verify(&valid_pub, &valid_hash, &[0u8; 50]),
            Err(CryptoError::MalformedSignature(_))
        ));
    }
}
