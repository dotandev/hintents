// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Software-based signer implementation for testing and development.
//!
//! This module provides a software-based implementation of the Signer trait
//! using in-memory cryptographic operations. It's primarily intended for:
//! - Testing and development environments
//! - CI/CD pipelines
//! - Situations where HSM hardware is not available

//! # Security Note
//!
//! This implementation keeps private keys in memory and should only be used
//! in non-production environments or for testing purposes.

use super::{HsmError, PublicKey, Signature, SignatureAlgorithm, Signer};
use ed25519_dalek::{Keypair, Signer as EdSigner, Verifier, Verifier as EdVerifier};
use p256::ecdsa::{SigningKey, VerifyingKey};
use p256::ecdsa::signature::{Signer as EcSigner, Verifier as EcVerifier};
use std::convert::TryInto;
use p256::pkcs8::{EncodePrivateKey, DecodePrivateKey};
use sec1::EncodePublicKey;
use signature::RandomizedSigner;
use std::sync::Arc;

/// Software-based signer implementation
pub struct SoftwareSigner {
    algorithm: SignatureAlgorithm,
    ed25519_keypair: Option<ed25519_dalek::Keypair>,
    secp256k1_signing_key: Option<p256::ecdsa::SigningKey>,
    public_key: Option<PublicKey>,
}

impl SoftwareSigner {
    /// Create a new software signer from raw private key bytes
    ///
    /// # Arguments
    ///
    /// * `private_key` - Raw private key bytes (32 bytes for Ed25519, PKCS#8 for secp256k1)
    /// * `algorithm` - Signature algorithm to use
    ///
    /// # Returns
    ///
    /// Returns a new SoftwareSigner instance
    ///
    /// # Errors
    ///
    /// Returns an error if the private key is invalid or malformed
    pub fn new(private_key: &[u8], algorithm: SignatureAlgorithm) -> HsmResult<Self> {
        match algorithm {
            SignatureAlgorithm::Ed25519 => {
                if private_key.len() != 32 {
                    return Err(HsmError::InvalidConfig(
                        "Ed25519 private key must be exactly 32 bytes".to_string()
                    ));
                }
                
                let key_bytes: [u8; 32] = private_key.try_into()
                    .map_err(|_| HsmError::InvalidConfig("Invalid Ed25519 private key length".to_string()))?;
                
                let keypair = Keypair::from_bytes(&key_bytes)
                    .map_err(|e| HsmError::InvalidConfig(format!("Invalid Ed25519 private key: {}", e)))?;
                
                let public_key = PublicKey::new(
                    keypair.public.to_bytes().to_vec(),
                    SignatureAlgorithm::Ed25519
                );
                
                Ok(Self {
                    algorithm,
                    ed25519_keypair: Some(keypair),
                    secp256k1_signing_key: None,
                    public_key: Some(public_key),
                })
            }
            
            SignatureAlgorithm::Secp256k1 => {
                let signing_key = SigningKey::from_pkcs8_der(private_key)
                    .map_err(|e| HsmError::InvalidConfig(format!("Invalid secp256k1 private key: {}", e)))?;
                
                let verifying_key = signing_key.verifying_key();
                let public_key_bytes = verifying_key.to_sec1_bytes()
                    .map_err(|e| HsmError::PublicKeyFailed(format!("Failed to encode public key: {}", e)))?;
                
                let public_key = PublicKey::new(
                    public_key_bytes.to_vec(),
                    SignatureAlgorithm::Secp256k1
                );
                
                Ok(Self {
                    algorithm,
                    ed25519_keypair: None,
                    secp256k1_signing_key: Some(signing_key),
                    public_key: Some(public_key),
                })
            }
        }
    }

    /// Create a new software signer and generate a fresh keypair
    ///
    /// # Arguments
    ///
    /// * `algorithm` - Signature algorithm to use
    ///
    /// # Returns
    ///
    /// Returns a new SoftwareSigner with a freshly generated keypair
    pub fn generate(algorithm: SignatureAlgorithm) -> HsmResult<Self> {
        match algorithm {
            SignatureAlgorithm::Ed25519 => {
                let mut csprng = rand::rngs::OsRng;
                let keypair = Keypair::generate(&mut csprng);
                
                let public_key = PublicKey::new(
                    keypair.public.to_bytes().to_vec(),
                    SignatureAlgorithm::Ed25519
                );
                
                Ok(Self {
                    algorithm,
                    ed25519_keypair: Some(keypair),
                    secp256k1_signing_key: None,
                    public_key: Some(public_key),
                })
            }
            
            SignatureAlgorithm::Secp256k1 => {
                let signing_key = SigningKey::random(&mut rand::rngs::OsRng);
                let verifying_key = signing_key.verifying_key();
                
                let public_key_bytes = verifying_key.to_sec1_bytes()
                    .map_err(|e| HsmError::PublicKeyFailed(format!("Failed to encode public key: {}", e)))?;
                
                let public_key = PublicKey::new(
                    public_key_bytes.to_vec(),
                    SignatureAlgorithm::Secp256k1
                );
                
                Ok(Self {
                    algorithm,
                    ed25519_keypair: None,
                    secp256k1_signing_key: Some(signing_key),
                    public_key: Some(public_key),
                })
            }
        }
    }

    /// Export the private key in PKCS#8 format (secp256k1 only)
    ///
    /// # Returns
    ///
    /// Returns the private key in PKCS#8 DER format
    ///
    /// # Errors
    ///
    /// Returns an error if the algorithm is not secp256k1 or encoding fails
    pub fn export_private_key(&self) -> HsmResult<Vec<u8>> {
        match self.algorithm {
            SignatureAlgorithm::Ed25519 => {
                let keypair = self.ed25519_keypair.as_ref()
                    .ok_or_else(|| HsmError::SigningFailed("Ed25519 keypair not initialized".to_string()))?;
                Ok(keypair.to_bytes().to_vec())
            }
            
            SignatureAlgorithm::Secp256k1 => {
                let signing_key = self.secp256k1_signing_key.as_ref()
                    .ok_or_else(|| HsmError::SigningFailed("secp256k1 signing key not initialized".to_string()))?;
                
                let pkcs8_der = signing_key.to_pkcs8_der()
                    .map_err(|e| HsmError::SigningFailed(format!("Failed to encode private key: {}", e)))?;
                
                Ok(pkcs8_der.as_bytes().to_vec())
            }
        }
    }

    /// Get the private key bytes (Ed25519 only)
    ///
    /// # Returns
    ///
    /// Returns the 32-byte private key for Ed25519
    ///
    /// # Errors
    ///
    /// Returns an error if the algorithm is not Ed25519
    pub fn private_key_bytes(&self) -> HsmResult<[u8; 32]> {
        match self.algorithm {
            SignatureAlgorithm::Ed25519 => {
                let keypair = self.ed25519_keypair.as_ref()
                    .ok_or_else(|| HsmError::SigningFailed("Ed25519 keypair not initialized".to_string()))?;
                Ok(keypair.to_bytes())
            }
            
            SignatureAlgorithm::Secp256k1 => {
                Err(HsmError::InvalidConfig(
                    "Private key bytes not available for secp256k1. Use export_private_key() instead.".to_string()
                ))
            }
        }
    }
}

#[async_trait::async_trait]
impl Signer for SoftwareSigner {
    async fn sign(&self, data: &[u8]) -> HsmResult<Signature> {
        match self.algorithm {
            SignatureAlgorithm::Ed25519 => {
                let keypair = self.ed25519_keypair.as_ref()
                    .ok_or_else(|| HsmError::SigningFailed("Ed25519 keypair not initialized".to_string()))?;
                
                let mut csprng = rand::rngs::OsRng;
                let signature = keypair.try_sign(&mut csprng, data)
                    .map_err(|e| HsmError::SigningFailed(format!("Ed25519 signing failed: {}", e)))?;
                
                Ok(Signature::new(signature.to_bytes().to_vec(), SignatureAlgorithm::Ed25519))
            }
            
            SignatureAlgorithm::Secp256k1 => {
                let signing_key = self.secp256k1_signing_key.as_ref()
                    .ok_or_else(|| HsmError::SigningFailed("secp256k1 signing key not initialized".to_string()))?;
                
                // For secp256k1, we need to hash the data first
                let hash = sha2::Sha256::digest(data);
                let signature = signing_key.try_sign(&hash)
                    .map_err(|e| HsmError::SigningFailed(format!("secp256k1 signing failed: {}", e)))?;
                
                // Convert signature to DER format
                let der_bytes = signature.to_der()
                    .map_err(|e| HsmError::SigningFailed(format!("Failed to encode signature: {}", e)))?;
                
                Ok(Signature::new(der_bytes.to_vec(), SignatureAlgorithm::Secp256k1))
            }
        }
    }

    async fn public_key(&self) -> HsmResult<PublicKey> {
        self.public_key.clone()
            .ok_or_else(|| HsmError::PublicKeyFailed("Public key not initialized".to_string()))
    }

    async fn verify(&self, data: &[u8], signature: &Signature, public_key: &PublicKey) -> HsmResult<bool> {
        if signature.algorithm != self.algorithm || public_key.algorithm != self.algorithm {
            return Ok(false);
        }

        match self.algorithm {
            SignatureAlgorithm::Ed25519 => {
                let public_key_bytes: [u8; 32] = public_key.bytes.try_into()
                    .map_err(|_| HsmError::PublicKeyFailed("Invalid Ed25519 public key length".to_string()))?;
                
                let public_key = ed25519_dalek::PublicKey::from_bytes(&public_key_bytes)
                    .map_err(|e| HsmError::PublicKeyFailed(format!("Invalid Ed25519 public key: {}", e)))?;
                
                let signature_bytes: [u8; 64] = signature.bytes.try_into()
                    .map_err(|_| HsmError::SigningFailed("Invalid Ed25519 signature length".to_string()))?;
                
                let signature = ed25519_dalek::Signature::from_bytes(&signature_bytes)
                    .map_err(|e| HsmError::SigningFailed(format!("Invalid Ed25519 signature: {}", e)))?;
                
                Ok(public_key.verify(data, &signature).is_ok())
            }
            
            SignatureAlgorithm::Secp256k1 => {
                let verifying_key = p256::ecdsa::VerifyingKey::from_sec1_bytes(&public_key.bytes)
                    .map_err(|e| HsmError::PublicKeyFailed(format!("Invalid secp256k1 public key: {}", e)))?;
                
                let signature = p256::ecdsa::Signature::from_der(&signature.bytes)
                    .map_err(|e| HsmError::SigningFailed(format!("Invalid secp256k1 signature: {}", e)))?;
                
                // Hash the data first
                let hash = sha2::Sha256::digest(data);
                
                Ok(verifying_key.verify(&hash, &signature).is_ok())
            }
        }
    }

    fn algorithm(&self) -> SignatureAlgorithm {
        self.algorithm
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Keypair, Signer as EdSigner, Verifier as EdVerifier};
    use signature::RandomizedSigner;

    #[tokio::test]
    async fn test_software_signer_ed25519_sign_verify() {
        let mut csprng = rand::rngs::OsRng;
        let keypair = Keypair::generate(&mut csprng);
        
        let signer = SoftwareSigner::new(keypair.to_bytes().as_slice(), SignatureAlgorithm::Ed25519).unwrap();
        
        let data = b"test message for signing";
        let signature = signer.sign(data).await.unwrap();
        
        assert_eq!(signature.algorithm, SignatureAlgorithm::Ed25519);
        assert_eq!(signature.bytes.len(), 64); // Ed25519 signatures are 64 bytes
        
        let public_key = signer.public_key().await.unwrap();
        let is_valid = signer.verify(data, &signature, &public_key).await.unwrap();
        
        assert!(is_valid);
        
        // Test with wrong data
        let wrong_data = b"wrong message";
        let is_invalid = signer.verify(wrong_data, &signature, &public_key).await.unwrap();
        assert!(!is_invalid);
    }

    #[tokio::test]
    async fn test_software_signer_secp256k1_sign_verify() {
        let signing_key = p256::ecdsa::SigningKey::random(&mut rand::rngs::OsRng);
        let pkcs8_der = signing_key.to_pkcs8_der();
        
        let signer = SoftwareSigner::new(pkcs8_der.as_bytes(), SignatureAlgorithm::Secp256k1).unwrap();
        
        let data = b"test message for secp256k1";
        let signature = signer.sign(data).await.unwrap();
        
        assert_eq!(signature.algorithm, SignatureAlgorithm::Secp256k1);
        // secp256k1 DER signatures vary in length but are typically 70-72 bytes
        assert!(signature.bytes.len() >= 70);
        
        let public_key = signer.public_key().await.unwrap();
        let is_valid = signer.verify(data, &signature, &public_key).await.unwrap();
        
        assert!(is_valid);
    }

    #[tokio::test]
    async fn test_software_signer_generate_ed25519() {
        let signer = SoftwareSigner::generate(SignatureAlgorithm::Ed25519).unwrap();
        
        let data = b"generated key test";
        let signature = signer.sign(data).await.unwrap();
        let public_key = signer.public_key().await.unwrap();
        
        let is_valid = signer.verify(data, &signature, &public_key).await.unwrap();
        assert!(is_valid);
    }

    #[tokio::test]
    async fn test_software_signer_generate_secp256k1() {
        let signer = SoftwareSigner::generate(SignatureAlgorithm::Secp256k1).unwrap();
        
        let data = b"generated secp256k1 test";
        let signature = signer.sign(data).await.unwrap();
        let public_key = signer.public_key().await.unwrap();
        
        let is_valid = signer.verify(data, &signature, &public_key).await.unwrap();
        assert!(is_valid);
    }

    #[test]
    fn test_invalid_ed25519_key_length() {
        let invalid_key = vec![1, 2, 3]; // Too short
        let result = SoftwareSigner::new(&invalid_key, SignatureAlgorithm::Ed25519);
        assert!(result.is_err());
        assert!(matches!(result.unwrap_err(), HsmError::InvalidConfig(_)));
    }

    #[test]
    fn test_export_private_key_ed25519() {
        let mut csprng = rand::rngs::OsRng;
        let keypair = Keypair::generate(&mut csprng);
        
        let signer = SoftwareSigner::new(keypair.to_bytes().as_slice(), SignatureAlgorithm::Ed25519).unwrap();
        let exported = signer.export_private_key().unwrap();
        
        assert_eq!(exported.len(), 32);
        assert_eq!(exported, keypair.to_bytes());
    }

    #[test]
    fn test_private_key_bytes_ed25519() {
        let mut csprng = rand::rngs::OsRng;
        let keypair = Keypair::generate(&mut csprng);
        
        let signer = SoftwareSigner::new(keypair.to_bytes().as_slice(), SignatureAlgorithm::Ed25519).unwrap();
        let key_bytes = signer.private_key_bytes().unwrap();
        
        assert_eq!(key_bytes, keypair.to_bytes());
    }
}
