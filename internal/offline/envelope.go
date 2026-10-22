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
	stderrors "errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/stellar/go-stellar-sdk/network"
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

// ErrInvalidSequenceOrder is returned (wrapped in a *SequenceOrderError) when
// transactions in a batch from the same source account do not strictly
// increment the account sequence number by one.
var ErrInvalidSequenceOrder = stderrors.New("invalid sequence order")

// SequenceOrderError describes the first pair of transactions in a batch that
// violates the SeqNum + 1 rule for a single source account. It matches
// ErrInvalidSequenceOrder under errors.Is.
type SequenceOrderError struct {
	// Account is the source account address shared by both transactions.
	Account string
	// PreviousIndex and Index are the batch positions of the two transactions.
	PreviousIndex int
	Index         int
	// PreviousTxHash and TxHash are the hex-encoded transaction hashes.
	PreviousTxHash string
	TxHash         string
	// PreviousSeqNum is the sequence number of the earlier transaction.
	PreviousSeqNum int64
	// SeqNum is the sequence number found on the conflicting transaction.
	SeqNum int64
	// ExpectedSeqNum is PreviousSeqNum + 1 (the value SeqNum should have had).
	ExpectedSeqNum int64
}

func (e *SequenceOrderError) Error() string {
	return fmt.Sprintf(
		"%v: account %s: transaction %s (batch index %d, sequence %d) must follow transaction %s (batch index %d, sequence %d) with sequence %d",
		ErrInvalidSequenceOrder, e.Account,
		e.TxHash, e.Index, e.SeqNum,
		e.PreviousTxHash, e.PreviousIndex, e.PreviousSeqNum, e.ExpectedSeqNum,
	)
}

// Unwrap lets callers match the sentinel with errors.Is(err, ErrInvalidSequenceOrder).
func (e *SequenceOrderError) Unwrap() error { return ErrInvalidSequenceOrder }

// ValidateBatchSequenceNumbers checks, before a batch is dispatched to the
// simulator, that consecutive transactions from the same source account use
// strictly incrementing sequence numbers (previous + 1).
//
// The first transaction seen for an account is accepted as-is, because the
// account's on-ledger sequence number is not known offline. For fee-bump
// envelopes the inner transaction's source account and sequence number are
// used, as those are what the network consumes. Muxed accounts are resolved to
// their underlying account.
//
// On the first violation a *SequenceOrderError (matching ErrInvalidSequenceOrder)
// is returned carrying both conflicting transaction hashes. Malformed input is
// reported as a validation error.
func ValidateBatchSequenceNumbers(envelopes []*EnvelopeFile) error {
	type lastSeen struct {
		seq   int64
		hash  string
		index int
	}
	last := make(map[string]lastSeen, len(envelopes))

	for i, ef := range envelopes {
		if ef == nil {
			return errors.WrapValidationError(fmt.Sprintf("batch envelope %d is nil", i))
		}

		var env xdr.TransactionEnvelope
		if err := xdr.SafeUnmarshalBase64(ef.EnvelopeXDR, &env); err != nil {
			return errors.WrapValidationError(
				fmt.Sprintf("batch envelope %d: invalid envelope XDR: %v", i, err),
			)
		}

		switch env.Type {
		case xdr.EnvelopeTypeEnvelopeTypeTx,
			xdr.EnvelopeTypeEnvelopeTypeTxV0,
			xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		default:
			return errors.WrapValidationError(
				fmt.Sprintf("batch envelope %d: unsupported envelope type %s", i, env.Type),
			)
		}

		hashBytes, err := network.HashTransactionInEnvelope(env, ef.NetworkPassphrase)
		if err != nil {
			return errors.WrapValidationError(
				fmt.Sprintf("batch envelope %d: failed to hash transaction: %v", i, err),
			)
		}
		txHash := hex.EncodeToString(hashBytes[:])

		src := env.SourceAccount()
		account := src.ToAccountId().Address()
		seq := env.SeqNum()

		if prev, ok := last[account]; ok {
			if prev.seq == math.MaxInt64 || seq != prev.seq+1 {
				expected := prev.seq
				if prev.seq != math.MaxInt64 {
					expected = prev.seq + 1
				}
				return &SequenceOrderError{
					Account:        account,
					PreviousIndex:  prev.index,
					Index:          i,
					PreviousTxHash: prev.hash,
					TxHash:         txHash,
					PreviousSeqNum: prev.seq,
					SeqNum:         seq,
					ExpectedSeqNum: expected,
				}
			}
		}
		last[account] = lastSeen{seq: seq, hash: txHash, index: i}
	}

	return nil
}

// checksumOf returns the hex-encoded SHA-256 digest of s.
func checksumOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
