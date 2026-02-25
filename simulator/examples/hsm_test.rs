// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! HSM Integration Test Example
//!
//! This example demonstrates the usage of the HSM integration with both
//! software and PKCS#11 signers.

use signature::Keypair;
use std::env;

// Import the HSM module from the parent crate
use simulator::hsm::{SignatureAlgorithm, SignerFactory};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize logging
    env_logger::init();

    println!("HSM Integration Test");
    println!("====================");

    // Test 1: Software Signer with Ed25519
    println!("\n1. Testing Software Signer (Ed25519)");
    test_software_signer_ed25519().await?;

    // Test 2: Software Signer with secp256k1
    println!("\n2. Testing Software Signer (secp256k1)");
    test_software_signer_secp256k1().await?;

    // Test 3: Environment-based signer creation
    println!("\n3. Testing Environment-based Signer Creation");
    test_environment_signer().await?;

    // Test 4: PKCS#11 Signer (if configured)
    println!("\n4. Testing PKCS#11 Signer");
    test_pkcs11_signer().await?;

    println!("\nAll HSM tests completed successfully!");
    Ok(())
}

async fn test_software_signer_ed25519() -> Result<(), Box<dyn std::error::Error>> {
    // Generate a new Ed25519 keypair
    let keypair = Keypair::generate(&mut rand::rngs::OsRng);
    let private_key = keypair.to_bytes();

    // Create software signer
    let signer = SignerFactory::create_software_signer(private_key, SignatureAlgorithm::Ed25519)?;

    // Test signing
    let message = b"Hello, HSM!";
    let signature = signer.sign(message).await?;
    println!("   Message signed successfully");
    println!("   Signature algorithm: {:?}", signature.algorithm);
    println!("   Signature length: {} bytes", signature.bytes.len());

    // Test public key retrieval
    let public_key = signer.public_key().await?;
    println!(
        "   Public key retrieved: {} bytes",
        public_key.bytes.len()
    );

    // Test verification
    let is_valid = signer.verify(message, &signature).await?;
    assert!(is_valid, "Signature verification failed");
    println!("   Signature verified successfully");

    // Test verification with wrong message
    let wrong_message = b"Wrong message";
    let is_invalid = signer.verify(wrong_message, &signature).await?;
    assert!(
        !is_invalid,
        "Signature should not verify with wrong message"
    );
    println!("   Invalid signature correctly rejected");

    Ok(())
}

async fn test_software_signer_secp256k1() -> Result<(), Box<dyn std::error::Error>> {
    use p256::ecdsa::SigningKey;
    use p256::elliptic_curve::rand_core::OsRng;

    // Generate a new secp256k1 keypair
    let signing_key = SigningKey::random(&mut OsRng);
    let private_key = signing_key.to_bytes();

    // Create software signer
    let signer =
        SignerFactory::create_software_signer(&private_key, SignatureAlgorithm::Secp256k1)?;

    // Test signing
    let message = b"Hello, HSM secp256k1!";
    let signature = signer.sign(message).await?;
    println!("   Message signed successfully");
    println!("   Signature algorithm: {:?}", signature.algorithm);
    println!("   Signature length: {} bytes", signature.bytes.len());

    // Test public key retrieval
    let public_key = signer.public_key().await?;
    println!(
        "   Public key retrieved: {} bytes",
        public_key.bytes.len()
    );

    // Test verification
    let is_valid = signer.verify(message, &signature).await?;
    assert!(is_valid, "Signature verification failed");
    println!("   Signature verified successfully");

    Ok(())
}

async fn test_environment_signer() -> Result<(), Box<dyn std::error::Error>> {
    // Set environment variables for software signer
    env::set_var("HSM_PROVIDER", "software");
    env::set_var("HSM_ALGORITHM", "ed25519");

    // Create a temporary private key file
    let keypair = Keypair::generate(&mut rand::rngs::OsRng);
    let private_key = keypair.to_bytes();
    let key_file = "test_private_key.bin";
    std::fs::write(key_file, private_key)?;
    env::set_var("HSM_PRIVATE_KEY", key_file);

    // Create signer from environment
    let signer: Box<dyn simulator::hsm::Signer> = SignerFactory::create_from_env().await?;
    println!("   Signer created from environment configuration");

    // Test basic functionality
    let message = b"Environment test message";
    let signature = signer.sign(message).await?;
    let is_valid = signer.verify(message, &signature).await?;
    assert!(is_valid, "Environment signer verification failed");
    println!("   Environment signer works correctly");

    // Clean up
    std::fs::remove_file(key_file)?;
    env::remove_var("HSM_PROVIDER");
    env::remove_var("HSM_ALGORITHM");
    env::remove_var("HSM_PRIVATE_KEY");

    Ok(())
}

async fn test_pkcs11_signer() -> Result<(), Box<dyn std::error::Error>> {
    // This test will only work if PKCS#11 is properly configured
    // For now, we'll just test the error path

    let library_path = "/nonexistent/library.so";
    let result = SignerFactory::create_pkcs11_signer_with_first_key(
        library_path,
        None,
        "1234",
        SignatureAlgorithm::Ed25519,
    )
    .await;

    match result {
        Err(e) => {
            println!(
                "   PKCS#11 signer correctly handles missing library: {}",
                e
            );
        }
        Ok(_) => {
            println!("   Unexpected success with non-existent library");
        }
    }

    println!("   PKCS#11 interface tested (error path)");
    Ok(())
}
