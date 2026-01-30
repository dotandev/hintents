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

package decoder

import (
	"fmt"

	"github.com/stellar/go/xdr"
)

type DecodedEnvelope struct {
	Type       string
	Source     string
	Fee        int64
	Operations []xdr.Operation
	InnerTx    *DecodedEnvelope // for FeeBump
}

func AnalyzeEnvelope(b64 string) (*DecodedEnvelope, error) {
	var env xdr.TransactionEnvelope

	if err := xdr.SafeUnmarshalBase64(b64, &env); err != nil {
		return nil, err
	}

	switch env.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		return decodeV0(env.V0.Tx)

	case xdr.EnvelopeTypeEnvelopeTypeTx:
		return decodeV1(env.V1.Tx)

	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		return decodeFeeBump(env.FeeBump.Tx)

	default:
		return nil, fmt.Errorf("unsupported envelope type: %s", env.Type)
	}
}

func decodeV0(tx xdr.TransactionV0) (*DecodedEnvelope, error) {
	source := xdr.AccountId{
		Type:    xdr.PublicKeyTypePublicKeyTypeEd25519,
		Ed25519: &tx.SourceAccountEd25519,
	}
	return &DecodedEnvelope{
		Type:       "TransactionV0",
		Source:     source.Address(),
		Fee:        int64(tx.Fee),
		Operations: tx.Operations,
	}, nil
}
func decodeV1(tx xdr.Transaction) (*DecodedEnvelope, error) {
	return &DecodedEnvelope{
		Type:       "TransactionV1",
		Source:     tx.SourceAccount.Address(),
		Fee:        int64(tx.Fee),
		Operations: tx.Operations,
	}, nil
}

func decodeFeeBump(fb xdr.FeeBumpTransaction) (*DecodedEnvelope, error) {
	inner, err := DecodeEnvelopeFromInner(fb.InnerTx)
	if err != nil {
		return nil, err
	}

	return &DecodedEnvelope{
		Type:    "FeeBumpTransaction",
		Source:  fb.FeeSource.Address(),
		Fee:     int64(fb.Fee),
		InnerTx: inner,
	}, nil
}

func DecodeEnvelopeFromInner(inner xdr.FeeBumpTransactionInnerTx) (*DecodedEnvelope, error) {
	switch inner.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		return decodeV1(inner.V1.Tx)
	default:
		return nil, fmt.Errorf("unsupported inner tx type")
	}
}
