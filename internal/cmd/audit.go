// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dotandev/hintents/internal/errors"
)

// AttestationCertificate represents a single X.509 certificate in the
// hardware attestation chain. Certificates are ordered leaf-to-root.
type AttestationCertificate struct {
	PEM     string `json:"pem"`
	Subject string `json:"subject"`
	Issuer  string `json:"issuer"`
	Serial  string `json:"serial"`
}

// HardwareAttestation contains the full attestation chain retrieved from
// an HSM or hardware security token. When present in an AuditLog it
// provides cryptographic proof that the signing key resides on a
// hardware device and is non-exportable.
type HardwareAttestation struct {
	Certificates     []AttestationCertificate `json:"certificates"`
	TokenInfo        string                   `json:"token_info"`
	KeyNonExportable bool                     `json:"key_non_exportable"`
	RetrievedAt      string                   `json:"retrieved_at"`
}

// AuditLog represents the signed audit trail of a transaction simulation
type AuditLog struct {
	Version             string               `json:"version"`
	Timestamp           time.Time            `json:"timestamp"`
	TransactionHash     string               `json:"transaction_hash"`
	TraceHash           string               `json:"trace_hash"`
	Signature           string               `json:"signature"`
	PublicKey           string               `json:"public_key"`
	Payload             Payload              `json:"payload"`
	HardwareAttestation *HardwareAttestation `json:"hardware_attestation,omitempty"`
}

// Payload contains the actual trace data
type Payload struct {
	EnvelopeXdr   string   `json:"envelope_xdr"`
	ResultMetaXdr string   `json:"result_meta_xdr"`
	Events        []string `json:"events"`
	Logs          []string `json:"logs"`
}

// GenerateOptions controls optional audit generation behavior
type GenerateOptions struct {
	// HardwareAttestation, if non-nil, will be embedded in the signed
	// audit log. The attestation data is included in the hash to prevent
	// post-signing removal or substitution.
	HardwareAttestation *HardwareAttestation
}

// Generate creates a signed audit log from the simulation results.
// If opts is non-nil and contains a HardwareAttestation, the attestation
// chain is embedded and covered by the signature.
func Generate(txHash string, envelopeXdr, resultMetaXdr string, events, logs []string, privateKeyHex string, opts *GenerateOptions) (*AuditLog, error) {
	// 1. Construct Payload
	payload := Payload{
		EnvelopeXdr:   envelopeXdr,
		ResultMetaXdr: resultMetaXdr,
		Events:        events,
		Logs:          logs,
	}

	// 2. Construct the hash input.
	// When hardware attestation is present, it is included in the hash
	// so that stripping it would invalidate the signature.
	type hashInput struct {
		Payload             Payload              `json:"payload"`
		HardwareAttestation *HardwareAttestation `json:"hardware_attestation,omitempty"`
	}

	hi := hashInput{Payload: payload}
	if opts != nil && opts.HardwareAttestation != nil {
		hi.HardwareAttestation = opts.HardwareAttestation
	}

	payloadBytes, err := json.Marshal(hi)
	if err != nil {
		return nil, errors.WrapMarshalFailed(err)
	}

	// 3. Calculate Trace Hash (SHA256)
	hash := sha256.Sum256(payloadBytes)
	traceHashHex := hex.EncodeToString(hash[:])

	// 4. Parse Private Key
	privKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, errors.WrapValidationError(fmt.Sprintf("invalid private key hex: %v", err))
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize && len(privKeyBytes) != ed25519.SeedSize {
		return nil, errors.WrapValidationError(fmt.Sprintf("invalid private key length: %d", len(privKeyBytes)))
	}

	var privateKey ed25519.PrivateKey
	if len(privKeyBytes) == ed25519.SeedSize {
		privateKey = ed25519.NewKeyFromSeed(privKeyBytes)
	} else {
		privateKey = ed25519.PrivateKey(privKeyBytes)
	}

	// 5. Sign the Trace Hash
	// We sign the hash of the payload to ensure integrity.
	signature := ed25519.Sign(privateKey, hash[:])

	auditLog := &AuditLog{
		Version:         "1.1.0",
		Timestamp:       time.Now().UTC(),
		TransactionHash: txHash,
		TraceHash:       traceHashHex,
		Signature:       hex.EncodeToString(signature),
		PublicKey:       hex.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
		Payload:         payload,
	}

	if opts != nil && opts.HardwareAttestation != nil {
		auditLog.HardwareAttestation = opts.HardwareAttestation
	}

	return auditLog, nil
}

// VerifyAuditLog verifies the integrity and signature of an AuditLog.
// It returns true if both the hash and signature are valid.
func VerifyAuditLog(auditLog *AuditLog) (bool, error) {
	// Re-construct hash input
	type hashInput struct {
		Payload             Payload              `json:"payload"`
		HardwareAttestation *HardwareAttestation `json:"hardware_attestation,omitempty"`
	}

	hi := hashInput{Payload: auditLog.Payload}
	if auditLog.HardwareAttestation != nil {
		hi.HardwareAttestation = auditLog.HardwareAttestation
	}

	payloadBytes, err := json.Marshal(hi)
	if err != nil {
		return false, fmt.Errorf("failed to marshal payload for verification: %w", err)
	}

	// Verify hash
	hash := sha256.Sum256(payloadBytes)
	expectedHash := hex.EncodeToString(hash[:])
	if expectedHash != auditLog.TraceHash {
		return false, nil
	}

	// Verify signature
	pubKeyBytes, err := hex.DecodeString(auditLog.PublicKey)
	if err != nil {
		return false, fmt.Errorf("invalid public key hex: %w", err)
	}

	sigBytes, err := hex.DecodeString(auditLog.Signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}

	return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), hash[:], sigBytes), nil
}
