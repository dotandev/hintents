// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"encoding/base64"
	"testing"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAccountID(b byte) xdr.AccountId {
	var k xdr.Uint256
	for i := range k {
		k[i] = b
	}
	return xdr.AccountId{Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: &k}
}

func b64ScVal(t *testing.T, v xdr.ScVal) string {
	t.Helper()
	raw, err := v.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func symTopic(t *testing.T, s string) string {
	sym := xdr.ScSymbol(s)
	return b64ScVal(t, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym})
}

func strTopic(t *testing.T, s string) string {
	str := xdr.ScString(s)
	return b64ScVal(t, xdr.ScVal{Type: xdr.ScValTypeScvString, Str: &str})
}

func accountTopic(t *testing.T, id xdr.AccountId) string {
	addr := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeAccount, AccountId: &id}
	return b64ScVal(t, xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &addr})
}

func contractTopic(t *testing.T) string {
	var h xdr.ContractId
	h[0] = 7
	addr := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &h}
	return b64ScVal(t, xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &addr})
}

func amountData(t *testing.T, n int64) string {
	v := xdr.ScVal{Type: xdr.ScValTypeScvI128, I128: &xdr.Int128Parts{Hi: 0, Lo: xdr.Uint64(n)}}
	return b64ScVal(t, v)
}

// testAsset returns the canonical "CODE:ISSUER" string and the xdr asset.
func testAsset(t *testing.T, issuer xdr.AccountId) (string, xdr.TrustLineAsset) {
	var code xdr.AssetCode4
	copy(code[:], "USDC")
	asset := xdr.Asset{
		Type:      xdr.AssetTypeAssetTypeCreditAlphanum4,
		AlphaNum4: &xdr.AlphaNum4{AssetCode: code, Issuer: issuer},
	}
	return asset.StringCanonical(), asset.ToTrustLineAsset()
}

func trustlineEntry(t *testing.T, account xdr.AccountId, line xdr.TrustLineAsset, flags xdr.TrustLineFlags) string {
	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 1,
		Data: xdr.LedgerEntryData{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.TrustLineEntry{
				AccountId: account,
				Asset:     line,
				Balance:   100,
				Limit:     1000,
				Flags:     xdr.Uint32(flags),
			},
		},
	}
	s, err := xdr.MarshalBase64(entry)
	require.NoError(t, err)
	return s
}

func transferEvent(t *testing.T, from, to xdr.AccountId, asset string, amount int64) simulator.CategorizedEvent {
	cid := "CSAC"
	return simulator.CategorizedEvent{
		Category:   "contract",
		EventType:  "contract",
		ContractID: &cid,
		Topics: []string{
			symTopic(t, "transfer"), accountTopic(t, from), accountTopic(t, to), strTopic(t, asset),
		},
		Data: amountData(t, amount),
	}
}

func TestAssetSafety_AuthorizedTrustlinePasses(t *testing.T) {
	issuer, from, to := testAccountID(1), testAccountID(2), testAccountID(3)
	asset, line := testAsset(t, issuer)
	entries := map[string]string{
		"k1": trustlineEntry(t, from, line, xdr.TrustLineFlagsAuthorizedFlag),
		"k2": trustlineEntry(t, to, line, xdr.TrustLineFlagsAuthorizedFlag),
	}
	resp := &simulator.SimulationResponse{Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{transferEvent(t, from, to, asset, 50)}}

	assert.Empty(t, NewAssetSafetyAnalyzer(entries).Analyze(resp))
}

func TestAssetSafety_MissingRecipientTrustline(t *testing.T) {
	issuer, from, to := testAccountID(1), testAccountID(2), testAccountID(3)
	asset, line := testAsset(t, issuer)
	entries := map[string]string{"k1": trustlineEntry(t, from, line, xdr.TrustLineFlagsAuthorizedFlag)}
	resp := &simulator.SimulationResponse{Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{transferEvent(t, from, to, asset, 50)}}

	w := NewAssetSafetyAnalyzer(entries).Analyze(resp)
	require.Len(t, w, 1)
	assert.Equal(t, WarningMissingTrustline, w[0].Type)
	assert.Equal(t, to.Address(), w[0].Account)
	assert.Equal(t, "recipient", w[0].Role)
	assert.Equal(t, asset, w[0].Asset)
	assert.Equal(t, "transfer", w[0].Operation)
	assert.Equal(t, "50", w[0].Amount)
	assert.Equal(t, "CSAC", w[0].ContractID)
	assert.Equal(t, "event_index:0", w[0].Location)
}

func TestAssetSafety_UnauthorizedTrustline(t *testing.T) {
	issuer, from, to := testAccountID(1), testAccountID(2), testAccountID(3)
	asset, line := testAsset(t, issuer)
	entries := map[string]string{
		"k1": trustlineEntry(t, from, line, xdr.TrustLineFlagsAuthorizedFlag),
		"k2": trustlineEntry(t, to, line, 0),
	}
	resp := &simulator.SimulationResponse{Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{transferEvent(t, from, to, asset, 5)}}

	w := NewAssetSafetyAnalyzer(entries).Analyze(resp)
	require.Len(t, w, 1)
	assert.Equal(t, WarningTrustlineNotAuthorized, w[0].Type)
	assert.Equal(t, to.Address(), w[0].Account)
}

func TestAssetSafety_MaintainLiabilitiesOnly(t *testing.T) {
	issuer, from, to := testAccountID(1), testAccountID(2), testAccountID(3)
	asset, line := testAsset(t, issuer)
	entries := map[string]string{
		"k1": trustlineEntry(t, from, line, xdr.TrustLineFlagsAuthorizedFlag),
		"k2": trustlineEntry(t, to, line, xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag),
	}
	resp := &simulator.SimulationResponse{Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{transferEvent(t, from, to, asset, 5)}}

	w := NewAssetSafetyAnalyzer(entries).Analyze(resp)
	require.Len(t, w, 1)
	assert.Equal(t, WarningTrustlineMaintainLiabilitiesOnly, w[0].Type)
}

func TestAssetSafety_SenderIsChecked(t *testing.T) {
	issuer, from, to := testAccountID(1), testAccountID(2), testAccountID(3)
	asset, line := testAsset(t, issuer)
	entries := map[string]string{
		"k1": trustlineEntry(t, from, line, 0),
		"k2": trustlineEntry(t, to, line, xdr.TrustLineFlagsAuthorizedFlag),
	}
	resp := &simulator.SimulationResponse{Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{transferEvent(t, from, to, asset, 5)}}

	w := NewAssetSafetyAnalyzer(entries).Analyze(resp)
	require.Len(t, w, 1)
	assert.Equal(t, "sender", w[0].Role)
	assert.Equal(t, from.Address(), w[0].Account)
}

func TestAssetSafety_MintAndBurn(t *testing.T) {
	issuer, acct := testAccountID(1), testAccountID(2)
	asset, line := testAsset(t, issuer)
	// A trustline for another account makes trustline data available.
	entries := map[string]string{"k": trustlineEntry(t, testAccountID(9), line, xdr.TrustLineFlagsAuthorizedFlag)}

	mint := simulator.CategorizedEvent{EventType: "contract",
		Topics: []string{symTopic(t, "mint"), accountTopic(t, acct), strTopic(t, asset)}, Data: amountData(t, 10)}
	burn := simulator.CategorizedEvent{EventType: "contract",
		Topics: []string{symTopic(t, "burn"), accountTopic(t, acct), strTopic(t, asset)}, Data: amountData(t, 4)}
	resp := &simulator.SimulationResponse{Status: "success", CategorizedEvents: []simulator.CategorizedEvent{mint, burn}}

	w := NewAssetSafetyAnalyzer(entries).Analyze(resp)
	require.Len(t, w, 2)
	assert.Equal(t, "mint", w[0].Operation)
	assert.Equal(t, "recipient", w[0].Role)
	assert.Equal(t, "burn", w[1].Operation)
	assert.Equal(t, "sender", w[1].Role)
}

func TestAssetSafety_IssuerContractsAndNativeAreExempt(t *testing.T) {
	issuer, acct := testAccountID(1), testAccountID(2)
	asset, line := testAsset(t, issuer)
	entries := map[string]string{"k": trustlineEntry(t, acct, line, xdr.TrustLineFlagsAuthorizedFlag)}

	// Issuer as sender (classic mint-like transfer) and a contract recipient.
	toContract := simulator.CategorizedEvent{EventType: "contract",
		Topics: []string{symTopic(t, "transfer"), accountTopic(t, issuer), contractTopic(t), strTopic(t, asset)},
		Data:   amountData(t, 1)}
	// Native asset never needs a trustline.
	native := simulator.CategorizedEvent{EventType: "contract",
		Topics: []string{symTopic(t, "transfer"), accountTopic(t, acct), accountTopic(t, testAccountID(5)), strTopic(t, "native")},
		Data:   amountData(t, 1)}
	resp := &simulator.SimulationResponse{Status: "success", CategorizedEvents: []simulator.CategorizedEvent{toContract, native}}

	assert.Empty(t, NewAssetSafetyAnalyzer(entries).Analyze(resp))
}

func TestAssetSafety_NonSACEventsIgnored(t *testing.T) {
	issuer, acct := testAccountID(1), testAccountID(2)
	asset, line := testAsset(t, issuer)
	entries := map[string]string{"k": trustlineEntry(t, testAccountID(9), line, xdr.TrustLineFlagsAuthorizedFlag)}

	resp := &simulator.SimulationResponse{Status: "success", CategorizedEvents: []simulator.CategorizedEvent{
		{EventType: "require_auth", Topics: []string{"require_auth"}, Data: "x"},
		{EventType: "contract", Topics: []string{symTopic(t, "approve"), accountTopic(t, acct), accountTopic(t, acct), strTopic(t, asset)}},
		{EventType: "contract", Topics: []string{"not-base64!!", "b", "c"}},
	}}

	assert.Empty(t, NewAssetSafetyAnalyzer(entries).Analyze(resp))
}

func TestAssetSafety_NoTrustlineDataReportsNothing(t *testing.T) {
	issuer, from, to := testAccountID(1), testAccountID(2), testAccountID(3)
	asset, _ := testAsset(t, issuer)
	resp := &simulator.SimulationResponse{Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{transferEvent(t, from, to, asset, 5)}}

	assert.Empty(t, NewAssetSafetyAnalyzer(nil).Analyze(resp))
	assert.Empty(t, NewAssetSafetyAnalyzer(map[string]string{"bad": "!!"}).Analyze(resp))
	assert.Empty(t, NewAssetSafetyAnalyzer(nil).Analyze(nil))
}

func TestAssetSafety_DuplicateEventsAcrossFeedsDeduped(t *testing.T) {
	issuer, from, to := testAccountID(1), testAccountID(2), testAccountID(3)
	asset, line := testAsset(t, issuer)
	entries := map[string]string{"k1": trustlineEntry(t, from, line, xdr.TrustLineFlagsAuthorizedFlag)}

	cat := transferEvent(t, from, to, asset, 50)
	diag := simulator.DiagnosticEvent{EventType: cat.EventType, ContractID: cat.ContractID, Topics: cat.Topics, Data: cat.Data}
	resp := &simulator.SimulationResponse{Status: "success",
		CategorizedEvents: []simulator.CategorizedEvent{cat}, DiagnosticEvents: []simulator.DiagnosticEvent{diag}}

	assert.Len(t, NewAssetSafetyAnalyzer(entries).Analyze(resp), 1)
}
