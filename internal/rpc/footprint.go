// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// IncompleteFootprintError reports footprint keys that could be resolved
// neither from the transaction metadata nor from the network.
//
// Replaying without them does not fail cleanly: the host aborts with
// "trying to access contract instance outside of the footprint", which names
// no key and reads like a bug in the contract rather than missing input. This
// error exists so the gap is reported where it is still explicable.
type IncompleteFootprintError struct {
	Keys []string
}

func (e *IncompleteFootprintError) Error() string {
	return fmt.Sprintf(
		"incomplete replay state: %d footprint %s could not be loaded (%s); "+
			"the simulation would fail inside the host with an \"outside of the footprint\" error",
		len(e.Keys),
		pluralize("entry", "entries", len(e.Keys)),
		strings.Join(describeLedgerKeys(e.Keys), ", "),
	)
}

func pluralize(singular, plural string, n int) string {
	if n == 1 {
		return singular
	}
	return plural
}

// describeLedgerKeys renders base64 ledger keys as their entry types, counted,
// so the message says "3 ContractData, 1 ContractCode" instead of base64 blobs.
func describeLedgerKeys(keys []string) []string {
	counts := make(map[string]int)
	for _, k := range keys {
		counts[ledgerKeyTypeName(k)]++
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	described := make([]string, 0, len(names))
	for _, name := range names {
		described = append(described, fmt.Sprintf("%d %s", counts[name], name))
	}
	return described
}

func ledgerKeyTypeName(encoded string) string {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "unknown"
	}
	var key xdr.LedgerKey
	if err := key.UnmarshalBinary(raw); err != nil {
		return "unknown"
	}
	return strings.TrimPrefix(key.Type.String(), "LedgerEntryType")
}

// ExtractFootprintKeys returns the ledger keys a Soroban transaction declared it
// would touch, as base64 XDR, read-only keys first.
//
// This is the authoritative read set. The protocol rejects any access outside
// it, so it is precisely the state a replay has to have loaded — and it lives
// on the envelope, not in the result metadata. Entries the transaction only
// read (the contract instance and its code, typically) never appear as ledger
// entry changes at all, which is why metadata alone cannot reconstruct it.
//
// A classic, non-Soroban transaction declares no footprint and returns nil.
func ExtractFootprintKeys(envelopeXDR string) ([]string, error) {
	raw, err := base64.StdEncoding.DecodeString(envelopeXDR)
	if err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "transaction envelope")
	}

	var envelope xdr.TransactionEnvelope
	if err := envelope.UnmarshalBinary(raw); err != nil {
		return nil, errors.WrapUnmarshalFailed(err, "transaction envelope binary")
	}

	return FootprintKeysFromEnvelope(envelope)
}

// FootprintKeysFromEnvelope is ExtractFootprintKeys for a decoded envelope.
func FootprintKeysFromEnvelope(envelope xdr.TransactionEnvelope) ([]string, error) {
	data := sorobanDataFromEnvelope(envelope)
	if data == nil {
		return nil, nil
	}

	footprint := data.Resources.Footprint
	keys := make([]string, 0, len(footprint.ReadOnly)+len(footprint.ReadWrite))
	seen := make(map[string]struct{}, cap(keys))

	for _, key := range append(append([]xdr.LedgerKey{}, footprint.ReadOnly...), footprint.ReadWrite...) {
		encoded, err := EncodeLedgerKey(key)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[encoded]; duplicate {
			continue
		}
		seen[encoded] = struct{}{}
		keys = append(keys, encoded)
	}

	return keys, nil
}

// sorobanDataFromEnvelope reaches the Soroban resource declaration through
// whichever envelope shape the transaction uses. V0 envelopes predate Soroban
// and carry none.
func sorobanDataFromEnvelope(envelope xdr.TransactionEnvelope) *xdr.SorobanTransactionData {
	switch envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		if envelope.V1 != nil {
			return envelope.V1.Tx.Ext.SorobanData
		}
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		if envelope.FeeBump != nil {
			if inner := envelope.FeeBump.Tx.InnerTx.V1; inner != nil {
				return inner.Tx.Ext.SorobanData
			}
		}
	}
	return nil
}

// ResolveReplayState assembles the ledger state a transaction replay must start
// from: the pre-transaction value of every key in the transaction's declared
// footprint.
//
// Metadata is preferred wherever it covers a key, because it carries the value
// as of the ledger the transaction was applied in. Whatever it does not cover is
// fetched from the network, which answers with the entry as it stands now —
// exact for immutable entries such as contract code, an approximation for
// contract data that has changed since.
//
// Keys the metadata proves did not exist beforehand are deliberately left unset
// rather than fetched, so a contract that branches on absence still takes the
// branch it originally took.
//
// A classic transaction declares no footprint; for those the metadata is all
// there is and is returned as-is.
func (c *Client) ResolveReplayState(ctx context.Context, envelopeXDR, resultMetaXDR string) (map[string]string, error) {
	pre, err := ExtractPreTxLedgerState(resultMetaXDR)
	if err != nil {
		return nil, err
	}

	entries := make(map[string]string, len(pre.Entries))
	for key, value := range pre.Entries {
		entries[key] = value
	}

	absent := make(map[string]struct{}, len(pre.KnownAbsent))
	for key := range pre.KnownAbsent {
		absent[key] = struct{}{}
	}

	footprint, err := ExtractFootprintKeys(envelopeXDR)
	if err != nil {
		return nil, err
	}
	if len(footprint) == 0 {
		return entries, nil
	}

	missing := unresolvedKeys(footprint, entries, absent)
	if len(missing) > 0 {
		logger.Logger.Debug(
			"Fetching footprint entries absent from transaction metadata",
			"missing", len(missing),
			"footprint", len(footprint),
		)

		fetched, fetchErr := c.GetLedgerEntries(ctx, missing)
		if fetchErr != nil {
			return nil, fmt.Errorf(
				"fetching %d footprint %s absent from transaction metadata: %w",
				len(missing), pluralize("entry", "entries", len(missing)), fetchErr,
			)
		}
		for key, value := range fetched {
			if _, have := entries[key]; !have {
				entries[key] = value
			}
		}

		// A key the network answered for with nothing simply holds no entry —
		// the transaction was about to create it, or it expired and was
		// archived. The replay has to start without it, so record it as
		// resolved rather than as a gap.
		for _, key := range missing {
			if _, loaded := entries[key]; !loaded {
				absent[key] = struct{}{}
			}
		}
	}

	if stillMissing := unresolvedKeys(footprint, entries, absent); len(stillMissing) > 0 {
		return nil, &IncompleteFootprintError{Keys: stillMissing}
	}

	logger.Logger.Info(
		"Resolved replay state",
		"entries", len(entries),
		"footprint", len(footprint),
		"from_metadata", len(pre.Entries),
		"absent", len(absent),
	)

	return entries, nil
}

// unresolvedKeys returns the footprint keys that are neither loaded nor known
// to have been absent before the transaction ran.
func unresolvedKeys(footprint []string, entries map[string]string, knownAbsent map[string]struct{}) []string {
	var unresolved []string
	for _, key := range footprint {
		if _, loaded := entries[key]; loaded {
			continue
		}
		if _, absent := knownAbsent[key]; absent {
			continue
		}
		unresolved = append(unresolved, key)
	}
	return unresolved
}
