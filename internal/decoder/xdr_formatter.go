// Copyright 2026 dotandev
// SPDX-License-Identifier: Apache-2.0

package decoder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

type FormatType string

const (
	FormatJSON  FormatType = "json"
	FormatTable FormatType = "table"
)

type XDRFormatter struct {
	format FormatType
}

func NewXDRFormatter(format FormatType) *XDRFormatter {
	return &XDRFormatter{format: format}
}

func (f *XDRFormatter) Format(data interface{}) (string, error) {
	switch f.format {
	case FormatJSON:
		return f.formatJSON(data)
	case FormatTable:
		return f.formatTable(data)
	default:
		return "", fmt.Errorf("unsupported format: %s", f.format)
	}
}

func (f *XDRFormatter) formatJSON(data interface{}) (string, error) {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(output), nil
}

func (f *XDRFormatter) formatTable(data interface{}) (string, error) {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	switch v := data.(type) {
	case *xdr.LedgerEntry:
		return formatLedgerEntryTable(w, v)
	case *xdr.TransactionEnvelope:
		return formatTransactionEnvelopeTable(w, v)
	case *xdr.DiagnosticEvent:
		return formatDiagnosticEventTable(w, v)
	case []interface{}:
		return formatGenericTable(w, v)
	default:
		fmt.Fprintf(w, "Type:\t%T\n", v)
		fmt.Fprintf(w, "Value:\t%v\n", v)
		w.Flush()
		return buf.String(), nil
	}
}

func formatLedgerEntryTable(w *tabwriter.Writer, entry *xdr.LedgerEntry) (string, error) {
	var buf bytes.Buffer
	w = tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Type:\t%v\n", entry.Data.Type)
	fmt.Fprintf(w, "Last Modified Ledger:\t%d\n", entry.LastModifiedLedgerSeq)

	switch entry.Data.Type {
	case xdr.LedgerEntryTypeAccount:
		if entry.Data.Account != nil {
			acc := entry.Data.Account
			fmt.Fprintf(w, "Account ID:\t%s\n", acc.AccountId.Address())
			fmt.Fprintf(w, "Balance:\t%d\n", acc.Balance)
			fmt.Fprintf(w, "Sequence:\t%d\n", acc.SeqNum)
			fmt.Fprintf(w, "Flags:\t%d\n", acc.Flags)
		}

	case xdr.LedgerEntryTypeTrustline:
		if entry.Data.TrustLine != nil {
			tl := entry.Data.TrustLine
			fmt.Fprintf(w, "Account:\t%s\n", tl.AccountId.Address())
			fmt.Fprintf(w, "Asset Type:\t%v\n", tl.Asset.Type)
			fmt.Fprintf(w, "Balance:\t%d\n", tl.Balance)
			fmt.Fprintf(w, "Flags:\t%d\n", tl.Flags)
		}

	case xdr.LedgerEntryTypeOffer:
		if entry.Data.Offer != nil {
			offer := entry.Data.Offer
			fmt.Fprintf(w, "Seller:\t%s\n", offer.SellerId.Address())
			fmt.Fprintf(w, "Offer ID:\t%d\n", offer.OfferId)
			fmt.Fprintf(w, "Amount:\t%d\n", offer.Amount)
		}

	case xdr.LedgerEntryTypeData:
		if entry.Data.Data != nil {
			data := entry.Data.Data
			fmt.Fprintf(w, "Account:\t%s\n", data.AccountId.Address())
			fmt.Fprintf(w, "Data Name:\t%s\n", data.DataName)
			fmt.Fprintf(w, "Data Value (bytes):\t%d\n", len(data.DataValue))
		}

	case xdr.LedgerEntryTypeClaimableBalance:
		if entry.Data.ClaimableBalance != nil {
			cb := entry.Data.ClaimableBalance
			if cb.BalanceId.V0 != nil {
				fmt.Fprintf(w, "Balance ID:\t%x\n", *cb.BalanceId.V0)
			}
			fmt.Fprintf(w, "Amount:\t%d\n", cb.Amount)
		}

	case xdr.LedgerEntryTypeContractData:
		if entry.Data.ContractData != nil {
			cd := entry.Data.ContractData
			fmt.Fprintf(w, "Durability:\t%v\n", cd.Durability)
		}

	case xdr.LedgerEntryTypeContractCode:
		if entry.Data.ContractCode != nil {
			cc := entry.Data.ContractCode
			fmt.Fprintf(w, "Code Hash:\t%x\n", cc.Hash)
			fmt.Fprintf(w, "Code Size:\t%d bytes\n", len(cc.Code))
		}
	}

	w.Flush()
	return buf.String(), nil
}

func formatTransactionEnvelopeTable(w *tabwriter.Writer, env *xdr.TransactionEnvelope) (string, error) {
	var buf bytes.Buffer
	w = tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Envelope Type:\t%v\n", env.Type)

	switch env.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		if env.V0 != nil {
			tx := env.V0.Tx
			fmt.Fprintf(w, "Fee:\t%d\n", tx.Fee)
			fmt.Fprintf(w, "Sequence Num:\t%d\n", tx.SeqNum)
			fmt.Fprintf(w, "Operations:\t%d\n", len(tx.Operations))
		}

	case xdr.EnvelopeTypeEnvelopeTypeTx:
		if env.V1 != nil {
			tx := env.V1.Tx
			fmt.Fprintf(w, "Source Account:\t%s\n", tx.SourceAccount.Address())
			fmt.Fprintf(w, "Fee:\t%d\n", tx.Fee)
			fmt.Fprintf(w, "Sequence Num:\t%d\n", tx.SeqNum)
			fmt.Fprintf(w, "Operations:\t%d\n", len(tx.Operations))
		}

	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		if env.FeeBump != nil {
			feeBump := env.FeeBump.Tx
			fmt.Fprintf(w, "Fee Source:\t%s\n", feeBump.FeeSource.Address())
			fmt.Fprintf(w, "Fee:\t%d\n", feeBump.Fee)
		}
	}

	w.Flush()
	return buf.String(), nil
}

func formatDiagnosticEventTable(w *tabwriter.Writer, event *xdr.DiagnosticEvent) (string, error) {
	var buf bytes.Buffer
	w = tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Successful:\t%v\n", event.InSuccessfulContractCall)
	fmt.Fprintf(w, "Event Type:\t%v\n", event.Event.Type)

	if event.Event.ContractId != nil {
		fmt.Fprintf(w, "Contract ID:\t%x\n", event.Event.ContractId)
	}

	w.Flush()
	return buf.String(), nil
}

func formatGenericTable(w *tabwriter.Writer, items []interface{}) (string, error) {
	var buf bytes.Buffer
	w = tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	fmt.Fprintf(w, "Items:\t%d\n", len(items))

	for i, item := range items {
		if i > 0 {
			fmt.Fprintf(w, "\n")
		}
		fmt.Fprintf(w, "Item %d:\t%T\n", i, item)
	}

	w.Flush()
	return buf.String(), nil
}

func DecodeXDRBase64AsLedgerEntry(data string) (*xdr.LedgerEntry, error) {
	var entry xdr.LedgerEntry
	if err := entry.UnmarshalBinary([]byte(data)); err != nil {
		return nil, fmt.Errorf("failed to decode ledger entry: %w", err)
	}
	return &entry, nil
}

func DecodeXDRBase64AsDiagnosticEvent(data string) (*xdr.DiagnosticEvent, error) {
	var event xdr.DiagnosticEvent
	if err := event.UnmarshalBinary([]byte(data)); err != nil {
		return nil, fmt.Errorf("failed to decode diagnostic event: %w", err)
	}
	return &event, nil
}

func SummarizeXDRObject(data interface{}) string {
	switch v := data.(type) {
	case *xdr.LedgerEntry:
		if v == nil {
			return "empty ledger entry"
		}
		return fmt.Sprintf("LedgerEntry(%v)", v.Data.Type)

	case *xdr.TransactionEnvelope:
		if v == nil {
			return "empty transaction envelope"
		}
		return fmt.Sprintf("TransactionEnvelope(%v)", v.Type)

	case *xdr.DiagnosticEvent:
		if v == nil {
			return "empty diagnostic event"
		}
		return fmt.Sprintf("DiagnosticEvent(successful=%v)", v.InSuccessfulContractCall)

	default:
		return fmt.Sprintf("%T", v)
	}
}
