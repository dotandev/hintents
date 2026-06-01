// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package tokenflow

import (
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/require"
)

func TestBuildReport_SACTransferAndMint_FromResultMeta(t *testing.T) {
	contract := bytes32(0xAA)
	cid := xdr.ContractId(contract)
	contractStr, err := strkey.Encode(strkey.VersionByteContract, cid[:])
	require.NoError(t, err)

	fromAddr := scAddressAccount(t, bytes32(0x01))
	toAddr := scAddressAccount(t, bytes32(0x02))

	transferEvent := diagnosticEvent(
		cid,
		[]xdr.ScVal{
			scSymbol("transfer"),
			scAddress(fromAddr),
			scAddress(toAddr),
		},
		scU128(50),
		true,
	)

	mintTo := scAddressAccount(t, bytes32(0x03))
	mintEvent := diagnosticEvent(
		cid,
		[]xdr.ScVal{
			scSymbol("mint"),
			scAddress(mintTo),
		},
		scU64(7),
		true,
	)

	rmB64 := encodeResultMetaWithDiagnosticEvents(t, []xdr.DiagnosticEvent{transferEvent, mintEvent})

	r, err := BuildReport("", rmB64)
	require.NoError(t, err)
	require.Len(t, r.Agg, 2)

	require.Equal(t, KindTransfer, r.Agg[0].Kind)
	require.Equal(t, contractStr, r.Agg[0].Token.ID)
	require.Equal(t, "SAC", r.Agg[0].Token.Symbol)
	require.Equal(t, addrString(t, fromAddr), r.Agg[0].From)
	require.Equal(t, addrString(t, toAddr), r.Agg[0].To)
	require.Equal(t, big.NewInt(50), r.Agg[0].Amount)

	require.Equal(t, KindMint, r.Agg[1].Kind)
	require.Equal(t, contractStr, r.Agg[1].Token.ID)
	require.Equal(t, "MINT", r.Agg[1].From)
	require.Equal(t, addrString(t, mintTo), r.Agg[1].To)
	require.Equal(t, big.NewInt(7), r.Agg[1].Amount)
}

func TestBuildReport_NativeXLMPayment_FromEnvelope(t *testing.T) {
	src := bytes32(0x10)
	dst := bytes32(0x20)

	envB64 := encodeEnvelopeWithNativePayment(t, src, dst, 12_345_678) // 1.2345678 XLM
	r, err := BuildReport(envB64, "")
	require.NoError(t, err)
	require.Len(t, r.Agg, 1)

	tr := r.Agg[0]
	require.Equal(t, KindTransfer, tr.Kind)
	require.Equal(t, "XLM", tr.Token.Symbol)
	require.Equal(t, "", tr.Token.ID)
	require.Equal(t, addrMuxed(t, src), tr.From)
	require.Equal(t, addrMuxed(t, dst), tr.To)
	require.Equal(t, big.NewInt(12_345_678), tr.Amount)
}

func encodeResultMetaWithDiagnosticEvents(t *testing.T, events []xdr.DiagnosticEvent) string {
	t.Helper()

	stm := xdr.SorobanTransactionMeta{
		Ext:              xdr.SorobanTransactionMetaExt{V: 0},
		Events:           nil,
		ReturnValue:      xdr.ScVal{Type: xdr.ScValTypeScvVoid},
		DiagnosticEvents: events,
	}

	tm3 := xdr.TransactionMetaV3{
		Ext:             xdr.ExtensionPoint{V: 0},
		TxChangesBefore: xdr.LedgerEntryChanges{},
		Operations:      nil,
		TxChangesAfter:  xdr.LedgerEntryChanges{},
		SorobanMeta:     &stm,
	}

	tm := xdr.TransactionMeta{V: 3, V3: &tm3}

	emptyOpResults := []xdr.OperationResult{}
	txResResult := xdr.TransactionResultResult{
		Code:    xdr.TransactionResultCodeTxSuccess,
		Results: &emptyOpResults,
	}
	txRes := xdr.TransactionResult{
		FeeCharged: xdr.Int64(0),
		Result:     txResResult,
		Ext:        xdr.TransactionResultExt{V: 0},
	}

	rm := xdr.TransactionResultMeta{
		Result:            xdr.TransactionResultPair{TransactionHash: xdr.Hash{}, Result: txRes},
		FeeProcessing:     xdr.LedgerEntryChanges{},
		TxApplyProcessing: tm,
	}

	b, err := rm.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(b)
}

func encodeEnvelopeWithNativePayment(t *testing.T, src [32]byte, dst [32]byte, stroops int64) string {
	t.Helper()

	srcMux, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256(src))
	if err != nil {
		t.Fatalf("new muxed source account: %v", err)
	}
	dstMux, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256(dst))
	if err != nil {
		t.Fatalf("new muxed destination account: %v", err)
	}

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
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func diagnosticEvent(contractID xdr.ContractId, topics []xdr.ScVal, data xdr.ScVal, successful bool) xdr.DiagnosticEvent {
	cid := contractID
	ce := xdr.ContractEvent{
		Ext:        xdr.ExtensionPoint{V: 0},
		ContractId: &cid,
		Type:       xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Topics: topics,
				Data:   data,
			},
		},
	}
	return xdr.DiagnosticEvent{InSuccessfulContractCall: successful, Event: ce}
}

func scSymbol(s string) xdr.ScVal {
	sym := xdr.ScSymbol(s)
	return xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym}
}

func scAddress(a xdr.ScAddress) xdr.ScVal {
	return xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &a}
}

func scU64(v uint64) xdr.ScVal {
	u := xdr.Uint64(v)
	return xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: &u}
}

func scU128(v uint64) xdr.ScVal {
	parts := xdr.UInt128Parts{Hi: xdr.Uint64(0), Lo: xdr.Uint64(v)}
	return xdr.ScVal{Type: xdr.ScValTypeScvU128, U128: &parts}
}

func scAddressAccount(t *testing.T, pk [32]byte) xdr.ScAddress {
	t.Helper()

	acc, err := xdr.NewAccountId(xdr.PublicKeyTypePublicKeyTypeEd25519, xdr.Uint256(pk))
	if err != nil {
		t.Fatalf("new account id: %v", err)
	}
	a := xdr.AccountId(acc)
	return xdr.ScAddress{
		Type:      xdr.ScAddressTypeScAddressTypeAccount,
		AccountId: &a,
	}
}

func bytes32(fill byte) [32]byte {
	var b [32]byte
	for i := 0; i < 32; i++ {
		b[i] = fill
	}
	return b
}

func addrString(t *testing.T, a xdr.ScAddress) string {
	t.Helper()

	s, err := a.String()
	if err != nil {
		t.Fatalf("address string: %v", err)
	}
	return s
}

func addrMuxed(t *testing.T, pk [32]byte) string {
	t.Helper()

	m, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, xdr.Uint256(pk))
	if err != nil {
		t.Fatalf("new muxed account: %v", err)
	}
	ma := xdr.MuxedAccount(m)
	s, err := (&ma).GetAddress()
	if err != nil {
		t.Fatalf("muxed account address: %v", err)
	}
	return s
}
