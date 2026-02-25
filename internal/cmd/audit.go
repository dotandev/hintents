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

// AuditLog represents the signed audit trail of a transaction simulation
type AuditLog struct {
	Version         string    `json:"version"`
	Timestamp       time.Time `json:"timestamp"`
	TransactionHash string    `json:"transaction_hash"`
	TraceHash       string    `json:"trace_hash"`
	Signature       string    `json:"signature"`
	PublicKey       string    `json:"public_key"`
	Payload         Payload   `json:"payload"`
}

// Payload contains the actual trace data
type Payload struct {
	EnvelopeXdr   string   `json:"envelope_xdr"`
	ResultMetaXdr string   `json:"result_meta_xdr"`
	Events        []string `json:"events"`
	Logs          []string `json:"logs"`
}

// Generate creates a signed audit log from the simulation results
func Generate(txHash string, envelopeXdr, resultMetaXdr string, events, logs []string, privateKeyHex string) (*AuditLog, error) {
	// 1. Construct Payload
	payload := Payload{
		EnvelopeXdr:   envelopeXdr,
		ResultMetaXdr: resultMetaXdr,
		Events:        events,
		Logs:          logs,
	}

	// 2. Serialize Payload to calculate hash
	payloadBytes, err := json.Marshal(payload)
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

	return &AuditLog{
		Version:         "1.0.0",
		Timestamp:       time.Now().UTC(),
		TransactionHash: txHash,
		TraceHash:       traceHashHex,
		Signature:       hex.EncodeToString(signature),
		PublicKey:       hex.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
		Payload:         payload,
	}, nil
}
