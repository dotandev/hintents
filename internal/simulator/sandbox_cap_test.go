// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeEnvelopeWithNativePayment(t *testing.T, stroops int64) string {
	t.Helper()
	var src, dst [32]byte
	src[0], dst[0] = 1, 2
	srcMux, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256(src))
	require.NoError(t, err)
	dstMux, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256(dst))
	require.NoError(t, err)

	payment := xdr.PaymentOp{
		Destination: xdr.MuxedAccount(dstMux),
		Asset:       xdr.Asset{Type: xdr.AssetTypeAssetTypeNative},
		Amount:      xdr.Int64(stroops),
	}

	op := xdr.Operation{
		SourceAccount: nil,
		Body:          xdr.OperationBody{Type: xdr.OperationTypePayment, PaymentOp: &payment},
	}

	tx := xdr.Transaction{
		SourceAccount: xdr.MuxedAccount(srcMux),
		Fee:           xdr.Uint32(100),
		SeqNum:        xdr.SequenceNumber(1),
		Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
		Memo:          xdr.Memo{Type: xdr.MemoTypeMemoNone},
		Operations:    []xdr.Operation{op},
		Ext:           xdr.TransactionExt{V: 0},
	}

	env := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx:         tx,
			Signatures: []xdr.DecoratedSignature{},
		},
	}

	b, err := env.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(b)
}

func TestSumNativePaymentsStroops(t *testing.T) {
	t.Run("empty envelope returns 0", func(t *testing.T) {
		sum, err := SumNativePaymentsStroops("")
		require.NoError(t, err)
		assert.Equal(t, uint64(0), sum)
	})

	t.Run("single native payment", func(t *testing.T) {
		env := encodeEnvelopeWithNativePayment(t, 10_000_000) // 1 XLM
		sum, err := SumNativePaymentsStroops(env)
		require.NoError(t, err)
		assert.Equal(t, uint64(10_000_000), sum)
	})

	t.Run("invalid base64 returns error", func(t *testing.T) {
		_, err := SumNativePaymentsStroops("not-base64!!!")
		require.Error(t, err)
	})
}

func TestEnforceSandboxNativeTokenCap(t *testing.T) {
	env := encodeEnvelopeWithNativePayment(t, 10_000_000)

	t.Run("under cap passes", func(t *testing.T) {
		err := EnforceSandboxNativeTokenCap(env, 20_000_000)
		require.NoError(t, err)
	})

	t.Run("at cap passes", func(t *testing.T) {
		err := EnforceSandboxNativeTokenCap(env, 10_000_000)
		require.NoError(t, err)
	})

	t.Run("over cap returns error", func(t *testing.T) {
		err := EnforceSandboxNativeTokenCap(env, 5_000_000)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSandboxNativeTokenCapExceeded)
	})

	t.Run("empty envelope under any cap", func(t *testing.T) {
		err := EnforceSandboxNativeTokenCap("", 1)
		require.NoError(t, err)
	})
}
