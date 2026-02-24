AWS KMS Direct Support for Audit Log Signing

OVERVIEW
The KmsEd25519Signer plugin provides native AWS KMS integration for signing audit trails
using the AWS Key Management Service API with Ed25519 asymmetric cryptography. This replaces
pure PKCS#11 dependency for users with AWS KMS infrastructure.

AWS KMS SIGN API REQUEST STRUCTURE
============================================

POST /
X-Amz-Target: TrentService.Sign
Content-Type: application/x-amz-json-1.1

{
  "KeyId": "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
  "Message": "<base64-encoded-payload>",
  "SigningAlgorithm": "Ed25519",
  "MessageFormat": "RAW"
}

RESPONSE
{
  "Signature": "<base64-encoded-signature>",
  "KeyId": "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
  "SigningAlgorithm": "Ed25519"
}

ENVIRONMENT VARIABLES
============================================

Required:
- ERST_KMS_KEY_ID: The KMS key identifier (ARN or key ID)
  Example: arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012
  
- ERST_KMS_PUBLIC_KEY_PEM: SPKI PEM format public key for verification and audit metadata
  Example: -----BEGIN PUBLIC KEY-----
           MCowBQYDK2VwAyEAGb+DYvh6SEqVTm50DFtMDoQikTmiCqirVv9mWG9qfSnCoAs=
           -----END PUBLIC KEY-----

Optional:
- ERST_KMS_REGION: AWS region (default: us-east-1)
  Example: us-west-2

IAM PERMISSIONS REQUIRED
============================================

Minimum policy for KMS signing:

{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "KmsSignPermission",
      "Effect": "Allow",
      "Action": [
        "kms:Sign"
      ],
      "Resource": "arn:aws:kms:*:ACCOUNT_ID:key/KEY_ID",
      "Condition": {
        "StringEquals": {
          "kms:SigningAlgorithm": "Ed25519"
        }
      }
    },
    {
      "Sid": "KmsDescribePermission",
      "Effect": "Allow",
      "Action": [
        "kms:DescribeKey"
      ],
      "Resource": "arn:aws:kms:*:ACCOUNT_ID:key/KEY_ID"
    }
  ]
}

USAGE
============================================

1. Export environment variables:
   export ERST_KMS_REGION=us-east-1
   export ERST_KMS_KEY_ID=arn:aws:kms:us-east-1:123456789012:key/12345678...
   export ERST_KMS_PUBLIC_KEY_PEM="$(cat ./ed25519-public-key-spki.pem)"

2. Invoke the audit signing command with KMS provider:
   node dist/index.js audit:sign \
     --hsm-provider kms \
     --payload '{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}'

IMPLEMENTATION DETAILS
============================================

Class: KmsEd25519Signer
Location: src/audit/signing/kmsSigner.ts

Implements:
- AuditSigner interface (sign, public_key methods)
- Lazy initialization of KMSClient
- Ed25519 signing with asymmetric key operations
- Deterministic signature generation for audit trails
- Error handling with clear messaging

Configuration:
- Region defaults to us-east-1 if ERST_KMS_REGION not set
- Validates required env vars (keyId, publicKeyPem) at construction time
- Public key is sourced from environment (no key derivation needed)

Signing Process:
1. Create KMS SignCommand with payload, KeyId, and Ed25519 algorithm
2. Send command via KMSClient
3. Extract signature bytes from response
4. Return as Buffer for compatibility with audit logger

TESTING
============================================

Unit Tests: tests/kms-signer.test.ts
- Mock KMS API responses
- Validate environment variable handling
- Test error conditions (missing env vars, API failures)
- Verify integration with AuditLogger
- Confirm signature structure

Integration Tests: tests/signer-factory.test.ts
- Test factory creates KMS signer when provider='kms'
- Verify case-insensitive provider selection
- Ensure backward compatibility with software/pkcs11 signers

Error Handling:
- Missing ERST_KMS_KEY_ID: "KMS signer selected but ERST_KMS_KEY_ID is not set"
- Missing ERST_KMS_PUBLIC_KEY_PEM: "KMS signer selected but ERST_KMS_PUBLIC_KEY_PEM is not set"
- KMS API errors: "kms signing failed: <API error message>"
- Missing signature in response: "KMS Sign response missing signature"

BACKWARD COMPATIBILITY
============================================

- Existing software signer unaffected
- Existing PKCS#11 signer unaffected
- Factory pattern maintains provider selection logic
- No breaking changes to AuditSigner interface
- HsmProvider type extended to include 'kms'

SECURITY CONSIDERATIONS
============================================

1. Key Material Protection:
   - Private key never leaves KMS
   - Public key distributed via environment
   - Signatures generated server-side in AWS

2. IAM Controls:
   - Fine-grained signing permission only
   - Algorithm restriction to Ed25519
   - Audit trail via CloudTrail

3. Data Integrity:
   - Ed25519 asymmetric cryptography
   - Deterministic signatures
   - SHA256 hash of audit payload

4. Error Handling:
   - No secret logging
   - Clear error messages without key exposure
   - Proper exception propagation

PERFORMANCE
============================================

Latency:
- KMS API calls typically 100-500ms
- Network round-trip to AWS
- Suitable for audit logging use case

Throughput:
- KMS quotas: Check AWS documentation
- Default: 10 API calls per second per key
- Configurable via service quota requests

Caching:
- Public key cached via environment variable
- KMSClient reused across requests
- No credential caching (IAM role or explicit credentials)

TROUBLESHOOTING
============================================

1. AccessDenied Error:
   - Verify IAM policy attached to identity
   - Ensure key ARN matches policy Resource
   - Check kms:Sign permission present
   - Verify Ed25519 algorithm in Condition block

2. InvalidKeyId Error:
   - Validate ERST_KMS_KEY_ID format
   - Confirm key exists in specified region
   - Check ERST_KMS_REGION matches key region

3. Network Errors:
   - Verify AWS SDK credentials configured (env/role/profile)
   - Check network connectivity to KMS endpoint
   - Confirm region endpoint availability

4. Signature Verification Failures:
   - Ensure ERST_KMS_PUBLIC_KEY_PEM matches private key
   - Verify public key is in SPKI PEM format
   - Confirm payload encoding (UTF-8)

REFERENCES
============================================

- AWS KMS Sign API: https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html
- SigningAlgorithmSpec: https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html#KMS-Sign-request-SigningAlgorithm
- Ed25519 Signing: https://datatracker.ietf.org/doc/html/rfc8032
- SPKI PEM Format: https://datatracker.ietf.org/doc/html/rfc5280
- IAM Best Practices: https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html
