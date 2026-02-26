// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// SumNativePaymentsStroops returns the sum of all native (XLM) payment amounts
// in the transaction envelope, in stroops. Returns 0 for empty envelope or
// non-Soroban envelope types (e.g. TxV0). Used to enforce sandbox token limits.
func SumNativePaymentsStroops(envelopeXdr string) (uint64, error) {
	if envelopeXdr == "" {
		return 0, nil
	}
	raw, err := base64.StdEncoding.DecodeString(envelopeXdr)
	if err != nil {
		return 0, fmt.Errorf("decode envelope base64: %w", err)
	}
	var env xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshal(raw, &env); err != nil {
		return 0, fmt.Errorf("unmarshal envelope: %w", err)
	}
	var tx xdr.Transaction
	switch env.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		tx = env.MustV1().Tx
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		inner := env.MustFeeBump().Tx.InnerTx
		if inner.Type != xdr.EnvelopeTypeEnvelopeTypeTx {
			return 0, nil
		}
		tx = inner.MustV1().Tx
	default:
		return 0, nil
	}
	var total uint64
	for _, op := range tx.Operations {
		if op.Body.Type != xdr.OperationTypePayment {
			continue
		}
		p := op.Body.MustPaymentOp()
		if p.Asset.Type != xdr.AssetTypeAssetTypeNative {
			continue
		}
		amt := p.Amount
		if amt < 0 {
			continue
		}
		total += uint64(amt)
	}
	return total, nil
}

// ErrSandboxNativeTokenCapExceeded is returned when the envelope's total native
// payment amount exceeds the configured sandbox cap.
var ErrSandboxNativeTokenCapExceeded = fmt.Errorf("sandbox native token cap exceeded")

// EnforceSandboxNativeTokenCap returns an error if the envelope's sum of native
// payments (stroops) exceeds capStroops. Use when running in sandbox/local mode
// to simulate realistic economic constraints.
func EnforceSandboxNativeTokenCap(envelopeXdr string, capStroops uint64) error {
	sum, err := SumNativePaymentsStroops(envelopeXdr)
	if err != nil {
		return err
	}
	if sum > capStroops {
		return fmt.Errorf("%w: total native payments %d stroops exceeds cap %d stroops", ErrSandboxNativeTokenCapExceeded, sum, capStroops)
	}
	return nil
}
