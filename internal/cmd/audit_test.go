// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKeyPair() (string, ed25519.PublicKey) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	return hex.EncodeToString(priv.Seed()), pub
}

func TestGenerate_WithoutAttestation(t *testing.T) {
	privHex, _ := generateTestKeyPair()

	log, err := Generate(
		"tx_abc123",
		"envelope_xdr_data",
		"result_meta_xdr_data",
		[]string{"event1", "event2"},
		[]string{"log1"},
		privHex,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, "1.1.0", log.Version)
	assert.Equal(t, "tx_abc123", log.TransactionHash)
	assert.Nil(t, log.HardwareAttestation)
	assert.NotEmpty(t, log.TraceHash)
	assert.NotEmpty(t, log.Signature)
	assert.NotEmpty(t, log.PublicKey)

	// Verify
	valid, err := VerifyAuditLog(log)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestGenerate_WithAttestation(t *testing.T) {
	privHex, _ := generateTestKeyPair()

	attestation := &HardwareAttestation{
		Certificates: []AttestationCertificate{
			{
				PEM:     "-----BEGIN CERTIFICATE-----\nMIIBfTCB...\n-----END CERTIFICATE-----",
				Subject: "CN=YubiKey PIV Attestation",
				Issuer:  "CN=Yubico PIV Root CA",
				Serial:  "0a1b2c3d",
			},
			{
				PEM:     "-----BEGIN CERTIFICATE-----\nMIIBgTCB...\n-----END CERTIFICATE-----",
				Subject: "CN=Yubico PIV Root CA",
				Issuer:  "CN=Yubico PIV Root CA",
				Serial:  "0001",
			},
		},
		TokenInfo:        "YubiKey 5 (Yubico)",
		KeyNonExportable: true,
		RetrievedAt:      "2026-02-24T14:00:00Z",
	}

	log, err := Generate(
		"tx_def456",
		"envelope_data",
		"meta_data",
		[]string{"evt"},
		[]string{"log"},
		privHex,
		&GenerateOptions{HardwareAttestation: attestation},
	)

	require.NoError(t, err)
	assert.NotNil(t, log.HardwareAttestation)
	assert.Equal(t, 2, len(log.HardwareAttestation.Certificates))
	assert.Equal(t, "YubiKey 5 (Yubico)", log.HardwareAttestation.TokenInfo)
	assert.True(t, log.HardwareAttestation.KeyNonExportable)

	// Verify
	valid, err := VerifyAuditLog(log)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestVerify_DetectsTamperedAttestation(t *testing.T) {
	privHex, _ := generateTestKeyPair()

	attestation := &HardwareAttestation{
		Certificates: []AttestationCertificate{
			{
				PEM:     "-----BEGIN CERTIFICATE-----\nMIIBfTCB...\n-----END CERTIFICATE-----",
				Subject: "CN=YubiKey PIV Attestation",
				Issuer:  "CN=Yubico PIV Root CA",
				Serial:  "aabb",
			},
		},
		TokenInfo:        "YubiKey 5",
		KeyNonExportable: true,
		RetrievedAt:      "2026-02-24T14:00:00Z",
	}

	log, err := Generate(
		"tx_tamper",
		"env",
		"meta",
		[]string{},
		[]string{},
		privHex,
		&GenerateOptions{HardwareAttestation: attestation},
	)
	require.NoError(t, err)

	// Tamper with attestation
	log.HardwareAttestation.KeyNonExportable = false

	valid, err := VerifyAuditLog(log)
	require.NoError(t, err)
	assert.False(t, valid, "tampering with attestation data should invalidate the log")
}

func TestVerify_DetectsRemovedAttestation(t *testing.T) {
	privHex, _ := generateTestKeyPair()

	attestation := &HardwareAttestation{
		Certificates: []AttestationCertificate{
			{
				PEM:     "-----BEGIN CERTIFICATE-----\nMIIBfTCBaQIBATANBg...\n-----END CERTIFICATE-----",
				Subject: "CN=Test Attestation",
				Issuer:  "CN=Test Root",
				Serial:  "ff00",
			},
		},
		TokenInfo:        "TestHSM",
		KeyNonExportable: true,
		RetrievedAt:      "2026-02-24T14:00:00Z",
	}

	log, err := Generate(
		"tx_strip",
		"env",
		"meta",
		[]string{},
		[]string{},
		privHex,
		&GenerateOptions{HardwareAttestation: attestation},
	)
	require.NoError(t, err)

	// Strip attestation
	log.HardwareAttestation = nil

	valid, err := VerifyAuditLog(log)
	require.NoError(t, err)
	assert.False(t, valid, "removing attestation should invalidate the hash")
}

func TestVerify_DetectsTamperedPayload(t *testing.T) {
	privHex, _ := generateTestKeyPair()

	log, err := Generate(
		"tx_payload_tamper",
		"env",
		"meta",
		[]string{"event1"},
		[]string{},
		privHex,
		nil,
	)
	require.NoError(t, err)

	// Tamper with payload
	log.Payload.Events = []string{"event1", "injected"}

	valid, err := VerifyAuditLog(log)
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestAuditLog_JSONSerialization(t *testing.T) {
	privHex, _ := generateTestKeyPair()

	attestation := &HardwareAttestation{
		Certificates: []AttestationCertificate{
			{
				PEM:     "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
				Subject: "CN=Test",
				Issuer:  "CN=Root",
				Serial:  "01",
			},
		},
		TokenInfo:        "TestToken",
		KeyNonExportable: true,
		RetrievedAt:      "2026-02-24T14:00:00Z",
	}

	log, err := Generate(
		"tx_json",
		"env",
		"meta",
		[]string{},
		[]string{},
		privHex,
		&GenerateOptions{HardwareAttestation: attestation},
	)
	require.NoError(t, err)

	// Marshal and unmarshal
	data, err := json.Marshal(log)
	require.NoError(t, err)

	var decoded AuditLog
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.NotNil(t, decoded.HardwareAttestation)
	assert.Equal(t, "TestToken", decoded.HardwareAttestation.TokenInfo)
	assert.True(t, decoded.HardwareAttestation.KeyNonExportable)
	assert.Equal(t, 1, len(decoded.HardwareAttestation.Certificates))

	// Verify round-tripped log
	valid, err := VerifyAuditLog(&decoded)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestGenerate_InvalidPrivateKey(t *testing.T) {
	_, err := Generate("tx", "env", "meta", nil, nil, "invalid_hex", nil)
	assert.Error(t, err)

	_, err = Generate("tx", "env", "meta", nil, nil, "aabb", nil)
	assert.Error(t, err)
}
