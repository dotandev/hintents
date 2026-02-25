// Copyright 2025 Hintents Users
// SPDX-License-Identifier: Apache-2.0

//! Hardware Security Module (HSM) integration for cryptographic operations.
//!
//! This module provides a generic Signer interface that supports multiple backends:
//! - Software-based signing (for development/testing)
//! - PKCS#11 HSM integration (for production use)
//! - Future support for cloud KMS services
//!
//! # Features
//!
//! - **Generic Signer Interface**: Abstract signing operations behind a common trait
//! - **PKCS#11 Support**: Dynamic loading of HSM libraries with standard PKCS#11 interface
//! - **Multiple Algorithms**: Support for Ed25519 and secp256k1 signatures
//! - **Hardware Attestation**: Optional attestation chain retrieval from HSMs
//! - **Error Handling**: Comprehensive error types for different failure modes
//!
//! # Usage
//!
//! ```rust
//! use simulator::hsm::{Signer, Pkcs11Signer, SoftwareSigner};
//!
//! // Create a software signer for testing
//! let signer = SoftwareSigner::new(private_key_bytes)?;
//!
//! // Create a PKCS#11 HSM signer for production
//! let signer = Pkcs11Signer::builder()
//!     .module_path("/usr/lib/softhsm/libsofthsm2.so")
//!     .pin("1234")
//!     .key_label("my-key")
//!     .build()?;
//!
//! // Sign data
//! let data = b"Hello, HSM!";
//! let signature = signer.sign(data).await?;
//!
//! // Verify signature
//! let public_key = signer.public_key().await?;
//! let is_valid = signer.verify(data, &signature, &public_key).await?;
//! ```

use std::fmt;
use std::path::PathBuf;
use std::sync::Arc;
use thiserror::Error;

pub mod software;
pub mod pkcs11;

pub use software::SoftwareSigner;
pub use pkcs11::Pkcs11Signer;

/// Errors that can occur during HSM operations
#[derive(Debug, Error)]
pub enum HsmError {
    #[error("PKCS#11 error: {0}")]
    Pkcs11(String),

    #[error("Library loading error: {0}")]
    LibraryLoad(String),

    #[error("Key not found: {0}")]
    KeyNotFound(String),

    #[error("Invalid configuration: {0}")]
    InvalidConfig(String),

    #[error("Signing operation failed: {0}")]
    SigningFailed(String),

    #[error("Public key retrieval failed: {0}")]
    PublicKeyFailed(String),

    #[error("Attestation not supported")]
    AttestationNotSupported,

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("UTF-8 error: {0}")]
    Utf8(#[from] std::string::FromUtf8Error),
}

/// Result type for HSM operations
pub type HsmResult<T> = Result<T, HsmError>;

/// Signature algorithm types
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SignatureAlgorithm {
    Ed25519,
    Secp256k1,
}

impl fmt::Display for SignatureAlgorithm {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SignatureAlgorithm::Ed25519 => write!(f, "ed25519"),
            SignatureAlgorithm::Secp256k1 => write!(f, "secp256k1"),
        }
    }
}

/// Public key representation
#[derive(Debug, Clone)]
pub struct PublicKey {
    /// Raw public key bytes
    pub bytes: Vec<u8>,
    /// Algorithm used
    pub algorithm: SignatureAlgorithm,
    /// PEM-encoded SPKI format (optional)
    pub pem: Option<String>,
}

impl PublicKey {
    /// Create a new public key
    pub fn new(bytes: Vec<u8>, algorithm: SignatureAlgorithm) -> Self {
        Self {
            bytes,
            algorithm,
            pem: None,
        }
    }

    /// Create a public key with PEM encoding
    pub fn with_pem(bytes: Vec<u8>, algorithm: SignatureAlgorithm, pem: String) -> Self {
        Self {
            bytes,
            algorithm,
            pem: Some(pem),
        }
    }
}

/// Digital signature
#[derive(Debug, Clone)]
pub struct Signature {
    /// Signature bytes
    pub bytes: Vec<u8>,
    /// Algorithm used
    pub algorithm: SignatureAlgorithm,
}

impl Signature {
    /// Create a new signature
    pub fn new(bytes: Vec<u8>, algorithm: SignatureAlgorithm) -> Self {
        Self { bytes, algorithm }
    }
}

/// Hardware attestation certificate
#[derive(Debug, Clone)]
pub struct AttestationCertificate {
    /// PEM-encoded X.509 certificate
    pub pem: String,
    /// Certificate subject
    pub subject: String,
    /// Certificate issuer
    pub issuer: String,
    /// Serial number (hex)
    pub serial: String,
}

/// Hardware attestation chain
#[derive(Debug, Clone)]
pub struct HardwareAttestation {
    /// Certificate chain from leaf to root
    pub certificates: Vec<AttestationCertificate>,
    /// Token information
    pub token_info: String,
    /// Whether key is non-exportable
    pub key_non_exportable: bool,
    /// Retrieval timestamp
    pub retrieved_at: chrono::DateTime<chrono::Utc>,
}

/// Generic signer interface for cryptographic operations
///
/// This trait abstracts different signing backends behind a common interface,
/// allowing applications to switch between software signing and HSM signing
/// without changing the application code.
#[async_trait::async_trait]
pub trait Signer: Send + Sync {
    /// Sign the provided data
    async fn sign(&self, data: &[u8]) -> HsmResult<Signature>;

    /// Get the public key for verification
    async fn public_key(&self) -> HsmResult<PublicKey>;

    /// Verify a signature against the public key
    async fn verify(&self, data: &[u8], signature: &Signature, public_key: &PublicKey) -> HsmResult<bool>;

    /// Get hardware attestation if available (optional)
    async fn attestation(&self) -> HsmResult<Option<HardwareAttestation>> {
        Ok(None)
    }

    /// Get the algorithm used by this signer
    fn algorithm(&self) -> SignatureAlgorithm;
}

/// Signer factory for creating different signer types
#[derive(Debug, Clone)]
pub struct SignerFactory;

impl SignerFactory {
    /// Create a software signer for testing/development
    pub fn create_software_signer(private_key: &[u8], algorithm: SignatureAlgorithm) -> HsmResult<Arc<dyn Signer>> {
        let signer = SoftwareSigner::new(private_key, algorithm)?;
        Ok(Arc::new(signer))
    }

    /// Create a PKCS#11 HSM signer
    pub async fn create_pkcs11_signer(
        library_path: &str,
        slot_id: Option<u32>,
        pin: &str,
        algorithm: SignatureAlgorithm,
        key_label: Option<&str>,
    ) -> Result<Arc<dyn Signer>, HsmError> {
        let signer = Pkcs11Signer::new(library_path, slot_id, pin, algorithm, key_label).await?;
        Ok(Arc::new(signer))
    }

    /// Create a PKCS#11 HSM signer with the first available key
    pub async fn create_pkcs11_signer_with_first_key(
        library_path: &str,
        slot_id: Option<u32>,
        pin: &str,
        algorithm: SignatureAlgorithm,
    ) -> Result<Arc<dyn Signer>, HsmError> {
        let signer = Pkcs11Signer::with_first_key(library_path, slot_id, pin, algorithm).await?;
        Ok(Arc::new(signer))
    }

    /// Create signer from environment configuration
    ///
    /// This method reads configuration from environment variables:
    /// - `HSM_PROVIDER`: "software", "pkcs11", or "kms" (future)
    /// - `HSM_PRIVATE_KEY`: Path to private key file (software provider)
    /// - `PKCS11_MODULE`: Path to PKCS#11 library
    /// - `PKCS11_PIN`: HSM PIN
    /// - `PKCS11_SLOT`: Slot ID (optional)
    /// - `PKCS11_KEY_LABEL`: Key label (optional)
    /// - `HSM_ALGORITHM`: "ed25519" or "secp256k1"
    pub async fn create_from_env() -> HsmResult<Arc<dyn Signer>> {
        let provider = std::env::var("HSM_PROVIDER")
            .unwrap_or_else(|_| "software".to_string())
            .to_lowercase();

        let algorithm_str = std::env::var("HSM_ALGORITHM")
            .unwrap_or_else(|_| "ed25519".to_string())
            .to_lowercase();
        let algorithm = match algorithm_str.as_str() {
            "ed25519" => SignatureAlgorithm::Ed25519,
            "secp256k1" => SignatureAlgorithm::Secp256k1,
            _ => return Err(HsmError::Configuration(
                "Unsupported algorithm. Use 'ed25519' or 'secp256k1'".to_string(),
            )),
        };

        match provider.as_str() {
            "software" => {
                let private_key_path = std::env::var("HSM_PRIVATE_KEY")
                    .map_err(|_| HsmError::Configuration(
                        "HSM_PRIVATE_KEY environment variable required for software provider".to_string(),
                    ))?;
                
                let private_key = std::fs::read(&private_key_path)
                    .map_err(|e| HsmError::Configuration(
                        format!("Failed to read private key file '{}': {}", private_key_path, e)
                    ))?;
                
                Self::create_software_signer(&private_key, algorithm)
            }
            "pkcs11" => {
                let library_path = std::env::var("PKCS11_MODULE")
                    .map_err(|_| HsmError::Configuration(
                        "PKCS11_MODULE environment variable required for PKCS#11 provider".to_string(),
                    ))?;
                
                let pin = std::env::var("PKCS11_PIN").unwrap_or_default();
                
                let slot_id = std::env::var("PKCS11_SLOT")
                    .ok()
                    .and_then(|s| s.parse().ok());
                
                let key_label = std::env::var("PKCS11_KEY_LABEL")
                    .ok()
                    .map(|s| s.leak()); // Convert to &'static str for simplicity
                
                if let Some(key_label) = key_label {
                    Self::create_pkcs11_signer(&library_path, slot_id, &pin, algorithm, Some(key_label)).await
                } else {
                    Self::create_pkcs11_signer_with_first_key(&library_path, slot_id, &pin, algorithm).await
                }
            }
            "kms" => Err(HsmError::NotImplemented("KMS provider not yet implemented".to_string())),
            _ => Err(HsmError::Configuration(
                format!("Unsupported HSM provider: '{}'. Use 'software', 'pkcs11', or 'kms'", provider)
            )),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Keypair, Signer as EdSigner, Verifier};
    use signature::RandomizedSigner;

    #[test]
    fn test_software_signer_ed25519() {
        let mut csprng = rand::rngs::OsRng;
        let keypair = Keypair::generate(&mut csprng);
        
        let signer = SoftwareSigner::new(keypair.to_bytes().as_slice(), SignatureAlgorithm::Ed25519).unwrap();
        
        let data = b"test message";
        let signature = tokio::runtime::Runtime::new().unwrap().block_on(
            signer.sign(data)
        ).unwrap();
        
        assert_eq!(signature.algorithm, SignatureAlgorithm::Ed25519);
        assert!(!signature.bytes.is_empty());
        
        // Verify with the original public key
        let public_key = tokio::runtime::Runtime::new().unwrap().block_on(
            signer.public_key()
        ).unwrap();
        
        let is_valid = tokio::runtime::Runtime::new().unwrap().block_on(
            signer.verify(data, &signature, &public_key)
        ).unwrap();
        
        assert!(is_valid);
    }

    #[test]
    fn test_public_key_creation() {
        let bytes = vec![1, 2, 3, 4];
        let public_key = PublicKey::new(bytes.clone(), SignatureAlgorithm::Ed25519);
        
        assert_eq!(public_key.bytes, bytes);
        assert_eq!(public_key.algorithm, SignatureAlgorithm::Ed25519);
        assert!(public_key.pem.is_none());
    }

    #[test]
    fn test_signature_creation() {
        let bytes = vec![5, 6, 7, 8];
        let signature = Signature::new(bytes.clone(), SignatureAlgorithm::Secp256k1);
        
        assert_eq!(signature.bytes, bytes);
        assert_eq!(signature.algorithm, SignatureAlgorithm::Secp256k1);
    }
}
