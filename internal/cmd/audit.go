// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
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
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 3. Calculate Trace Hash (SHA256)
	hash := sha256.Sum256(payloadBytes)
	traceHashHex := hex.EncodeToString(hash[:])

	// 4. Parse Private Key
	privKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize && len(privKeyBytes) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid private key length: %d", len(privKeyBytes))
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
