// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package offline

import (
	"errors"
	"math"
	"testing"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPassphrase = "Test SDF Network ; September 2015"

func testAccountKey(b byte) xdr.Uint256 {
	var k xdr.Uint256
	for i := range k {
		k[i] = b
	}
	return k
}

func testMuxed(b byte) xdr.MuxedAccount {
	k := testAccountKey(b)
	return xdr.MuxedAccount{Type: xdr.CryptoKeyTypeKeyTypeEd25519, Ed25519: &k}
}

func testTx(src xdr.MuxedAccount, seq int64) xdr.Transaction {
	return xdr.Transaction{
		SourceAccount: src,
		Fee:           100,
		SeqNum:        xdr.SequenceNumber(seq),
		Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
	}
}

func envFileFromEnvelope(t *testing.T, env xdr.TransactionEnvelope) *EnvelopeFile {
	t.Helper()
	b64, err := xdr.MarshalBase64(env)
	require.NoError(t, err)
	return NewEnvelopeFile("testnet", testPassphrase, b64, EnvelopeMetadata{})
}

func v1File(t *testing.T, src xdr.MuxedAccount, seq int64) *EnvelopeFile {
	t.Helper()
	return envFileFromEnvelope(t, xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1:   &xdr.TransactionV1Envelope{Tx: testTx(src, seq)},
	})
}

func hashOf(t *testing.T, ef *EnvelopeFile) string {
	t.Helper()
	var env xdr.TransactionEnvelope
	require.NoError(t, xdr.SafeUnmarshalBase64(ef.EnvelopeXDR, &env))
	h, err := network.HashTransactionInEnvelope(env, ef.NetworkPassphrase)
	require.NoError(t, err)
	return hexOf(h[:])
}

func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0xf])
	}
	return string(out)
}

func TestValidateBatchSequenceNumbers_Valid(t *testing.T) {
	a := testMuxed(1)
	batch := []*EnvelopeFile{v1File(t, a, 10), v1File(t, a, 11), v1File(t, a, 12)}
	assert.NoError(t, ValidateBatchSequenceNumbers(batch))
}

func TestValidateBatchSequenceNumbers_EmptyAndSingle(t *testing.T) {
	assert.NoError(t, ValidateBatchSequenceNumbers(nil))
	assert.NoError(t, ValidateBatchSequenceNumbers([]*EnvelopeFile{v1File(t, testMuxed(1), 999)}))
}

func TestValidateBatchSequenceNumbers_GapReportsHashes(t *testing.T) {
	a := testMuxed(1)
	first, second := v1File(t, a, 10), v1File(t, a, 12)

	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{first, second})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))

	var seqErr *SequenceOrderError
	require.True(t, errors.As(err, &seqErr))
	assert.Equal(t, hashOf(t, first), seqErr.PreviousTxHash)
	assert.Equal(t, hashOf(t, second), seqErr.TxHash)
	assert.Equal(t, 0, seqErr.PreviousIndex)
	assert.Equal(t, 1, seqErr.Index)
	assert.Equal(t, int64(10), seqErr.PreviousSeqNum)
	assert.Equal(t, int64(12), seqErr.SeqNum)
	assert.Equal(t, int64(11), seqErr.ExpectedSeqNum)
	assert.Contains(t, err.Error(), seqErr.PreviousTxHash)
	assert.Contains(t, err.Error(), seqErr.TxHash)
}

func TestValidateBatchSequenceNumbers_OutOfOrder(t *testing.T) {
	a := testMuxed(1)
	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{v1File(t, a, 11), v1File(t, a, 10)})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbers_Duplicate(t *testing.T) {
	a := testMuxed(1)
	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{v1File(t, a, 10), v1File(t, a, 10)})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbers_IndependentAccounts(t *testing.T) {
	a, b := testMuxed(1), testMuxed(2)
	// Interleaved accounts; each account's own sequence is contiguous.
	batch := []*EnvelopeFile{
		v1File(t, a, 5), v1File(t, b, 100), v1File(t, a, 6), v1File(t, b, 101),
	}
	assert.NoError(t, ValidateBatchSequenceNumbers(batch))
}

func TestValidateBatchSequenceNumbers_ConflictAcrossInterleaving(t *testing.T) {
	a, b := testMuxed(1), testMuxed(2)
	batch := []*EnvelopeFile{
		v1File(t, a, 5), v1File(t, b, 100), v1File(t, a, 7),
	}
	err := ValidateBatchSequenceNumbers(batch)
	var seqErr *SequenceOrderError
	require.True(t, errors.As(err, &seqErr))
	assert.Equal(t, 0, seqErr.PreviousIndex)
	assert.Equal(t, 2, seqErr.Index)
}

func TestValidateBatchSequenceNumbers_MuxedAccountSharesSequence(t *testing.T) {
	base := testAccountKey(1)
	muxed := xdr.MuxedAccount{
		Type:     xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{Id: 42, Ed25519: base},
	}
	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{
		v1File(t, testMuxed(1), 10), v1File(t, muxed, 12),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbers_FeeBumpUsesInnerTx(t *testing.T) {
	inner := func(seq int64) xdr.TransactionEnvelope {
		return xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1:   &xdr.TransactionV1Envelope{Tx: testTx(testMuxed(1), seq)},
		}
	}
	bump := func(seq int64) *EnvelopeFile {
		in := inner(seq)
		return envFileFromEnvelope(t, xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
			FeeBump: &xdr.FeeBumpTransactionEnvelope{Tx: xdr.FeeBumpTransaction{
				FeeSource: testMuxed(9),
				Fee:       200,
				InnerTx: xdr.FeeBumpTransactionInnerTx{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1:   in.V1,
				},
			}}})
	}

	assert.NoError(t, ValidateBatchSequenceNumbers([]*EnvelopeFile{bump(3), v1File(t, testMuxed(1), 4)}))
	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{bump(3), v1File(t, testMuxed(1), 6)})
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbers_V0Envelope(t *testing.T) {
	v0 := func(seq int64) *EnvelopeFile {
		return envFileFromEnvelope(t, xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTxV0,
			V0: &xdr.TransactionV0Envelope{Tx: xdr.TransactionV0{
				SourceAccountEd25519: testAccountKey(1),
				Fee:                  100,
				SeqNum:               xdr.SequenceNumber(seq),
			}},
		})
	}
	assert.NoError(t, ValidateBatchSequenceNumbers([]*EnvelopeFile{v0(1), v1File(t, testMuxed(1), 2)}))
	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{v0(1), v0(3)})
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbers_MaxSeqNumHasNoSuccessor(t *testing.T) {
	a := testMuxed(1)
	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{
		v1File(t, a, math.MaxInt64), v1File(t, a, math.MinInt64),
	})
	assert.True(t, errors.Is(err, ErrInvalidSequenceOrder))
}

func TestValidateBatchSequenceNumbers_MalformedInput(t *testing.T) {
	bad := NewEnvelopeFile("testnet", testPassphrase, "not-base64-xdr!!", EnvelopeMetadata{})
	err := ValidateBatchSequenceNumbers([]*EnvelopeFile{bad})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidSequenceOrder))

	err = ValidateBatchSequenceNumbers([]*EnvelopeFile{nil})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidSequenceOrder))
}
