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

package tokenflow

import (
	"encoding/base64"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/stellar/go/strkey"
	"github.com/stellar/go/xdr"
)

// Kind is the semantic meaning of a movement.
type Kind string

const (
	KindTransfer Kind = "transfer"
	KindMint     Kind = "mint"
)

// Token identifies an asset.
// - XLM: Symbol="XLM", ID=""
// - SAC: Symbol="SAC" (best-effort), ID="C...." (contract id)
type Token struct {
	Symbol string
	ID     string
}

func (t Token) Display() string {
	if t.Symbol == "XLM" && t.ID == "" {
		return "XLM"
	}
	if t.ID == "" {
		if t.Symbol == "" {
			return "TOKEN"
		}
		return t.Symbol
	}
	id := t.ID
	if len(id) > 12 {
		id = id[:12] + "…"
	}
	if t.Symbol == "" {
		return "SAC(" + id + ")"
	}
	return t.Symbol + "(" + id + ")"
}

// Transfer is a single token movement (or mint).
type Transfer struct {
	From   string
	To     string
	Token  Token
	Amount *big.Int // integer smallest units (XLM: stroops)
	Kind   Kind
}

// Report is the aggregated “money flow” view.
type Report struct {
	Raw []Transfer
	Agg []Transfer
}

// BuildReport extracts transfers/mints from:
// - native XLM payments in EnvelopeXdr
// - Soroban SAC transfer/mint events from ResultMetaXdr diagnostic events
func BuildReport(envelopeXdrB64, resultMetaXdrB64 string) (*Report, error) {
	var raw []Transfer

	if envelopeXdrB64 != "" {
		xlm, err := extractNativeXLMPayments(envelopeXdrB64)
		if err != nil {
			return nil, err
		}
		raw = append(raw, xlm...)
	}

	if resultMetaXdrB64 != "" {
		sac, err := extractSACTransfersAndMints(resultMetaXdrB64)
		if err != nil {
			return nil, err
		}
		raw = append(raw, sac...)
	}

	return &Report{
		Raw: raw,
		Agg: aggregate(raw),
	}, nil
}

func extractNativeXLMPayments(envelopeXdrB64 string) ([]Transfer, error) {
	envBytes, err := base64.StdEncoding.DecodeString(envelopeXdrB64)
	if err != nil {
		return nil, fmt.Errorf("decode envelope xdr base64: %w", err)
	}

	var env xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshal(envBytes, &env); err != nil {
		return nil, fmt.Errorf("unmarshal TransactionEnvelope: %w", err)
	}

	var tx xdr.Transaction
	switch env.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		tx = env.MustV1().Tx
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		inner := env.MustFeeBump().Tx.InnerTx
		if inner.Type != xdr.EnvelopeTypeEnvelopeTypeTx {
			return nil, fmt.Errorf("unsupported inner tx type: %s", inner.Type)
		}
		tx = inner.MustV1().Tx
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		// Rare; skip for now.
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported envelope type: %s", env.Type)
	}

	source, err := muxedAccountToAddress(tx.SourceAccount)
	if err != nil {
		return nil, err
	}

	var transfers []Transfer
	for _, op := range tx.Operations {
		opSource := source
		if op.SourceAccount != nil {
			if s, err := muxedAccountToAddress(*op.SourceAccount); err == nil {
				opSource = s
			}
		}

		if op.Body.Type != xdr.OperationTypePayment {
			continue
		}

		p := op.Body.MustPaymentOp()
		if p.Asset.Type != xdr.AssetTypeAssetTypeNative {
			continue
		}

		to, err := muxedAccountToAddress(p.Destination)
		if err != nil {
			return nil, err
		}

		amt := new(big.Int).SetInt64(int64(p.Amount))
		transfers = append(transfers, Transfer{
			From:   opSource,
			To:     to,
			Token:  Token{Symbol: "XLM"},
			Amount: amt,
			Kind:   KindTransfer,
		})
	}

	return transfers, nil
}

func extractSACTransfersAndMints(resultMetaXdrB64 string) ([]Transfer, error) {
	metaBytes, err := base64.StdEncoding.DecodeString(resultMetaXdrB64)
	if err != nil {
		return nil, fmt.Errorf("decode result_meta xdr base64: %w", err)
	}

	var rm xdr.TransactionResultMeta
	if err := xdr.SafeUnmarshal(metaBytes, &rm); err != nil {
		return nil, fmt.Errorf("unmarshal TransactionResultMeta: %w", err)
	}

	diag := extractDiagnosticEvents(rm.TxApplyProcessing)
	var out []Transfer

	for _, de := range diag {
		// Avoid counting reverted calls.
		if !de.InSuccessfulContractCall {
			continue
		}

		ce := de.Event
		if ce.ContractId == nil {
			continue
		}

		contractStr, err := strkey.Encode(strkey.VersionByteContract, ce.ContractId[:])
		if err != nil {
			continue
		}

		body, ok := ce.Body.GetV0()
		if !ok || len(body.Topics) == 0 {
			continue
		}

		op, ok := scValSymbol(body.Topics[0])
		if !ok {
			continue
		}

		switch op {
		case "transfer":
			// Expected topics: ["transfer", from, to], data: amount
			if len(body.Topics) < 3 {
				continue
			}
			from, ok := scValAddressString(body.Topics[1])
			if !ok {
				continue
			}
			to, ok := scValAddressString(body.Topics[2])
			if !ok {
				continue
			}
			amt, ok := scValAmount(body.Data)
			if !ok || amt.Sign() < 0 {
				continue
			}
			out = append(out, Transfer{
				From:   from,
				To:     to,
				Token:  Token{Symbol: "SAC", ID: contractStr},
				Amount: amt,
				Kind:   KindTransfer,
			})
		case "mint":
			// Expected topics: ["mint", to], data: amount
			if len(body.Topics) < 2 {
				continue
			}
			to, ok := scValAddressString(body.Topics[1])
			if !ok {
				continue
			}
			amt, ok := scValAmount(body.Data)
			if !ok || amt.Sign() < 0 {
				continue
			}
			out = append(out, Transfer{
				From:   "MINT",
				To:     to,
				Token:  Token{Symbol: "SAC", ID: contractStr},
				Amount: amt,
				Kind:   KindMint,
			})
		}
	}

	return out, nil
}

func extractDiagnosticEvents(tm xdr.TransactionMeta) []xdr.DiagnosticEvent {
	switch tm.V {
	case 3:
		if tm.V3 != nil && tm.V3.SorobanMeta != nil {
			return tm.V3.SorobanMeta.DiagnosticEvents
		}
	case 4:
		if tm.V4 != nil && len(tm.V4.DiagnosticEvents) > 0 {
			return tm.V4.DiagnosticEvents
		}
	}
	return nil
}

func scValSymbol(v xdr.ScVal) (string, bool) {
	if v.Type != xdr.ScValTypeScvSymbol || v.Sym == nil {
		return "", false
	}
	return string(*v.Sym), true
}

func scValAddressString(v xdr.ScVal) (string, bool) {
	if v.Type != xdr.ScValTypeScvAddress || v.Address == nil {
		return "", false
	}
	s, err := v.Address.String()
	if err != nil {
		return "", false
	}
	return s, true
}

func scValAmount(v xdr.ScVal) (*big.Int, bool) {
	switch v.Type {
	case xdr.ScValTypeScvU64:
		if v.U64 == nil {
			return nil, false
		}
		return new(big.Int).SetUint64(uint64(*v.U64)), true
	case xdr.ScValTypeScvI64:
		if v.I64 == nil {
			return nil, false
		}
		return new(big.Int).SetInt64(int64(*v.I64)), true
	case xdr.ScValTypeScvU128:
		if v.U128 == nil {
			return nil, false
		}
		return bigIntFromU128(*v.U128), true
	case xdr.ScValTypeScvI128:
		if v.I128 == nil {
			return nil, false
		}
		return bigIntFromI128(*v.I128), true
	default:
		return nil, false
	}
}

func bigIntFromU128(p xdr.UInt128Parts) *big.Int {
	hi := new(big.Int).SetUint64(uint64(p.Hi))
	lo := new(big.Int).SetUint64(uint64(p.Lo))
	hi.Lsh(hi, 64)
	return hi.Or(hi, lo)
}

func bigIntFromI128(p xdr.Int128Parts) *big.Int {
	hi := new(big.Int).SetInt64(int64(p.Hi))
	lo := new(big.Int).SetUint64(uint64(p.Lo))
	hi.Lsh(hi, 64)
	return hi.Or(hi, lo)
}

func muxedAccountToAddress(a xdr.MuxedAccount) (string, error) {
	ma := a
	return (&ma).GetAddress()
}

func aggregate(in []Transfer) []Transfer {
	type key struct {
		from string
		to   string
		kind Kind
		sym  string
		id   string
	}

	m := map[key]*big.Int{}
	for _, t := range in {
		k := key{from: t.From, to: t.To, kind: t.Kind, sym: t.Token.Symbol, id: t.Token.ID}
		if m[k] == nil {
			m[k] = new(big.Int)
		}
		m[k].Add(m[k], t.Amount)
	}

	var out []Transfer
	for k, v := range m {
		out = append(out, Transfer{
			From:   k.from,
			To:     k.to,
			Kind:   k.kind,
			Token:  Token{Symbol: k.sym, ID: k.id},
			Amount: new(big.Int).Set(v),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		ai := strings.Join([]string{out[i].Token.Symbol, out[i].Token.ID, out[i].From, out[i].To, string(out[i].Kind)}, "|")
		aj := strings.Join([]string{out[j].Token.Symbol, out[j].Token.ID, out[j].From, out[j].To, string(out[j].Kind)}, "|")
		return ai < aj
	})

	return out
}
