// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Hardware Security Module (HSM) integration for cryptographic operations.
//!
//! This module provides a generic Signer interface that can be implemented
//! by different cryptographic backends, including software-based signers
//! and PKCS#11 HSM signers.

pub mod pkcs11;
pub mod software;

use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use std::fmt;
use thiserror::Error;

/// Generic signer interface for cryptographic operations
#[async_trait]
pub trait Signer: Send + Sync {
    /// Sign the provided data and return a signature
    async fn sign(&self, data: &[u8]) -> Result<Signature, SignerError>;

    /// Get the public key corresponding to the signing key
    async fn public_key(&self) -> Result<PublicKey, SignerError>;

    /// Get information about the signer implementation
    fn signer_info(&self) -> SignerInfo;
}

/// Public key representation
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PublicKey {
    /// Key algorithm (e.g., "ed25519", "secp256k1")
    pub algorithm: String,
    /// Public key bytes in SPKI format
    pub spki_bytes: Vec<u8>,
}

/// Digital signature
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Signature {
    /// Signature algorithm
    pub algorithm: String,
    /// Signature bytes
    pub bytes: Vec<u8>,
}

/// Information about a signer implementation
#[derive(Debug, Clone)]
pub struct SignerInfo {
    /// Signer type (e.g., "software", "pkcs11")
    pub signer_type: String,
    /// Algorithm used by the signer
    pub algorithm: String,
    /// Additional metadata about the signer
    pub metadata: std::collections::HashMap<String, String>,
}

/// Errors that can occur during signing operations
#[derive(Debug, Error)]
pub enum SignerError {
    #[error("PKCS#11 error: {0}")]
    Pkcs11(String),

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Cryptographic error: {0}")]
    Crypto(String),

    #[error("Configuration error: {0}")]
    Config(String),

    #[error("Key not found: {0}")]
    KeyNotFound(String),

    #[error("Invalid signature format: {0}")]
    InvalidSignature(String),

    #[error("Hardware error: {0}")]
    Hardware(String),
}

impl fmt::Display for PublicKey {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}:{}", self.algorithm, hex::encode(&self.spki_bytes))
    }
}

impl fmt::Display for Signature {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}:{}", self.algorithm, hex::encode(&self.bytes))
    }
}

/// Factory for creating signers based on configuration
pub struct SignerFactory;

impl SignerFactory {
    /// Create a signer based on configuration
    pub async fn create_from_config(config: &SignerConfig) -> Result<Box<dyn Signer>, SignerError> {
        match config.signer_type.as_str() {
            "software" => {
                let software_signer = software::SoftwareSigner::from_config(config)?;
                Ok(Box::new(software_signer))
            }
            "pkcs11" => {
                let pkcs11_signer = pkcs11::Pkcs11Signer::from_config(config).await?;
                Ok(Box::new(pkcs11_signer))
            }
            other => Err(SignerError::Config(format!(
                "Unsupported signer type: {}",
                other
            ))),
        }
    }

    /// Create a signer from environment variables
    pub async fn create_from_env() -> Result<Box<dyn Signer>, SignerError> {
        let config = SignerConfig::from_env()?;
        Self::create_from_config(&config).await
    }
}

/// Configuration for signer creation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SignerConfig {
    /// Type of signer to create ("software", "pkcs11")
    pub signer_type: String,

    /// Algorithm to use ("ed25519", "secp256k1")
    pub algorithm: String,

    /// Software signer configuration
    pub software: Option<SoftwareSignerConfig>,

    /// PKCS#11 signer configuration
    pub pkcs11: Option<Pkcs11SignerConfig>,
}

impl SignerConfig {
    /// Create configuration from environment variables
    pub fn from_env() -> Result<Self, SignerError> {
        let signer_type = std::env::var("ERST_SIGNER_TYPE")
            .unwrap_or_else(|_| "software".to_string());

        let algorithm = std::env::var("ERST_SIGNER_ALGORITHM")
            .unwrap_or_else(|_| "ed25519".to_string());

        let mut config = SignerConfig {
            signer_type,
            algorithm,
            software: None,
            pkcs11: None,
        };

        match config.signer_type.as_str() {
            "software" => {
                config.software = Some(SoftwareSignerConfig::from_env()?);
            }
            "pkcs11" => {
                config.pkcs11 = Some(Pkcs11SignerConfig::from_env()?);
            }
            _ => {}
        }

        Ok(config)
    }
}

/// Configuration for software signer
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SoftwareSignerConfig {
    /// Path to private key file (PEM format)
    pub private_key_path: Option<String>,
    /// Private key in PEM format (direct string)
    pub private_key_pem: Option<String>,
}

impl SoftwareSignerConfig {
    /// Create configuration from environment variables
    pub fn from_env() -> Result<Self, SignerError> {
        Ok(Self {
            private_key_path: std::env::var("ERST_SOFTWARE_PRIVATE_KEY_PATH").ok(),
            private_key_pem: std::env::var("ERST_SOFTWARE_PRIVATE_KEY_PEM").ok(),
        })
    }
}

/// Configuration for PKCS#11 signer
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Pkcs11SignerConfig {
    /// Path to PKCS#11 module/library
    pub module_path: String,

    /// PIN for the token
    pub pin: String,

    /// Token label (optional)
    pub token_label: Option<String>,

    /// Slot index (optional)
    pub slot_index: Option<u32>,

    /// Key label (optional)
    pub key_label: Option<String>,

    /// Key ID in hex (optional)
    pub key_id_hex: Option<String>,

    /// PIV slot for YubiKey (optional)
    pub piv_slot: Option<String>,

    /// Public key in PEM format (optional, can be derived from HSM)
    pub public_key_pem: Option<String>,
}

impl Pkcs11SignerConfig {
    /// Create configuration from environment variables
    pub fn from_env() -> Result<Self, SignerError> {
        let module_path = std::env::var("ERST_PKCS11_MODULE")
            .map_err(|_| SignerError::Config("ERST_PKCS11_MODULE must be set".to_string()))?;

        let pin = std::env::var("ERST_PKCS11_PIN")
            .map_err(|_| SignerError::Config("ERST_PKCS11_PIN must be set".to_string()))?;

        Ok(Self {
            module_path,
            pin,
            token_label: std::env::var("ERST_PKCS11_TOKEN_LABEL").ok(),
            slot_index: std::env::var("ERST_PKCS11_SLOT")
                .ok()
                .and_then(|s| s.parse().ok()),
            key_label: std::env::var("ERST_PKCS11_KEY_LABEL").ok(),
            key_id_hex: std::env::var("ERST_PKCS11_KEY_ID").ok(),
            piv_slot: std::env::var("ERST_PKCS11_PIV_SLOT").ok(),
            public_key_pem: std::env::var("ERST_PKCS11_PUBLIC_KEY_PEM").ok(),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_public_key_display() {
        let pubkey = PublicKey {
            algorithm: "ed25519".to_string(),
            spki_bytes: vec![0x01, 0x02, 0x03],
        };
        assert_eq!(pubkey.to_string(), "ed25519:010203");
    }

    #[test]
    fn test_signature_display() {
        let sig = Signature {
            algorithm: "ed25519".to_string(),
            bytes: vec![0x04, 0x05, 0x06],
        };
        assert_eq!(sig.to_string(), "ed25519:040506");
    }

    #[test]
    fn test_signer_config_from_env_default() {
        // Temporarily unset environment variables
        let _type = std::env::var("ERST_SIGNER_TYPE");
        let _algo = std::env::var("ERST_SIGNER_ALGORITHM");
        
        std::env::remove_var("ERST_SIGNER_TYPE");
        std::env::remove_var("ERST_SIGNER_ALGORITHM");

        let config = SignerConfig::from_env().unwrap();
        assert_eq!(config.signer_type, "software");
        assert_eq!(config.algorithm, "ed25519");

        // Restore environment variables if they existed
        if let Ok(type_val) = _type {
            std::env::set_var("ERST_SIGNER_TYPE", type_val);
        }
        if let Ok(algo_val) = _algo {
            std::env::set_var("ERST_SIGNER_ALGORITHM", algo_val);
        }
    }
}
