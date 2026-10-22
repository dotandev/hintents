// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package offline implements an air-gapped transaction signing pipeline.
//
// The workflow is:
//  1. Generate – build an unsigned TransactionEnvelope and save it to a portable file.
//  2. Sign    – on an air-gapped machine, load the file, sign with a secret key, and write back.
//  3. Verify  – optionally verify the signed envelope before submission.
//  4. Submit  – bring the signed file back online and submit it to the Stellar network.
package offline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// EnvelopeFile is the portable JSON structure written to / read from disk.
// It contains everything needed to sign and later submit a transaction.
type EnvelopeFile struct {
	// Version of the envelope file format.
	Version int `json:"version"`

	// Network is the target Stellar network (testnet, mainnet, futurenet).
	Network string `json:"network"`

	// NetworkPassphrase is the passphrase required for signing.
	NetworkPassphrase string `json:"network_passphrase"`

	// EnvelopeXDR is the base64-encoded unsigned TransactionEnvelope XDR.
	EnvelopeXDR string `json:"envelope_xdr"`

	// Signatures collected so far (hex-encoded ed25519 signatures).
	Signatures []SignatureEntry `json:"signatures,omitempty"`

	// Checksum is the SHA-256 hex digest of EnvelopeXDR (integrity check).
	Checksum string `json:"checksum"`

	// Metadata carries human-readable context.
	Metadata EnvelopeMetadata `json:"metadata"`
}

// SignatureEntry records a single signature and the public key that produced it.
type SignatureEntry struct {
	PublicKey string `json:"public_key"` // hex-encoded ed25519 public key
	Signature string `json:"signature"`  // hex-encoded ed25519 signature
	SignedAt  string `json:"signed_at"`  // RFC 3339 timestamp
}

// EnvelopeMetadata provides human-readable context embedded in the file.
type EnvelopeMetadata struct {
	CreatedAt   string `json:"created_at"`
	Description string `json:"description,omitempty"`
	SourceAddr  string `json:"source_address,omitempty"`
	TxHash      string `json:"tx_hash,omitempty"`
	ErstVersion string `json:"erst_version,omitempty"`
}

// currentFormatVersion is bumped when the on-disk JSON schema changes.
const currentFormatVersion = 1

// NewEnvelopeFile builds an EnvelopeFile ready to be written to disk.
func NewEnvelopeFile(network, passphrase, envelopeXDR string, meta EnvelopeMetadata) *EnvelopeFile {
	meta.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	return &EnvelopeFile{
		Version:           currentFormatVersion,
		Network:           network,
		NetworkPassphrase: passphrase,
		EnvelopeXDR:       envelopeXDR,
		Checksum:          checksumOf(envelopeXDR),
		Metadata:          meta,
	}
}

// SaveToFile serialises the envelope to pretty-printed JSON and writes it to path.
func (e *EnvelopeFile) SaveToFile(path string) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return errors.WrapMarshalFailed(err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to write envelope file: %v", err))
	}

	return nil
}

// LoadEnvelopeFile reads and validates an envelope file from disk.
func LoadEnvelopeFile(path string) (*EnvelopeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.WrapValidationError(fmt.Sprintf("failed to read envelope file: %v", err))
	}

	var ef EnvelopeFile
	if err := json.Unmarshal(data, &ef); err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "envelope file")
	}

	if err := ef.Validate(); err != nil {
		return nil, err
	}

	return &ef, nil
}

// Validate performs integrity and format checks on the envelope.
func (e *EnvelopeFile) Validate() error {
	if e.Version != currentFormatVersion {
		return errors.WrapValidationError(
			fmt.Sprintf("unsupported envelope version %d (expected %d)", e.Version, currentFormatVersion),
		)
	}

	if e.EnvelopeXDR == "" {
		return errors.WrapValidationError("envelope_xdr is empty")
	}

	if e.NetworkPassphrase == "" {
		return errors.WrapValidationError("network_passphrase is empty")
	}

	// Verify checksum integrity.
	if expected := checksumOf(e.EnvelopeXDR); expected != e.Checksum {
		return errors.WrapValidationError(
			fmt.Sprintf("checksum mismatch: expected %s, got %s", expected, e.Checksum),
		)
	}

	return nil
}

// IsSigned returns true if at least one signature is present.
func (e *EnvelopeFile) IsSigned() bool {
	return len(e.Signatures) > 0
}

// checksumOf returns the hex-encoded SHA-256 digest of s.
func checksumOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}


// ErrInvalidSequenceOrder is returned when sequential transactions for the same
// source account in a batch do not strictly increment the sequence number by 1.
var ErrInvalidSequenceOrder = errors.New("invalid sequence order in batch")

// SequenceOrderConflictError records a sequence order mismatch between two envelopes.
type SequenceOrderConflictError struct {
	SourceAccount  string
	ExpectedSeq    int64
	ActualSeq      int64
	PreviousTxHash string
	CurrentTxHash  string
}

func (e *SequenceOrderConflictError) Error() string {
	return fmt.Sprintf(
		"%v: account %s expected sequence %d, got %d (previous tx: %s, conflicting tx: %s)",
		ErrInvalidSequenceOrder,
		e.SourceAccount,
		e.ExpectedSeq,
		e.ActualSeq,
		e.PreviousTxHash,
		e.CurrentTxHash,
	)
}

func (e *SequenceOrderConflictError) Is(target error) bool {
	return target == ErrInvalidSequenceOrder
}

type accountSequenceTracker struct {
	seq    int64
	txHash string
}

// ValidateBatchSequenceNumbers verifies that sequential transactions from the same
// source account strictly increment their sequence numbers (SeqNum + 1).
// If any sequence order violation or gap is detected, it returns a SequenceOrderConflictError
// matching ErrInvalidSequenceOrder with details about the conflicting transaction hashes.
func ValidateBatchSequenceNumbers(envelopes []*EnvelopeFile) error {
	if len(envelopes) <= 1 {
		return nil
	}

	tracked := make(map[string]accountSequenceTracker)

	for i, ef := range envelopes {
		if ef == nil {
			return errors.WrapValidationError(fmt.Sprintf("envelope at index %d is nil", i))
		}
		if ef.EnvelopeXDR == "" {
			return errors.WrapValidationError(fmt.Sprintf("envelope at index %d has empty envelope_xdr", i))
		}

		var env xdr.TransactionEnvelope
		if err := xdr.SafeUnmarshalBase64(ef.EnvelopeXDR, &env); err != nil {
			return errors.WrapUnmarshalFailed(err, fmt.Sprintf("envelope XDR at index %d", i))
		}

		source, seq, err := extractSourceAndSeq(&env)
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("envelope at index %d: %v", i, err))
		}

		txHash := effectiveTxHash(ef)

		if prev, exists := tracked[source]; exists {
			expectedSeq := prev.seq + 1
			if seq != expectedSeq {
				return &SequenceOrderConflictError{
					SourceAccount:  source,
					ExpectedSeq:    expectedSeq,
					ActualSeq:      seq,
					PreviousTxHash: prev.txHash,
					CurrentTxHash:  txHash,
				}
			}
		}

		tracked[source] = accountSequenceTracker{
			seq:    seq,
			txHash: txHash,
		}
	}

	return nil
}

// ValidateBatchSequenceNumbersXDR verifies sequence order directly from base64 XDR strings.
func ValidateBatchSequenceNumbersXDR(envelopeXDRs []string) error {
	envelopes := make([]*EnvelopeFile, len(envelopeXDRs))
	for i, xdrStr := range envelopeXDRs {
		envelopes[i] = &EnvelopeFile{
			EnvelopeXDR: xdrStr,
			Checksum:    checksumOf(xdrStr),
		}
	}
	return ValidateBatchSequenceNumbers(envelopes)
}

func extractSourceAndSeq(env *xdr.TransactionEnvelope) (string, int64, error) {
	switch env.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		if env.V0 == nil {
			return "", 0, fmt.Errorf("missing V0 transaction in envelope")
		}
		acc := xdr.AccountId{
			Type:    xdr.PublicKeyTypePublicKeyTypeEd25519,
			Ed25519: &env.V0.Tx.SourceAccountEd25519,
		}
		return acc.Address(), int64(env.V0.Tx.SeqNum), nil

	case xdr.EnvelopeTypeEnvelopeTypeTx:
		if env.V1 == nil {
			return "", 0, fmt.Errorf("missing V1 transaction in envelope")
		}
		return env.V1.Tx.SourceAccount.Address(), int64(env.V1.Tx.SeqNum), nil

	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		if env.FeeBump == nil {
			return "", 0, fmt.Errorf("missing FeeBump transaction in envelope")
		}
		inner := env.FeeBump.Tx.InnerTx
		switch inner.Type {
		case xdr.EnvelopeTypeEnvelopeTypeTx:
			if inner.V1 == nil {
				return "", 0, fmt.Errorf("missing V1 inner transaction in FeeBump")
			}
			return inner.V1.Tx.SourceAccount.Address(), int64(inner.V1.Tx.SeqNum), nil
		default:
			return "", 0, fmt.Errorf("unsupported inner transaction type: %v", inner.Type)
		}

	default:
		return "", 0, fmt.Errorf("unsupported transaction envelope type: %v", env.Type)
	}
}

func effectiveTxHash(ef *EnvelopeFile) string {
	if ef.Metadata.TxHash != "" {
		return ef.Metadata.TxHash
	}
	if ef.Checksum != "" {
		return ef.Checksum
	}
	return checksumOf(ef.EnvelopeXDR)
}
