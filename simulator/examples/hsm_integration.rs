// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Example demonstrating HSM integration for cryptographic operations.

use crate::hsm::{SignerFactory, SignerConfig, SignerError};
use std::env;

/// Example of using the HSM integration
pub async fn example_hsm_usage() -> Result<(), SignerError> {
    println!("=== HSM Integration Example ===\n");

    // Example 1: Create a software signer
    println!("1. Creating software signer...");
    let software_config = SignerConfig {
        signer_type: "software".to_string(),
        algorithm: "ed25519".to_string(),
        software: Some(crate::hsm::SoftwareSignerConfig {
            private_key_path: Some("test_key.pem".to_string()),
            private_key_pem: None,
        }),
        pkcs11: None,
    };

    match SignerFactory::create_from_config(&software_config).await {
        Ok(signer) => {
            println!("✓ Software signer created successfully");
            let info = signer.signer_info();
            println!("  Type: {}", info.signer_type);
            println!("  Algorithm: {}", info.algorithm);
            
            // Test signing
            let data = b"Hello, HSM world!";
            let signature = signer.sign(data).await?;
            println!("  Signature: {}", signature);
            
            let public_key = signer.public_key().await?;
            println!("  Public key: {}", public_key);
        }
        Err(e) => println!("✗ Failed to create software signer: {}", e),
    }

    println!();

    // Example 2: Create a PKCS#11 signer (if configured)
    println!("2. Creating PKCS#11 signer...");
    if env::var("ERST_PKCS11_MODULE").is_ok() {
        match SignerFactory::create_from_env().await {
            Ok(signer) => {
                println!("✓ PKCS#11 signer created successfully");
                let info = signer.signer_info();
                println!("  Type: {}", info.signer_type);
                println!("  Algorithm: {}", info.algorithm);
                
                // Test signing
                let data = b"Hello, HSM world!";
                let signature = signer.sign(data).await?;
                println!("  Signature: {}", signature);
                
                let public_key = signer.public_key().await?;
                println!("  Public key: {}", public_key);
            }
            Err(e) => println!("✗ Failed to create PKCS#11 signer: {}", e),
        }
    } else {
        println!("  Skipping PKCS#11 example (ERST_PKCS11_MODULE not set)");
    }

    println!();

    // Example 3: Generate a new software key pair
    println!("3. Generating new software key pair...");
    match crate::hsm::software::SoftwareSigner::generate() {
        Ok((signer, pem_data)) => {
            println!("✓ New key pair generated");
            println!("  Private key (PEM):\n{}", pem_data);
            
            let info = signer.signer_info();
            println!("  Signer info: {:?}", info);
            
            // Test signing with the new key
            let data = b"Test message with new key";
            let signature = signer.sign(data).await?;
            println!("  Test signature: {}", signature);
        }
        Err(e) => println!("✗ Failed to generate key pair: {}", e),
    }

    println!("\n=== Example completed ===");
    Ok(())
}

/// Example of environment variable setup for HSM
pub fn print_environment_setup() {
    println!("=== HSM Environment Setup ===\n");
    
    println!("For software signer:");
    println!("  ERST_SIGNER_TYPE=software");
    println!("  ERST_SIGNER_ALGORITHM=ed25519");
    println!("  ERST_SOFTWARE_PRIVATE_KEY_PATH=/path/to/private_key.pem");
    println!("  # OR");
    println!("  ERST_SOFTWARE_PRIVATE_KEY_PEM='-----BEGIN PRIVATE KEY-----...'\n");
    
    println!("For PKCS#11 signer:");
    println!("  ERST_SIGNER_TYPE=pkcs11");
    println!("  ERST_SIGNER_ALGORITHM=ed25519");
    println!("  ERST_PKCS11_MODULE=/usr/lib/libykcs11.so");
    println!("  ERST_PKCS11_PIN=123456");
    println!("  ERST_PKCS11_TOKEN_LABEL=YubiKey PIV");
    println!("  # OR");
    println!("  ERST_PKCS11_SLOT=0");
    println!("  ERST_PKCS11_KEY_LABEL=MySigningKey");
    println!("  # OR");
    println!("  ERST_PKCS11_KEY_ID=01");
    println!("  # OR");
    println!("  ERST_PKCS11_PIV_SLOT=9a");
    println!("  ERST_PKCS11_PUBLIC_KEY_PEM='-----BEGIN PUBLIC KEY-----...'\n");
    
    println!("YubiKey PIV slot mapping:");
    println!("  9a -> Key ID 1 (PIV Authentication)");
    println!("  9c -> Key ID 2 (Digital Signature)");
    println!("  9d -> Key ID 3 (Key Management)");
    println!("  9e -> Key ID 4 (Card Authentication)");
    println!("  82-95 -> Key IDs 5-24 (Retired Keys)");
    println!("  f9 -> Key ID 25 (Attestation)\n");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_example_hsm_usage() {
        // This test demonstrates the example usage
        // In practice, you'd need proper key files and HSM setup
        
        // Test with a generated key
        let (signer, _pem) = crate::hsm::software::SoftwareSigner::generate().unwrap();
        
        let data = b"Test message";
        let signature = signer.sign(data).await.unwrap();
        let public_key = signer.public_key().await.unwrap();
        
        assert_eq!(signature.algorithm, "ed25519");
        assert_eq!(public_key.algorithm, "ed25519");
        assert!(!signature.bytes.is_empty());
        assert!(!public_key.spki_bytes.is_empty());
    }

    #[test]
    fn test_print_environment_setup() {
        // Just ensures the function doesn't panic
        print_environment_setup();
    }
}
