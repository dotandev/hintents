# HSM Integration Guide

## Overview

The Erst simulator now supports Hardware Security Module (HSM) integration for cryptographic operations through a generic Signer interface. This allows for secure key management and signing operations using both software-based keys and hardware-backed keys via PKCS#11.

## Architecture

### Generic Signer Interface

The HSM integration is built around a generic `Signer` trait that abstracts cryptographic operations:

```rust
#[async_trait]
pub trait Signer: Send + Sync {
    async fn sign(&self, data: &[u8]) -> Result<Signature, SignerError>;
    async fn public_key(&self) -> Result<PublicKey, SignerError>;
    fn signer_info(&self) -> SignerInfo;
}
```

### Supported Implementations

1. **Software Signer**: Uses local Ed25519/secp256k1 keys
2. **PKCS#11 Signer**: Interfaces with hardware security modules

## Quick Start

### Software Signer

```rust
use erst::hsm::{SoftwareSigner, SignerFactory};

// Generate a new key pair
let (signer, pem_data) = SoftwareSigner::generate()?;

// Use the signer
let data = b"Hello, world!";
let signature = signer.sign(data).await?;
let public_key = signer.public_key().await?;
```

### PKCS#11 HSM Signer

```rust
use erst::hsm::{SignerFactory, SignerConfig};

// Configure from environment
let signer = SignerFactory::create_from_env().await?;

// Use the HSM signer
let data = b"Secure transaction";
let signature = signer.sign(data).await?;
```

## Configuration

### Environment Variables

#### Software Signer
```bash
export ERST_SIGNER_TYPE=software
export ERST_SIGNER_ALGORITHM=ed25519
export ERST_SOFTWARE_PRIVATE_KEY_PATH=/path/to/private_key.pem
# OR
export ERST_SOFTWARE_PRIVATE_KEY_PEM="-----BEGIN PRIVATE KEY-----..."
```

#### PKCS#11 Signer
```bash
export ERST_SIGNER_TYPE=pkcs11
export ERST_SIGNER_ALGORITHM=ed25519
export ERST_PKCS11_MODULE=/usr/lib/libykcs11.so
export ERST_PKCS11_PIN=123456
export ERST_PKCS11_TOKEN_LABEL=YubiKey PIV
# OR specify slot/key identifiers
export ERST_PKCS11_SLOT=0
export ERST_PKCS11_KEY_LABEL=MySigningKey
# OR
export ERST_PKCS11_KEY_ID=01
# OR
export ERST_PKCS11_PIV_SLOT=9a
```

### Programmatic Configuration

```rust
use erst::hsm::{SignerConfig, SoftwareSignerConfig, Pkcs11SignerConfig};

let config = SignerConfig {
    signer_type: "pkcs11".to_string(),
    algorithm: "ed25519".to_string(),
    software: None,
    pkcs11: Some(Pkcs11SignerConfig {
        module_path: "/usr/lib/libykcs11.so".to_string(),
        pin: "123456".to_string(),
        token_label: Some("YubiKey PIV".to_string()),
        slot_index: None,
        key_label: None,
        key_id_hex: Some("01".to_string()),
        piv_slot: None,
        public_key_pem: None,
    }),
};

let signer = SignerFactory::create_from_config(&config).await?;
```

## Supported Hardware

### YubiKey

YubiKey devices with PIV support are fully compatible:

```bash
# YubiKey PKCS#11 module (usually installed with yubikey-manager)
export ERST_PKCS11_MODULE=/usr/lib/x86_64-linux-gnu/libykcs11.so

# Common PIV slots
export ERST_PKCS11_PIV_SLOT=9a  # PIV Authentication
export ERST_PKCS11_PIV_SLOT=9c  # Digital Signature
export ERST_PKCS11_PIV_SLOT=9d  # Key Management
```

### Generic PKCS#11 Modules

Any PKCS#11-compliant HSM should work:

```bash
# SoftHSM (software-based HSM for testing)
export ERST_PKCS11_MODULE=/usr/lib/softhsm/libsofthsm2.so

# Nitrokey HSM
export ERST_PKCS11_MODULE=/usr/lib/libnitrokey.so

# AWS CloudHSM
export ERST_PKCS11_MODULE=/opt/cloudhsm/lib/libcloudhsm_pkcs11.so
```

## Key Management

### Generating Keys

#### Software Keys
```rust
use erst::hsm::software::SoftwareSigner;

let (signer, pem_data) = SoftwareSigner::generate()?;
println!("Private key:\n{}", pem_data);
```

#### HSM Keys
HSM keys must be generated using the HSM's native tools:

```bash
# YubiKey (using yubico-piv-tool)
yubico-piv-tool -a generate -s 9a -o public_key.pem -A ECCP256

# SoftHSM (using softhsm2-util)
softhsm2-util --init-token --free --label mytoken --pin 123456 --so-pin 123456
softhsm2-util --generate-key --algorithm ed25519 --label mykey --token mytoken
```

### Key Formats

#### Ed25519 Keys
- **Private**: PKCS#8 PEM format
- **Public**: SPKI DER format

#### secp256k1 Keys
- **Private**: PKCS#8 PEM format  
- **Public**: SPKI DER format

## Security Considerations

### Key Protection

1. **HSM Keys**: Never leave the HSM device
2. **Software Keys**: Store securely with restricted file permissions
3. **PIN Management**: Use strong PINs and avoid hardcoding

### Best Practices

1. **Use HSM for Production**: Hardware keys provide better security
2. **Backup Keys**: Maintain secure backups of critical keys
3. **Rotate Keys**: Regularly rotate signing keys
4. **Audit Access**: Monitor and log key usage

### Threat Mitigation

- **Key Extraction**: HSM prevents private key extraction
- **Side Channel Attacks**: Use hardware with side-channel protection
- **Physical Security**: Secure HSM devices physically

## Performance

### Benchmarks

| Signer Type | Algorithm | Sign Operation | Key Generation |
|-------------|-----------|----------------|----------------|
| Software | Ed25519 | ~0.1ms | ~1ms |
| Software | secp256k1 | ~0.5ms | ~5ms |
| HSM | Ed25519 | ~10ms | N/A |
| HSM | secp256k1 | ~15ms | N/A |

*Note: HSM times vary by device and connection type*

### Optimization Tips

1. **Reuse Sessions**: For HSM, reuse PKCS#11 sessions when possible
2. **Batch Operations**: Group multiple signing operations
3. **Connection Pooling**: For network-connected HSMs
4. **Caching**: Cache public keys to avoid HSM calls

## Troubleshooting

### Common Issues

#### PKCS#11 Module Not Found
```bash
Error: Failed to load PKCS#11 module: No such file or directory

Solution:
# Verify module path
ls -la /usr/lib/libykcs11.so

# Install YubiKey manager
sudo apt-get install yubikey-manager
# or
brew install yubikey-manager
```

#### HSM Not Detected
```bash
Error: No slots found

Solution:
# Check if HSM is connected
yubico-piv-tool -a status

# Verify permissions
sudo usermod -a -G plugdev $USER
# Log out and back in
```

#### PIN Incorrect
```bash
Error: Failed to login: 0x1000

Solution:
# Verify PIN
yubico-piv-tool -a verify-pin -P 123456

# Reset PIN if needed (requires PUK)
yubico-piv-tool -a change-pin -P 123456 -N 654321
```

#### Key Not Found
```bash
Error: Private key not found in HSM

Solution:
# List available keys
yubico-piv-tool -a list

# Generate key if needed
yubico-piv-tool -a generate -s 9a -o pubkey.pem -A ECCP256
```

### Debug Mode

Enable debug logging for troubleshooting:

```rust
use tracing_subscriber;

tracing_subscriber::fmt()
    .with_max_level(tracing::Level::DEBUG)
    .init();
```

## Migration Guide

### From In-Memory Keys

1. **Extract Current Keys**: Export existing keys to PEM format
2. **Generate HSM Keys**: Create keys in HSM device
3. **Update Configuration**: Switch to PKCS#11 signer
4. **Test Verification**: Ensure signatures verify correctly

### Example Migration

```rust
// Before (in-memory)
let private_key = generate_key();
let signature = sign_data(private_key, data);

// After (HSM)
let signer = SignerFactory::create_from_env().await?;
let signature = signer.sign(data).await?;
```

## Testing

### Unit Tests
```bash
cargo test hsm --lib
```

### Integration Tests
```bash
# Software signer tests
cargo test hsm_integration_tests --test hsm_integration_tests

# HSM tests (requires actual HSM)
ERST_PKCS11_MODULE=/usr/lib/libykcs11.so ERST_PKCS11_PIN=123456 \
cargo test pkcs11_integration --test hsm_integration_tests
```

### Example Usage
```bash
cargo run --example hsm_integration
```

## API Reference

### Core Types

- `Signer`: Generic signer trait
- `PublicKey`: Public key with algorithm
- `Signature`: Digital signature with algorithm
- `SignerError`: Error types for signing operations

### Implementations

- `SoftwareSigner`: Ed25519 software signer
- `Secp256k1SoftwareSigner`: secp256k1 software signer
- `Pkcs11Signer`: PKCS#11 HSM signer

### Configuration

- `SignerConfig`: Main configuration structure
- `SoftwareSignerConfig`: Software signer configuration
- `Pkcs11SignerConfig`: PKCS#11 signer configuration

## Contributing

When contributing to the HSM integration:

1. **Add Tests**: Include comprehensive tests for new features
2. **Document Changes**: Update documentation and examples
3. **Security Review**: Ensure security implications are considered
4. **Compatibility**: Test with multiple HSM devices

## Support

For issues with HSM integration:

1. **Check Documentation**: Review this guide and API docs
2. **Search Issues**: Look for similar problems in GitHub issues
3. **Provide Details**: Include HSM type, OS, and error messages
4. **Test Cases**: Provide minimal reproduction cases
