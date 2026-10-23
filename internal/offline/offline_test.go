// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package offline

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: generate a fresh ed25519 key pair and return hex-encoded seed.
func generateTestKey(t *testing.T) (seedHex string, pubHex string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return hex.EncodeToString(priv.Seed()), hex.EncodeToString(pub)
}

func TestNewEnvelopeFile(t *testing.T) {
	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA", EnvelopeMetadata{
		Description: "unit test envelope",
		SourceAddr:  "GABCDEF",
	})

	assert.Equal(t, currentFormatVersion, ef.Version)
	assert.Equal(t, "testnet", ef.Network)
	assert.Equal(t, "AAAA", ef.EnvelopeXDR)
	assert.NotEmpty(t, ef.Checksum)
	assert.NotEmpty(t, ef.Metadata.CreatedAt)
	assert.False(t, ef.IsSigned())
}

func TestSaveAndLoadEnvelopeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tx.erst.json")

	original := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{
		Description: "roundtrip test",
	})

	require.NoError(t, original.SaveToFile(path))

	loaded, err := LoadEnvelopeFile(path)
	require.NoError(t, err)

	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.EnvelopeXDR, loaded.EnvelopeXDR)
	assert.Equal(t, original.Checksum, loaded.Checksum)
	assert.Equal(t, original.NetworkPassphrase, loaded.NetworkPassphrase)
}

func TestValidate_EmptyEnvelope(t *testing.T) {
	ef := &EnvelopeFile{
		Version:           currentFormatVersion,
		NetworkPassphrase: "test",
		Checksum:          "abc",
	}
	err := ef.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "envelope_xdr is empty")
}

func TestValidate_EmptyPassphrase(t *testing.T) {
	ef := &EnvelopeFile{
		Version:     currentFormatVersion,
		EnvelopeXDR: "AAAA",
		Checksum:    checksumOf("AAAA"),
	}
	err := ef.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network_passphrase is empty")
}

func TestValidate_ChecksumMismatch(t *testing.T) {
	ef := &EnvelopeFile{
		Version:           currentFormatVersion,
		EnvelopeXDR:       "AAAA",
		NetworkPassphrase: "test",
		Checksum:          "bad",
	}
	err := ef.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestSignAndVerify(t *testing.T) {
	seedHex, _ := generateTestKey(t)

	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})

	require.NoError(t, SignEnvelope(ef, seedHex))
	assert.True(t, ef.IsSigned())
	assert.Len(t, ef.Signatures, 1)
	assert.NotEmpty(t, ef.Signatures[0].SignedAt)

	require.NoError(t, VerifySignatures(ef))
}

func TestSignDuplicate(t *testing.T) {
	seedHex, _ := generateTestKey(t)

	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})
	require.NoError(t, SignEnvelope(ef, seedHex))

	err := SignEnvelope(ef, seedHex)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already signed")
}

func TestSignMultipleKeys(t *testing.T) {
	seed1, _ := generateTestKey(t)
	seed2, _ := generateTestKey(t)

	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})

	require.NoError(t, SignEnvelope(ef, seed1))
	require.NoError(t, SignEnvelope(ef, seed2))
	assert.Len(t, ef.Signatures, 2)

	require.NoError(t, VerifySignatures(ef))
}

func TestVerifyTamperedEnvelope(t *testing.T) {
	seedHex, _ := generateTestKey(t)

	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})
	require.NoError(t, SignEnvelope(ef, seedHex))

	// Tamper with envelope after signing.
	ef.EnvelopeXDR = "BBBB=="
	ef.Checksum = checksumOf("BBBB==") // fix checksum to pass Validate

	err := VerifySignatures(ef)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verification failed")
}

func TestVerifyNoSignatures(t *testing.T) {
	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})
	err := VerifySignatures(ef)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no signatures")
}

func TestSignInvalidKey(t *testing.T) {
	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAA==", EnvelopeMetadata{})

	err := SignEnvelope(ef, "not-hex")
	assert.Error(t, err)

	err = SignEnvelope(ef, hex.EncodeToString([]byte("tooshort")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be 32 or 64 bytes")
}

func TestLoadEnvelopeFile_NotFound(t *testing.T) {
	_, err := LoadEnvelopeFile("/nonexistent/path.json")
	assert.Error(t, err)
}

func TestLoadEnvelopeFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0600))

	_, err := LoadEnvelopeFile(path)
	assert.Error(t, err)
}

func TestRoundTrip_SaveSignLoadVerify(t *testing.T) {
	dir := t.TempDir()
	unsignedPath := filepath.Join(dir, "unsigned.json")
	signedPath := filepath.Join(dir, "signed.json")

	// Step 1: Generate & save unsigned envelope.
	ef := NewEnvelopeFile("testnet", "Test SDF Network ; September 2015", "AAAAAgAAAA==", EnvelopeMetadata{
		Description: "full roundtrip test",
		SourceAddr:  "GABCDEF",
	})
	require.NoError(t, ef.SaveToFile(unsignedPath))

	// Step 2: Load on "airgapped" machine and sign.
	loaded, err := LoadEnvelopeFile(unsignedPath)
	require.NoError(t, err)

	seedHex, _ := generateTestKey(t)
	require.NoError(t, SignEnvelope(loaded, seedHex))

	// Step 3: Save signed envelope.
	require.NoError(t, loaded.SaveToFile(signedPath))

	// Step 4: Load signed envelope back on online machine and verify.
	final, err := LoadEnvelopeFile(signedPath)
	require.NoError(t, err)
	assert.True(t, final.IsSigned())
	require.NoError(t, VerifySignatures(final))
}

func TestSorobanURLForNetwork(t *testing.T) {
	tests := []struct {
		network string
		wantErr bool
	}{
		{"testnet", false},
		{"mainnet", false},
		{"futurenet", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.network, func(t *testing.T) {
			url, err := SorobanURLForNetwork(tt.network)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, url)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, url)
			}
		})
	}
}

func TestParsePrivateKey_FullKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	fullHex := hex.EncodeToString(priv)
	parsedPriv, parsedPub, parseErr := parsePrivateKey(fullHex)
	require.NoError(t, parseErr)
	assert.Equal(t, priv.Public(), ed25519.PublicKey(parsedPub))
	assert.Equal(t, priv, parsedPriv)
}

func TestChecksumOf(t *testing.T) {
	c1 := checksumOf("hello")
	c2 := checksumOf("hello")
	c3 := checksumOf("world")

	assert.Equal(t, c1, c2)
	assert.NotEqual(t, c1, c3)
	assert.Len(t, c1, 64) // SHA-256 hex = 64 chars
}


func makeTestEnvelopeXDR(t *testing.T, pubKey [32]byte, seqNum int64) string {
	t.Helper()
	srcMux, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256(pubKey))
	require.NoError(t, err)

	tx := xdr.Transaction{
		SourceAccount: srcMux,
		Fee:           100,
		SeqNum:        xdr.SequenceNumber(seqNum),
		Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
		Memo:          xdr.Memo{Type: xdr.MemoTypeMemoNone},
		Operations:    []xdr.Operation{},
	}

	env := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx:         tx,
			Signatures: []xdr.DecoratedSignature{},
		},
	}

	raw, err := env.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func makeTestFeeBumpEnvelopeXDR(t *testing.T, innerKey [32]byte, innerSeqNum int64, feeSourceKey [32]byte) string {
	t.Helper()
	innerSrc, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256(innerKey))
	require.NoError(t, err)

	feeSrc, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256(feeSourceKey))
	require.NoError(t, err)

	innerTx := xdr.Transaction{
		SourceAccount: innerSrc,
		Fee:           100,
		SeqNum:        xdr.SequenceNumber(innerSeqNum),
		Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
		Memo:          xdr.Memo{Type: xdr.MemoTypeMemoNone},
		Operations:    []xdr.Operation{},
	}

	env := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &xdr.FeeBumpTransactionEnvelope{
			Tx: xdr.FeeBumpTransaction{
				FeeSource: feeSrc,
				Fee:       200,
				InnerTx: xdr.FeeBumpTransactionInnerTx{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1: &xdr.TransactionV1Envelope{
						Tx:         innerTx,
						Signatures: []xdr.DecoratedSignature{},
					},
				},
			},
			Signatures: []xdr.DecoratedSignature{},
		},
	}

	raw, err := env.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestValidateBatchSequenceNumbers_ValidSequence(t *testing.T) {
	keyA := [32]byte{1, 2, 3, 4}
	keyB := [32]byte{5, 6, 7, 8}

	envA1 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 10),
		Metadata:    EnvelopeMetadata{TxHash: "hash-a-1"},
	}
	envB1 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyB, 100),
		Metadata:    EnvelopeMetadata{TxHash: "hash-b-1"},
	}
	envA2 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 11),
		Metadata:    EnvelopeMetadata{TxHash: "hash-a-2"},
	}
	envB2 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyB, 101),
		Metadata:    EnvelopeMetadata{TxHash: "hash-b-2"},
	}
	envA3 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 12),
		Metadata:    EnvelopeMetadata{TxHash: "hash-a-3"},
	}

	batch := []*EnvelopeFile{envA1, envB1, envA2, envB2, envA3}
	err := ValidateBatchSequenceNumbers(batch)
	assert.NoError(t, err)
}

func TestValidateBatchSequenceNumbers_SingleAndEmpty(t *testing.T) {
	assert.NoError(t, ValidateBatchSequenceNumbers(nil))
	assert.NoError(t, ValidateBatchSequenceNumbers([]*EnvelopeFile{}))

	keyA := [32]byte{1, 2, 3, 4}
	envSingle := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 5),
	}
	assert.NoError(t, ValidateBatchSequenceNumbers([]*EnvelopeFile{envSingle}))
}

func TestValidateBatchSequenceNumbers_OutOfOrder_Gap(t *testing.T) {
	keyA := [32]byte{1, 2, 3, 4}

	env1 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 10),
		Metadata:    EnvelopeMetadata{TxHash: "hash-prev"},
	}
	env2 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 12), // skipped 11!
		Metadata:    EnvelopeMetadata{TxHash: "hash-conflict"},
	}

	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{env1, env2})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))

	var conflict *SequenceOrderConflictError
	require.True(t, errors.As(err, &conflict))
	assert.Equal(t, int64(11), conflict.ExpectedSeq)
	assert.Equal(t, int64(12), conflict.ActualSeq)
	assert.Equal(t, "hash-prev", conflict.PreviousTxHash)
	assert.Equal(t, "hash-conflict", conflict.CurrentTxHash)
	assert.Contains(t, err.Error(), "expected sequence 11, got 12")
	assert.Contains(t, err.Error(), "hash-prev")
	assert.Contains(t, err.Error(), "hash-conflict")
}

func TestValidateBatchSequenceNumbers_OutOfOrder_Duplicate(t *testing.T) {
	keyA := [32]byte{1, 2, 3, 4}

	env1 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 100),
		Metadata:    EnvelopeMetadata{TxHash: "hash-1"},
	}
	env2 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 100), // duplicate sequence!
		Metadata:    EnvelopeMetadata{TxHash: "hash-2"},
	}

	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{env1, env2})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))

	var conflict *SequenceOrderConflictError
	require.True(t, errors.As(err, &conflict))
	assert.Equal(t, int64(101), conflict.ExpectedSeq)
	assert.Equal(t, int64(100), conflict.ActualSeq)
}

func TestValidateBatchSequenceNumbers_OutOfOrder_Decreasing(t *testing.T) {
	keyA := [32]byte{1, 2, 3, 4}

	env1 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 50),
		Metadata:    EnvelopeMetadata{TxHash: "tx-50"},
	}
	env2 := &EnvelopeFile{
		EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 49), // decreasing sequence!
		Metadata:    EnvelopeMetadata{TxHash: "tx-49"},
	}

	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{env1, env2})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbers_FeeBump(t *testing.T) {
	innerKey := [32]byte{9, 9, 9, 9}
	feeKey1 := [32]byte{1, 1, 1, 1}
	feeKey2 := [32]byte{2, 2, 2, 2}

	env1 := &EnvelopeFile{
		EnvelopeXDR: makeTestFeeBumpEnvelopeXDR(t, innerKey, 42, feeKey1),
		Metadata:    EnvelopeMetadata{TxHash: "fb-1"},
	}
	env2 := &EnvelopeFile{
		EnvelopeXDR: makeTestFeeBumpEnvelopeXDR(t, innerKey, 43, feeKey2),
		Metadata:    EnvelopeMetadata{TxHash: "fb-2"},
	}

	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{env1, env2})
	assert.NoError(t, err)

	env3 := &EnvelopeFile{
		EnvelopeXDR: makeTestFeeBumpEnvelopeXDR(t, innerKey, 45, feeKey2), // skipped 44
		Metadata:    EnvelopeMetadata{TxHash: "fb-3"},
	}
	err = ValidateBatchSequenceNumbers([]*EnvelopeFile{env1, env2, env3})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbersXDR(t *testing.T) {
	keyA := [32]byte{1, 2, 3, 4}
	xdr1 := makeTestEnvelopeXDR(t, keyA, 1)
	xdr2 := makeTestEnvelopeXDR(t, keyA, 2)
	xdr3 := makeTestEnvelopeXDR(t, keyA, 3)

	assert.NoError(t, ValidateBatchSequenceNumbersXDR([]string{xdr1, xdr2, xdr3}))

	xdrBad := makeTestEnvelopeXDR(t, keyA, 5) // skipped 4
	err := ValidateBatchSequenceNumbersXDR([]string{xdr1, xdr2, xdrBad})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbers_InvalidInput(t *testing.T) {
	keyA := [32]byte{1, 2, 3, 4}
	valid := &EnvelopeFile{EnvelopeXDR: makeTestEnvelopeXDR(t, keyA, 1)}

	// Nil envelope
	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{valid, nil})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")

	// Empty XDR
	err = ValidateBatchSequenceNumbers([]*EnvelopeFile{valid, {EnvelopeXDR: ""}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty envelope_xdr")

	// Malformed XDR
	err = ValidateBatchSequenceNumbers([]*EnvelopeFile{valid, {EnvelopeXDR: "not-valid-base64-xdr"}})
	assert.Error(t, err)
}
