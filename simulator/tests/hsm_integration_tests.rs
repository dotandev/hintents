// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Integration tests for HSM cryptographic operations.

use crate::hsm::{
    Signer, SignerFactory, SignerConfig, SignerError,
    SoftwareSignerConfig, Pkcs11SignerConfig,
    software::SoftwareSigner,
};
use std::env;
use std::fs;
use std::path::Path;
use tempfile::TempDir;

#[cfg(test)]
mod tests {
    use super::*;

    /// Test software signer with generated key
    #[tokio::test]
    async fn test_software_signer_generated_key() {
        let (signer, _pem_data) = SoftwareSigner::generate().unwrap();
        
        // Test signing
        let data = b"Hello, world!";
        let signature = signer.sign(data).await.unwrap();
        
        assert_eq!(signature.algorithm, "ed25519");
        assert_eq!(signature.bytes.len(), 64); // Ed25519 signature size
        
        // Test public key retrieval
        let public_key = signer.public_key().await.unwrap();
        assert_eq!(public_key.algorithm, "ed25519");
        assert!(!public_key.spki_bytes.is_empty());
        
        // Test signer info
        let info = signer.signer_info();
        assert_eq!(info.signer_type, "software");
        assert_eq!(info.algorithm, "ed25519");
        assert!(info.metadata.contains_key("key_type"));
        assert!(info.metadata.contains_key("implementation"));
    }

    /// Test software signer with PEM file
    #[tokio::test]
    async fn test_software_signer_pem_file() {
        let temp_dir = TempDir::new().unwrap();
        let key_file_path = temp_dir.path().join("test_key.pem");
        
        // Generate a key and save it
        let (_signer, pem_data) = SoftwareSigner::generate().unwrap();
        fs::write(&key_file_path, pem_data).unwrap();
        
        // Create signer from file
        let config = SoftwareSignerConfig {
            private_key_path: Some(key_file_path.to_string_lossy().to_string()),
            private_key_pem: None,
        };
        
        let signer = SoftwareSigner::from_config(&config).unwrap();
        
        // Test signing
        let data = b"Test from PEM file";
        let signature = signer.sign(data).await.unwrap();
        assert_eq!(signature.algorithm, "ed25519");
        
        let public_key = signer.public_key().await.unwrap();
        assert_eq!(public_key.algorithm, "ed25519");
    }

    /// Test software signer with direct PEM data
    #[tokio::test]
    async fn test_software_signer_pem_data() {
        // Generate a key
        let (_signer, pem_data) = SoftwareSigner::generate().unwrap();
        
        // Create signer from PEM data
        let config = SoftwareSignerConfig {
            private_key_path: None,
            private_key_pem: Some(pem_data),
        };
        
        let signer = SoftwareSigner::from_config(&config).unwrap();
        
        // Test signing
        let data = b"Test from PEM data";
        let signature = signer.sign(data).await.unwrap();
        assert_eq!(signature.algorithm, "ed25519");
        
        let public_key = signer.public_key().await.unwrap();
        assert_eq!(public_key.algorithm, "ed25519");
    }

    /// Test software signer configuration errors
    #[test]
    fn test_software_signer_config_errors() {
        // No key provided
        let config = SoftwareSignerConfig {
            private_key_path: None,
            private_key_pem: None,
        };
        
        assert!(SoftwareSigner::from_config(&config).is_err());
        
        // Invalid file path
        let config = SoftwareSignerConfig {
            private_key_path: Some("/nonexistent/path/key.pem".to_string()),
            private_key_pem: None,
        };
        
        assert!(SoftwareSigner::from_config(&config).is_err());
        
        // Invalid PEM data
        let config = SoftwareSignerConfig {
            private_key_path: None,
            private_key_pem: Some("invalid pem data".to_string()),
        };
        
        assert!(SoftwareSigner::from_config(&config).is_err());
    }

    /// Test signer factory with software configuration
    #[tokio::test]
    async fn test_signer_factory_software() {
        let config = SignerConfig {
            signer_type: "software".to_string(),
            algorithm: "ed25519".to_string(),
            software: Some(SoftwareSignerConfig {
                private_key_pem: None, // Will be generated
                private_key_path: None,
            }),
            pkcs11: None,
        };
        
        // This should fail because no key is provided
        assert!(SignerFactory::create_from_config(&config).await.is_err());
        
        // Test with generated key
        let temp_dir = TempDir::new().unwrap();
        let key_file_path = temp_dir.path().join("test_key.pem");
        let (_signer, pem_data) = SoftwareSigner::generate().unwrap();
        fs::write(&key_file_path, pem_data).unwrap();
        
        let config = SignerConfig {
            signer_type: "software".to_string(),
            algorithm: "ed25519".to_string(),
            software: Some(SoftwareSignerConfig {
                private_key_path: Some(key_file_path.to_string_lossy().to_string()),
                private_key_pem: None,
            }),
            pkcs11: None,
        };
        
        let signer = SignerFactory::create_from_config(&config).await.unwrap();
        
        // Test the created signer
        let data = b"Factory test";
        let signature = signer.sign(data).await.unwrap();
        assert_eq!(signature.algorithm, "ed25519");
        
        let public_key = signer.public_key().await.unwrap();
        assert_eq!(public_key.algorithm, "ed25519");
    }

    /// Test signer factory with unsupported type
    #[tokio::test]
    async fn test_signer_factory_unsupported_type() {
        let config = SignerConfig {
            signer_type: "unsupported".to_string(),
            algorithm: "ed25519".to_string(),
            software: None,
            pkcs11: None,
        };
        
        let result = SignerFactory::create_from_config(&config).await;
        assert!(result.is_err());
        
        match result.unwrap_err() {
            SignerError::Config(msg) => {
                assert!(msg.contains("Unsupported signer type"));
            }
            _ => panic!("Expected Config error"),
        }
    }

    /// Test signer configuration from environment
    #[test]
    fn test_signer_config_from_env() {
        // Test default values
        let _type = env::var("ERST_SIGNER_TYPE");
        let _algo = env::var("ERST_SIGNER_ALGORITHM");
        
        env::remove_var("ERST_SIGNER_TYPE");
        env::remove_var("ERST_SIGNER_ALGORITHM");
        
        let config = SignerConfig::from_env().unwrap();
        assert_eq!(config.signer_type, "software");
        assert_eq!(config.algorithm, "ed25519");
        
        // Test custom values
        env::set_var("ERST_SIGNER_TYPE", "pkcs11");
        env::set_var("ERST_SIGNER_ALGORITHM", "secp256k1");
        
        let config = SignerConfig::from_env().unwrap();
        assert_eq!(config.signer_type, "pkcs11");
        assert_eq!(config.algorithm, "secp256k1");
        
        // Restore environment
        if let Ok(type_val) = _type {
            env::set_var("ERST_SIGNER_TYPE", type_val);
        } else {
            env::remove_var("ERST_SIGNER_TYPE");
        }
        
        if let Ok(algo_val) = _algo {
            env::set_var("ERST_SIGNER_ALGORITHM", algo_val);
        } else {
            env::remove_var("ERST_SIGNER_ALGORITHM");
        }
    }

    /// Test PKCS#11 configuration from environment
    #[test]
    fn test_pkcs11_config_from_env() {
        // Set required environment variables
        env::set_var("ERST_PKCS11_MODULE", "/usr/lib/libykcs11.so");
        env::set_var("ERST_PKCS11_PIN", "123456");
        
        let config = Pkcs11SignerConfig::from_env().unwrap();
        assert_eq!(config.module_path, "/usr/lib/libykcs11.so");
        assert_eq!(config.pin, "123456");
        
        // Test optional variables
        env::set_var("ERST_PKCS11_TOKEN_LABEL", "YubiKey");
        env::set_var("ERST_PKCS11_SLOT", "0");
        env::set_var("ERST_PKCS11_KEY_LABEL", "TestKey");
        env::set_var("ERST_PKCS11_KEY_ID", "01");
        env::set_var("ERST_PKCS11_PIV_SLOT", "9a");
        env::set_var("ERST_PKCS11_PUBLIC_KEY_PEM", "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----");
        
        let config = Pkcs11SignerConfig::from_env().unwrap();
        assert_eq!(config.token_label, Some("YubiKey".to_string()));
        assert_eq!(config.slot_index, Some(0));
        assert_eq!(config.key_label, Some("TestKey".to_string()));
        assert_eq!(config.key_id_hex, Some("01".to_string()));
        assert_eq!(config.piv_slot, Some("9a".to_string()));
        assert!(config.public_key_pem.is_some());
        
        // Clean up
        env::remove_var("ERST_PKCS11_MODULE");
        env::remove_var("ERST_PKCS11_PIN");
        env::remove_var("ERST_PKCS11_TOKEN_LABEL");
        env::remove_var("ERST_PKCS11_SLOT");
        env::remove_var("ERST_PKCS11_KEY_LABEL");
        env::remove_var("ERST_PKCS11_KEY_ID");
        env::remove_var("ERST_PKCS11_PIV_SLOT");
        env::remove_var("ERST_PKCS11_PUBLIC_KEY_PEM");
    }

    /// Test PKCS#11 configuration missing required variables
    #[test]
    fn test_pkcs11_config_missing_required() {
        // Ensure required variables are not set
        env::remove_var("ERST_PKCS11_MODULE");
        env::remove_var("ERST_PKCS11_PIN");
        
        let result = Pkcs11SignerConfig::from_env();
        assert!(result.is_err());
        
        match result.unwrap_err() {
            SignerError::Config(msg) => {
                assert!(msg.contains("ERST_PKCS11_MODULE") || msg.contains("ERST_PKCS11_PIN"));
            }
            _ => panic!("Expected Config error"),
        }
    }

    /// Test signature verification
    #[tokio::test]
    async fn test_signature_verification() {
        let (signer, _pem) = SoftwareSigner::generate().unwrap();
        
        let data = b"Test message for verification";
        let signature = signer.sign(data).await.unwrap();
        let public_key = signer.public_key().await.unwrap();
        
        // Verify the signature using the public key
        use ed25519_dalek::{Verifier, VerifyingKey, Signature as EdSignature};
        
        // Extract the verifying key from SPKI (simplified - in practice you'd parse DER)
        let key_bytes = &public_key.spki_bytes[public_key.spki_bytes.len() - 32..];
        let verifying_key = VerifyingKey::from_bytes(key_bytes).unwrap();
        
        let ed_signature = EdSignature::from_bytes(&signature.bytes).unwrap();
        let result = verifying_key.verify(data, &ed_signature);
        
        assert!(result.is_ok(), "Signature verification failed");
        
        // Test with wrong data
        let wrong_data = b"Wrong message";
        let result = verifying_key.verify(wrong_data, &ed_signature);
        assert!(result.is_err(), "Signature should not verify with wrong data");
    }

    /// Test multiple signers with different keys
    #[tokio::test]
    async fn test_multiple_signers() {
        let (signer1, _pem1) = SoftwareSigner::generate().unwrap();
        let (signer2, _pem2) = SoftwareSigner::generate().unwrap();
        
        let data = b"Test message";
        
        let sig1 = signer1.sign(data).await.unwrap();
        let sig2 = signer2.sign(data).await.unwrap();
        
        // Signatures should be different (different keys)
        assert_ne!(sig1.bytes, sig2.bytes);
        
        // Public keys should be different
        let pub1 = signer1.public_key().await.unwrap();
        let pub2 = signer2.public_key().await.unwrap();
        assert_ne!(pub1.spki_bytes, pub2.spki_bytes);
        
        // Each signature should verify with its corresponding public key
        use ed25519_dalek::{Verifier, VerifyingKey, Signature as EdSignature};
        
        let key1_bytes = &pub1.spki_bytes[pub1.spki_bytes.len() - 32..];
        let key2_bytes = &pub2.spki_bytes[pub2.spki_bytes.len() - 32..];
        
        let verifying_key1 = VerifyingKey::from_bytes(key1_bytes).unwrap();
        let verifying_key2 = VerifyingKey::from_bytes(key2_bytes).unwrap();
        
        let ed_sig1 = EdSignature::from_bytes(&sig1.bytes).unwrap();
        let ed_sig2 = EdSignature::from_bytes(&sig2.bytes).unwrap();
        
        assert!(verifying_key1.verify(data, &ed_sig1).is_ok());
        assert!(verifying_key2.verify(data, &ed_sig2).is_ok());
        
        // Cross-verification should fail
        assert!(verifying_key1.verify(data, &ed_sig2).is_err());
        assert!(verifying_key2.verify(data, &ed_sig1).is_err());
    }

    /// Test concurrent signing operations
    #[tokio::test]
    async fn test_concurrent_signing() {
        let (signer, _pem) = SoftwareSigner::generate().unwrap();
        
        let data = b"Concurrent test message";
        
        // Spawn multiple signing tasks
        let mut handles = vec![];
        for i in 0..10 {
            let signer_clone = signer.clone();
            let data_clone = data.to_vec();
            let handle = tokio::spawn(async move {
                let signature = signer_clone.sign(&data_clone).await.unwrap();
                (i, signature)
            });
            handles.push(handle);
        }
        
        // Wait for all tasks to complete
        let mut signatures = vec![];
        for handle in handles {
            let (i, signature) = handle.await.unwrap();
            signatures.push((i, signature));
        }
        
        // All signatures should be identical (same key, same data)
        let first_sig = &signatures[0].1.bytes;
        for (_, signature) in &signatures {
            assert_eq!(signature.bytes, *first_sig);
        }
        
        // Verify the signature
        let public_key = signer.public_key().await.unwrap();
        use ed25519_dalek::{Verifier, VerifyingKey, Signature as EdSignature};
        
        let key_bytes = &public_key.spki_bytes[public_key.spki_bytes.len() - 32..];
        let verifying_key = VerifyingKey::from_bytes(key_bytes).unwrap();
        let ed_signature = EdSignature::from_bytes(first_sig).unwrap();
        
        assert!(verifying_key.verify(data, &ed_signature).is_ok());
    }
}
