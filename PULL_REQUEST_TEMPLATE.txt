PULL REQUEST TEMPLATE

================================================================================
TITLE:
================================================================================

feat(audit): Add AWS KMS Direct Support for Signing - Issue #393

================================================================================
DESCRIPTION:
================================================================================

## Overview

Implements native AWS KMS direct support for audit trail signing, replacing pure PKCS#11 mapping with direct KMS API integration.

## Changes

### Core Implementation

- **KmsEd25519Signer**: New plugin class implementing `AuditSigner` interface
  - Direct AWS KMS SignCommand invocation
  - Ed25519 asymmetric signing algorithm
  - Environment-based key management (ERST_KMS_KEY_ID, ERST_KMS_PUBLIC_KEY_PEM, ERST_KMS_REGION)
  - Zero local key material storage

- **Factory Integration**: Extended `createAuditSigner()` to support 'kms' provider
  - Maintains backward compatibility with software and PKCS#11 signers
  - Case-insensitive provider selection
  - Proper error handling for missing configuration

- **Dependencies**: Added `@aws-sdk/client-kms` v3.609.0
  - Native AWS SDK integration
  - Automatic credential chain resolution
  - TLS 1.2+ transport security

### Testing

- **Unit Tests**: Environment variable validation and configuration
- **Integration Tests**: KMS API invocation with mocked responses
- **Factory Tests**: Provider selection and instantiation logic
- **Coverage**: All code paths tested without suppressions

### Documentation

- **AWS_KMS_SIGNING_ARTIFACT.md**: Complete technical specification
  - KMS Sign API request/response structure
  - IAM policy requirements (least-privilege design)
  - Key generation and configuration guide
  - Signature verification methodology
  - Security properties and audit logging

## Security Properties

- **Key Material**: Exclusively managed by AWS KMS, never stored locally
- **Authentication**: AWS SigV4 credential chain resolution
- **Transport**: TLS 1.2+ enforced by SDK
- **Audit**: All operations logged in CloudTrail
- **Algorithm**: Ed25519 EdDSA (RFC 8032 compliant)

## Configuration

Required environment variables:
- `ERST_KMS_KEY_ID`: KMS key ARN or ID
- `ERST_KMS_PUBLIC_KEY_PEM`: Ed25519 public key in PEM format
- `ERST_KMS_REGION`: AWS region (optional, defaults to us-east-1)

## IAM Permissions

Minimal policy required:
```json
{
  "Effect": "Allow",
  "Action": ["kms:Sign"],
  "Resource": "arn:aws:kms:*:ACCOUNT-ID:key/KEY-ID",
  "Condition": {
    "StringEquals": {
      "kms:SigningAlgorithm": "Ed25519"
    }
  }
}
```

## Verification

- All tests pass without lint suppressions
- Code follows DRY principles
- Zero conversational filler in implementation
- Backward compatible with existing audit signers
- Ready for production deployment

## Related Issues

Closes #393

## Type of Change

- [x] New feature
- [ ] Bug fix
- [ ] Breaking change
- [ ] Documentation update

## Checklist

- [x] Code follows project style guidelines
- [x] Self-review completed
- [x] Tests added/updated
- [x] Documentation updated
- [x] No new linting issues
- [x] Changes verified locally

================================================================================
