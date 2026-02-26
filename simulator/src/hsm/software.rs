// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Software-based signer implementation using local cryptographic keys.

use super::{PublicKey, Signature, Signer, SignerError, SignerInfo, SoftwareSignerConfig};
use async_trait::async_trait;
use ed25519_dalek::{Signer as EdSigner, SigningKey, VerifyingKey};
use ed25519_dalek::pkcs8::DecodePrivateKey;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::Path;

/// Software-based signer using local Ed25519 keys
pub struct SoftwareSigner {
    signing_key: SigningKey,
    algorithm: String,
}

impl SoftwareSigner {
    /// Create a new software signer from a private key file
    pub fn from_key_file<P: AsRef<Path>>(path: P) -> Result<Self, SignerError> {
        let pem_data = fs::read_to_string(path)
            .map_err(|e| SignerError::Io(e))?;
        
        Self::from_pem(&pem_data)
    }

    /// Create a new software signer from PEM data
    pub fn from_pem(pem_data: &str) -> Result<Self, SignerError> {
        let signing_key = SigningKey::from_pkcs8_pem(pem_data)
            .map_err(|e| SignerError::Crypto(format!("Failed to parse private key: {}", e)))?;

        Ok(Self {
            signing_key,
            algorithm: "ed25519".to_string(),
        })
    }

    /// Create a software signer from configuration
    pub fn from_config(config: &SoftwareSignerConfig) -> Result<Self, SignerError> {
        if let Some(pem_data) = &config.private_key_pem {
            Self::from_pem(pem_data)
        } else if let Some(path) = &config.private_key_path {
            Self::from_key_file(path)
        } else {
            Err(SignerError::Config(
                "Either private_key_pem or private_key_path must be provided".to_string()
            ))
        }
    }

    /// Generate a new random key pair and return the signer
    pub fn generate() -> Result<(Self, String), SignerError> {
        let mut csprng = rand::rngs::OsRng;
        let signing_key = SigningKey::generate(&mut csprng);
        
        let public_key = signing_key.verifying_key();
        let pem_data = signing_key.to_pkcs8_pem(Default::default())
            .map_err(|e| SignerError::Crypto(format!("Failed to serialize private key: {}", e)))?;

        let signer = Self {
            signing_key,
            algorithm: "ed25519".to_string(),
        };

        Ok((signer, pem_data))
    }

    /// Get the verifying key
    pub fn verifying_key(&self) -> &VerifyingKey {
        &self.signing_key.verifying_key()
    }
}

#[async_trait]
impl Signer for SoftwareSigner {
    async fn sign(&self, data: &[u8]) -> Result<Signature, SignerError> {
        let signature = self.signing_key.sign(data);
        
        Ok(Signature {
            algorithm: self.algorithm.clone(),
            bytes: signature.to_bytes().to_vec(),
        })
    }

    async fn public_key(&self) -> Result<PublicKey, SignerError> {
        let verifying_key = self.signing_key.verifying_key();
        let spki_bytes = verifying_key.to_public_key_der()
            .map_err(|e| SignerError::Crypto(format!("Failed to serialize public key: {}", e)))?;

        Ok(PublicKey {
            algorithm: self.algorithm.clone(),
            spki_bytes: spki_bytes.as_bytes().to_vec(),
        })
    }

    fn signer_info(&self) -> SignerInfo {
        let mut metadata = HashMap::new();
        metadata.insert("key_type".to_string(), "ed25519".to_string());
        metadata.insert("implementation".to_string(), "software".to_string());

        SignerInfo {
            signer_type: "software".to_string(),
            algorithm: self.algorithm.clone(),
            metadata,
        }
    }
}

/// Configuration for secp256k1 software signer
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Secp256k1SoftwareSignerConfig {
    /// Path to private key file (PEM format)
    pub private_key_path: Option<String>,
    /// Private key in PEM format (direct string)
    pub private_key_pem: Option<String>,
}

/// Software-based signer using local secp256k1 keys
pub struct Secp256k1SoftwareSigner {
    signing_key: k256::ecdsa::SigningKey,
    algorithm: String,
}

impl Secp256k1SoftwareSigner {
    /// Create a new secp256k1 software signer from a private key file
    pub fn from_key_file<P: AsRef<Path>>(path: P) -> Result<Self, SignerError> {
        let pem_data = fs::read_to_string(path)
            .map_err(|e| SignerError::Io(e))?;
        
        Self::from_pem(&pem_data)
    }

    /// Create a new secp256k1 software signer from PEM data
    pub fn from_pem(pem_data: &str) -> Result<Self, SignerError> {
        let signing_key = k256::ecdsa::SigningKey::from_pkcs8_pem(pem_data)
            .map_err(|e| SignerError::Crypto(format!("Failed to parse private key: {}", e)))?;

        Ok(Self {
            signing_key,
            algorithm: "secp256k1".to_string(),
        })
    }

    /// Create a secp256k1 software signer from configuration
    pub fn from_config(config: &Secp256k1SoftwareSignerConfig) -> Result<Self, SignerError> {
        if let Some(pem_data) = &config.private_key_pem {
            Self::from_pem(pem_data)
        } else if let Some(path) = &config.private_key_path {
            Self::from_key_file(path)
        } else {
            Err(SignerError::Config(
                "Either private_key_pem or private_key_path must be provided".to_string()
            ))
        }
    }

    /// Generate a new random key pair and return the signer
    pub fn generate() -> Result<(Self, String), SignerError> {
        let mut csprng = rand::rngs::OsRng;
        let signing_key = k256::ecdsa::SigningKey::random(&mut csprng);
        
        let pem_data = signing_key.to_pkcs8_pem(k256::pkcs8::LineEnding::LF)
            .map_err(|e| SignerError::Crypto(format!("Failed to serialize private key: {}", e)))?;

        let signer = Self {
            signing_key,
            algorithm: "secp256k1".to_string(),
        };

        Ok((signer, pem_data))
    }

    /// Get the verifying key
    pub fn verifying_key(&self) -> &k256::ecdsa::VerifyingKey {
        self.signing_key.verifying_key()
    }
}

#[async_trait]
impl Signer for Secp256k1SoftwareSigner {
    async fn sign(&self, data: &[u8]) -> Result<Signature, SignerError> {
        use k256::ecdsa::signature::Signer;
        
        let signature: k256::ecdsa::Signature = self.signing_key
            .sign_digest_recoverable(k256::ecdsa::digest::Digest::hash(data))
            .map_err(|e| SignerError::Crypto(format!("Failed to sign data: {}", e)))?;
        
        Ok(Signature {
            algorithm: self.algorithm.clone(),
            bytes: signature.to_bytes().to_vec(),
        })
    }

    async fn public_key(&self) -> Result<PublicKey, SignerError> {
        let verifying_key = self.signing_key.verifying_key();
        let spki_bytes = verifying_key.to_public_key_der()
            .map_err(|e| SignerError::Crypto(format!("Failed to serialize public key: {}", e)))?;

        Ok(PublicKey {
            algorithm: self.algorithm.clone(),
            spki_bytes: spki_bytes.as_bytes().to_vec(),
        })
    }

    fn signer_info(&self) -> SignerInfo {
        let mut metadata = HashMap::new();
        metadata.insert("key_type".to_string(), "secp256k1".to_string());
        metadata.insert("implementation".to_string(), "software".to_string());

        SignerInfo {
            signer_type: "software".to_string(),
            algorithm: self.algorithm.clone(),
            metadata,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_ed25519_software_signer() {
        let (signer, _pem) = SoftwareSigner::generate().unwrap();
        
        let data = b"Hello, world!";
        let signature = signer.sign(data).await.unwrap();
        
        assert_eq!(signature.algorithm, "ed25519");
        assert_eq!(signature.bytes.len(), 64); // Ed25519 signature size
        
        let public_key = signer.public_key().await.unwrap();
        assert_eq!(public_key.algorithm, "ed25519");
        assert!(!public_key.spki_bytes.is_empty());
        
        let info = signer.signer_info();
        assert_eq!(info.signer_type, "software");
        assert_eq!(info.algorithm, "ed25519");
    }

    #[tokio::test]
    async fn test_secp256k1_software_signer() {
        let (signer, _pem) = Secp256k1SoftwareSigner::generate().unwrap();
        
        let data = b"Hello, world!";
        let signature = signer.sign(data).await.unwrap();
        
        assert_eq!(signature.algorithm, "secp256k1");
        assert_eq!(signature.bytes.len(), 64); // secp256k1 signature size
        
        let public_key = signer.public_key().await.unwrap();
        assert_eq!(public_key.algorithm, "secp256k1");
        assert!(!public_key.spki_bytes.is_empty());
        
        let info = signer.signer_info();
        assert_eq!(info.signer_type, "software");
        assert_eq!(info.algorithm, "secp256k1");
    }

    #[tokio::test]
    async fn test_ed25519_signature_verification() {
        let (signer, _pem) = SoftwareSigner::generate().unwrap();
        
        let data = b"Test message";
        let signature = signer.sign(data).await.unwrap();
        let public_key = signer.public_key().await.unwrap();
        
        // Verify the signature
        let verifying_key = ed25519_dalek::VerifyingKey::from_bytes(
            &public_key.spki_bytes[public_key.spki_bytes.len() - 32..]
        ).unwrap();
        
        let sig_bytes = ed25519_dalek::Signature::from_bytes(&signature.bytes).unwrap();
        assert!(verifying_key.verify(data, &sig_bytes).is_ok());
    }

    #[test]
    fn test_software_signer_config() {
        let config = SoftwareSignerConfig {
            private_key_path: Some("/path/to/key.pem".to_string()),
            private_key_pem: None,
        };

        // This should fail since the file doesn't exist
        assert!(SoftwareSigner::from_config(&config).is_err());
    }
}
