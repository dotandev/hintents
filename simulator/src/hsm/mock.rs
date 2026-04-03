// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Mock HSM implementation for local testing and simulation.
//!
//! This module provides a mock HSM that simulates hardware security module
//! behavior including configurable latency and failure rates.

use super::{MockHsmConfig, PublicKey, Signature, Signer, SignerError, SignerInfo};
use async_trait::async_trait;
use ed25519_dalek::{Signer as EdSigner, SigningKey, VerifyingKey};
use rand::Rng;
use std::collections::HashMap;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

/// Mock HSM signer for testing and simulation.
pub struct MockHsm {
    signing_key: SigningKey,
    config: MockHsmConfig,
    sign_call_count: AtomicU64,
}

impl MockHsm {
    /// Create a new MockHSM with the given configuration.
    pub fn new(config: MockHsmConfig) -> Result<Self, SignerError> {
        let signing_key = if let Some(ref seed_hex) = config.seed_hex {
            let seed_bytes = hex::decode(seed_hex)
                .map_err(|e| SignerError::Config(format!("Invalid seed hex: {}", e)))?;
            if seed_bytes.len() != 32 {
                return Err(SignerError::Config(
                    "Seed must be exactly 32 bytes".to_string(),
                ));
            }
            let seed: [u8; 32] = seed_bytes
                .try_into()
                .map_err(|_| SignerError::Config("Invalid seed length".to_string()))?;
            SigningKey::from_bytes(&seed)
        } else {
            let mut csprng = rand::rngs::OsRng;
            SigningKey::generate(&mut csprng)
        };

        Ok(Self {
            signing_key,
            config,
            sign_call_count: AtomicU64::new(0),
        })
    }

    /// Create a MockHSM from configuration with defaults.
    pub fn from_config(config: &MockHsmConfig) -> Result<Self, SignerError> {
        Self::new(config.clone())
    }

    /// Get the verifying key.
    pub fn verifying_key(&self) -> VerifyingKey {
        self.signing_key.verifying_key()
    }

    /// Get the number of sign calls made.
    pub fn sign_call_count(&self) -> u64 {
        self.sign_call_count.load(Ordering::SeqCst)
    }

    /// Simulate latency based on configuration.
    async fn simulate_latency(&self) {
        if self.config.latency_ms > 0 {
            tokio::time::sleep(Duration::from_millis(self.config.latency_ms)).await;
        }
    }

    /// Check if this call should fail based on failure rate.
    fn should_fail(&self) -> bool {
        if self.config.failure_rate <= 0.0 {
            return false;
        }
        if self.config.failure_rate >= 1.0 {
            return true;
        }
        let mut rng = rand::thread_rng();
        rng.gen::<f64>() < self.config.failure_rate
    }
}

#[async_trait]
impl Signer for MockHsm {
    async fn sign(&self, data: &[u8]) -> Result<Signature, SignerError> {
        self.sign_call_count.fetch_add(1, Ordering::SeqCst);

        self.simulate_latency().await;

        if self.should_fail() {
            return Err(SignerError::Hardware("Simulated HSM failure".to_string()));
        }

        let signature = self.signing_key.sign(data);

        Ok(Signature {
            algorithm: "ed25519".to_string(),
            bytes: signature.to_bytes().to_vec(),
        })
    }

    async fn public_key(&self) -> Result<PublicKey, SignerError> {
        self.simulate_latency().await;

        if self.should_fail() {
            return Err(SignerError::Hardware("Simulated HSM failure".to_string()));
        }

        let verifying_key = self.signing_key.verifying_key();
        let spki_bytes = verifying_key.to_bytes().to_vec();

        Ok(PublicKey {
            algorithm: "ed25519".to_string(),
            spki_bytes,
        })
    }

    fn signer_info(&self) -> SignerInfo {
        let mut metadata = HashMap::new();
        metadata.insert("implementation".to_string(), "mock".to_string());
        metadata.insert("latency_ms".to_string(), self.config.latency_ms.to_string());
        metadata.insert(
            "failure_rate".to_string(),
            self.config.failure_rate.to_string(),
        );
        metadata.insert(
            "sign_call_count".to_string(),
            self.sign_call_count().to_string(),
        );

        SignerInfo {
            signer_type: "mock".to_string(),
            algorithm: "ed25519".to_string(),
            metadata,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn default_config() -> MockHsmConfig {
        MockHsmConfig {
            latency_ms: 0,
            failure_rate: 0.0,
            seed_hex: None,
        }
    }

    #[tokio::test]
    async fn test_mock_hsm_sign() {
        let config = default_config();
        let hsm = MockHsm::new(config).unwrap();

        let data = b"Hello, world!";
        let signature = hsm.sign(data).await.unwrap();

        assert_eq!(signature.algorithm, "ed25519");
        assert_eq!(signature.bytes.len(), 64);
    }

    #[tokio::test]
    async fn test_mock_hsm_public_key() {
        let config = default_config();
        let hsm = MockHsm::new(config).unwrap();

        let public_key = hsm.public_key().await.unwrap();

        assert_eq!(public_key.algorithm, "ed25519");
        assert!(!public_key.spki_bytes.is_empty());
    }

    #[tokio::test]
    async fn test_mock_hsm_deterministic_with_seed() {
        let config = MockHsmConfig {
            latency_ms: 0,
            failure_rate: 0.0,
            seed_hex: Some(
                "0000000000000000000000000000000000000000000000000000000000000001".to_string(),
            ),
        };

        let hsm1 = MockHsm::new(config.clone()).unwrap();
        let hsm2 = MockHsm::new(config).unwrap();

        let data = b"Test data";
        let sig1 = hsm1.sign(data).await.unwrap();
        let sig2 = hsm2.sign(data).await.unwrap();

        assert_eq!(sig1.bytes, sig2.bytes);
    }

    #[tokio::test]
    async fn test_mock_hsm_failure_rate_always_fail() {
        let config = MockHsmConfig {
            latency_ms: 0,
            failure_rate: 1.0,
            seed_hex: None,
        };
        let hsm = MockHsm::new(config).unwrap();

        let result = hsm.sign(b"test").await;
        assert!(result.is_err());

        if let Err(SignerError::Hardware(msg)) = result {
            assert!(msg.contains("Simulated HSM failure"));
        } else {
            panic!("Expected Hardware error");
        }
    }

    #[tokio::test]
    async fn test_mock_hsm_failure_rate_never_fail() {
        let config = MockHsmConfig {
            latency_ms: 0,
            failure_rate: 0.0,
            seed_hex: None,
        };
        let hsm = MockHsm::new(config).unwrap();

        for _ in 0..10 {
            let result = hsm.sign(b"test").await;
            assert!(result.is_ok());
        }
    }

    #[tokio::test]
    async fn test_mock_hsm_sign_call_count() {
        let config = default_config();
        let hsm = MockHsm::new(config).unwrap();

        assert_eq!(hsm.sign_call_count(), 0);

        hsm.sign(b"test1").await.unwrap();
        assert_eq!(hsm.sign_call_count(), 1);

        hsm.sign(b"test2").await.unwrap();
        assert_eq!(hsm.sign_call_count(), 2);
    }

    #[tokio::test]
    async fn test_mock_hsm_signer_info() {
        let config = MockHsmConfig {
            latency_ms: 100,
            failure_rate: 0.5,
            seed_hex: None,
        };
        let hsm = MockHsm::new(config).unwrap();

        let info = hsm.signer_info();

        assert_eq!(info.signer_type, "mock");
        assert_eq!(info.algorithm, "ed25519");
        assert_eq!(info.metadata.get("latency_ms"), Some(&"100".to_string()));
        assert_eq!(info.metadata.get("failure_rate"), Some(&"0.5".to_string()));
    }

    #[test]
    fn test_mock_hsm_invalid_seed() {
        let config = MockHsmConfig {
            latency_ms: 0,
            failure_rate: 0.0,
            seed_hex: Some("invalid".to_string()),
        };

        let result = MockHsm::new(config);
        assert!(result.is_err());
    }

    #[test]
    fn test_mock_hsm_wrong_seed_length() {
        let config = MockHsmConfig {
            latency_ms: 0,
            failure_rate: 0.0,
            seed_hex: Some("00".to_string()),
        };

        let result = MockHsm::new(config);
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_mock_hsm_signature_verification() {
        use ed25519_dalek::Verifier;

        let config = default_config();
        let hsm = MockHsm::new(config).unwrap();

        let data = b"Test message";
        let signature = hsm.sign(data).await.unwrap();

        let verifying_key = hsm.verifying_key();
        let sig_bytes: [u8; 64] = signature.bytes.as_slice().try_into().unwrap();
        let sig = ed25519_dalek::Signature::from_bytes(&sig_bytes);

        assert!(verifying_key.verify(data, &sig).is_ok());
    }
}
