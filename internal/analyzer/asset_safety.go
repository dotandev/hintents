// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/visualizer"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// PrintAssetAnomalies formats and prints Move-Level asset safety anomalies.
func PrintAssetAnomalies(anomalies []simulator.AssetAnomaly) {
	if len(anomalies) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(visualizer.Colorize("=== Move-Level Asset Safety Anomalies Detected ===", "red"))
	fmt.Println(visualizer.Colorize("These mathematical violations were detected during simulation:", "yellow"))

	for i, anomaly := range anomalies {
		fmt.Printf("\n%d. [%s] in Contract %s\n", i+1, visualizer.Colorize(anomaly.AnomalyType, "red"), anomaly.ContractID)
		fmt.Printf("   Details: %s\n", anomaly.Message)
		fmt.Printf("   Amount involved: %d\n", anomaly.Amount)
	}

	fmt.Println(visualizer.Colorize("==================================================", "red"))
	fmt.Println()
}

// Warning types emitted by AssetSafetyAnalyzer.
const (
	// WarningMissingTrustline: a classic account is party to a Stellar Asset
	// Contract (SAC) balance operation but holds no trustline for the asset.
	WarningMissingTrustline = "MissingTrustline"
	// WarningTrustlineNotAuthorized: the trustline exists but lacks AUTHORIZED_FLAG.
	WarningTrustlineNotAuthorized = "TrustlineNotAuthorized"
	// WarningTrustlineMaintainLiabilitiesOnly: the trustline only carries
	// AUTHORIZED_TO_MAINTAIN_LIABILITIES_FLAG, which does not permit balance changes.
	WarningTrustlineMaintainLiabilitiesOnly = "TrustlineMaintainLiabilitiesOnly"
)

// AssetSafetyWarning reports a SAC balance operation that the simulation
// performed (or attempted) against a classic account that is not entitled to it.
type AssetSafetyWarning struct {
	Type       string
	Severity   string
	Account    string // classic account (G...) party to the operation
	Asset      string // canonical asset ("CODE:ISSUER")
	Operation  string // SAC function: transfer, mint or burn
	Role       string // "sender" or "recipient"
	Amount     string // decimal i128 amount, empty if not decodable
	ContractID string
	Location   string // e.g. "event_index:3"
	Message    string
}

// AssetSafetyAnalyzer inspects Stellar Asset Contract (SAC) events and checks
// that every classic account involved in a balance change has a trustline for
// the asset carrying AUTHORIZED_FLAG.
//
// Trustline state is read from the ledger entries supplied with the simulation
// request. If no trustline entries are available the analyzer has nothing to
// compare against and reports nothing, rather than guessing.
type AssetSafetyAnalyzer struct {
	trustlines    map[string]uint32 // "account|asset" -> trustline flags
	haveTrustData bool
}

// NewAssetSafetyAnalyzer builds an analyzer from the base64 ledger entries of a
// simulation request (SimulationRequest.LedgerEntries). Entries that are not
// trustlines, or that cannot be decoded, are ignored.
func NewAssetSafetyAnalyzer(ledgerEntries map[string]string) *AssetSafetyAnalyzer {
	a := &AssetSafetyAnalyzer{trustlines: make(map[string]uint32)}

	for _, raw := range ledgerEntries {
		var entry xdr.LedgerEntry
		if err := xdr.SafeUnmarshalBase64(raw, &entry); err != nil {
			continue
		}
		if entry.Data.Type != xdr.LedgerEntryTypeTrustline || entry.Data.TrustLine == nil {
			continue
		}
		tl := entry.Data.TrustLine
		if tl.Asset.Type == xdr.AssetTypeAssetTypePoolShare {
			continue
		}
		a.trustlines[trustlineKey(tl.AccountId.Address(), tl.Asset.ToAsset().StringCanonical())] = uint32(tl.Flags)
		a.haveTrustData = true
	}

	return a
}

func trustlineKey(account, asset string) string { return account + "|" + asset }

// Analyze scans the response's events for SAC transfer/mint/burn operations and
// returns a warning for each classic account lacking a valid, authorized trustline.
func (a *AssetSafetyAnalyzer) Analyze(resp *simulator.SimulationResponse) []AssetSafetyWarning {
	var warnings []AssetSafetyWarning
	if resp == nil || !a.haveTrustData {
		return warnings
	}

	seen := make(map[string]struct{})
	check := func(index int, contractID *string, topics []string, data string) {
		op, ok := parseSACOperation(topics, data)
		if !ok {
			return
		}
		cid := ""
		if contractID != nil {
			cid = *contractID
		}
		for _, p := range op.parties {
			w, bad := a.checkParty(p, op)
			if !bad {
				continue
			}
			w.ContractID = cid
			w.Location = fmt.Sprintf("event_index:%d", index)
			// Categorized and diagnostic events can describe the same event.
			key := strings.Join([]string{w.Type, w.Account, w.Asset, w.Operation, w.Role, w.Amount, w.ContractID}, "|")
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			warnings = append(warnings, w)
		}
	}

	for i, e := range resp.CategorizedEvents {
		check(i, e.ContractID, e.Topics, e.Data)
	}
	for i, e := range resp.DiagnosticEvents {
		check(i, e.ContractID, e.Topics, e.Data)
	}

	return warnings
}

func (a *AssetSafetyAnalyzer) checkParty(p sacParty, op sacOperation) (AssetSafetyWarning, bool) {
	w := AssetSafetyWarning{
		Severity:  "high",
		Account:   p.account,
		Asset:     op.asset,
		Operation: op.name,
		Role:      p.role,
		Amount:    op.amount,
	}

	flags, ok := a.trustlines[trustlineKey(p.account, op.asset)]
	switch {
	case !ok:
		w.Type = WarningMissingTrustline
		w.Message = fmt.Sprintf("SAC %s %s %s for %s, which has no trustline for this asset",
			op.name, roleVerb(p.role), amountSuffix(op.amount), p.account)
	case flags&uint32(xdr.TrustLineFlagsAuthorizedFlag) != 0:
		return w, false
	case flags&uint32(xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag) != 0:
		w.Type = WarningTrustlineMaintainLiabilitiesOnly
		w.Message = fmt.Sprintf("SAC %s %s %s for %s, whose trustline is only authorized to maintain liabilities",
			op.name, roleVerb(p.role), amountSuffix(op.amount), p.account)
	default:
		w.Type = WarningTrustlineNotAuthorized
		w.Message = fmt.Sprintf("SAC %s %s %s for %s, whose trustline is not authorized",
			op.name, roleVerb(p.role), amountSuffix(op.amount), p.account)
	}
	return w, true
}

func roleVerb(role string) string {
	if role == "recipient" {
		return "credits"
	}
	return "debits"
}

func amountSuffix(amount string) string {
	if amount == "" {
		return "a balance"
	}
	return "an amount of " + amount
}

type sacParty struct {
	account string
	role    string
}

type sacOperation struct {
	name    string
	asset   string
	amount  string
	parties []sacParty
}

// parseSACOperation recognises the CAP-46-6 SAC events:
//
//	transfer: [transfer, from, to, asset]
//	mint:     [mint, to, asset]
//	burn:     [burn, from, asset]
//
// The trailing topic is the asset ("native" or "CODE:ISSUER"). Only classic
// (G...) accounts that are not the asset's issuer are returned as parties, since
// contracts hold SAC balances in contract data and the issuer needs no trustline.
// Native XLM never needs a trustline and is skipped.
func parseSACOperation(topics []string, data string) (sacOperation, bool) {
	var op sacOperation
	if len(topics) < 3 {
		return op, false
	}

	vals := make([]xdr.ScVal, len(topics))
	for i, t := range topics {
		raw, err := base64.StdEncoding.DecodeString(t)
		if err != nil || xdr.SafeUnmarshal(raw, &vals[i]) != nil {
			return op, false
		}
	}

	if vals[0].Type != xdr.ScValTypeScvSymbol {
		return op, false
	}
	name := string(vals[0].MustSym())

	last := vals[len(vals)-1]
	if last.Type != xdr.ScValTypeScvString {
		return op, false
	}
	asset := string(last.MustStr())
	issuer := ""
	if asset == "native" {
		return op, false
	}
	if idx := strings.LastIndex(asset, ":"); idx > 0 {
		issuer = asset[idx+1:]
	} else {
		return op, false
	}

	var roles []string
	switch name {
	case "transfer":
		if len(vals) != 4 {
			return op, false
		}
		roles = []string{"sender", "recipient"}
	case "mint":
		if len(vals) != 3 {
			return op, false
		}
		roles = []string{"recipient"}
	case "burn":
		if len(vals) != 3 {
			return op, false
		}
		roles = []string{"sender"}
	default:
		return op, false
	}

	op = sacOperation{name: name, asset: asset, amount: decodeI128(data)}
	for i, role := range roles {
		account, ok := classicAccount(vals[1+i])
		if !ok || account == issuer {
			continue
		}
		op.parties = append(op.parties, sacParty{account: account, role: role})
	}
	return op, len(op.parties) > 0
}

// classicAccount returns the G... address behind an account or muxed-account
// ScVal address; contracts and other address kinds are not classic accounts.
func classicAccount(v xdr.ScVal) (string, bool) {
	if v.Type != xdr.ScValTypeScvAddress || v.Address == nil {
		return "", false
	}
	switch v.Address.Type {
	case xdr.ScAddressTypeScAddressTypeAccount:
		id := v.Address.MustAccountId()
		return id.Address(), true
	case xdr.ScAddressTypeScAddressTypeMuxedAccount:
		m := v.Address.MustMuxedAccount()
		id := xdr.AccountId{Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: &m.Ed25519}
		return id.Address(), true
	}
	return "", false
}

// decodeI128 renders an i128 amount (or the "amount" of a CAP-67 muxed map)
// as a decimal string, returning "" if the data is in neither form.
func decodeI128(data string) string {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return ""
	}
	var v xdr.ScVal
	if xdr.SafeUnmarshal(raw, &v) != nil {
		return ""
	}
	if v.Type == xdr.ScValTypeScvMap && v.Map != nil && *v.Map != nil {
		for _, e := range **v.Map {
			if e.Key.Type == xdr.ScValTypeScvSymbol && string(e.Key.MustSym()) == "amount" {
				v = e.Val
				break
			}
		}
	}
	if v.Type != xdr.ScValTypeScvI128 || v.I128 == nil {
		return ""
	}
	n := new(big.Int).Lsh(big.NewInt(int64(v.I128.Hi)), 64)
	n.Add(n, new(big.Int).SetUint64(uint64(v.I128.Lo)))
	return n.String()
}

// PrintAssetSafetyWarnings formats and prints trustline/authorization warnings.
func PrintAssetSafetyWarnings(warnings []AssetSafetyWarning) {
	if len(warnings) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(visualizer.Colorize("=== Unauthorized Asset Operations Detected ===", "red"))
	fmt.Println(visualizer.Colorize("SAC balance operations touched accounts without a valid trustline:", "yellow"))

	for i, w := range warnings {
		fmt.Printf("\n%d. [%s] %s\n", i+1, visualizer.Colorize(w.Type, "red"), w.Message)
		fmt.Printf("   Asset: %s  Account: %s (%s)\n", w.Asset, w.Account, w.Role)
		if w.Location != "" {
			fmt.Printf("   Location: %s\n", w.Location)
		}
	}

	fmt.Println(visualizer.Colorize("==============================================", "red"))
	fmt.Println()
}
