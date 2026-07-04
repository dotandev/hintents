// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"fmt"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// PortConfig defines the parameters for porting a transaction across networks.
type PortConfig struct {
	TargetSequence        uint32
	TargetProtocolVersion uint32
	SequenceOffsets       map[string]int64 // AccountID to sequence offset
}

// PortedTransactionState represents the state required to run a transaction on a new network.
type PortedTransactionState struct {
	Sequence        uint32
	ProtocolVersion uint32
	Entries         map[string]string // XDR encoded ledger entries
}

// PortTransactionState translates ledger footprints and parameters from a source network to a target network.
func PortTransactionState(sourceHeader *rpc.LedgerHeaderResponse, sourceEntries map[string]string, config PortConfig) (result *PortedTransactionState, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in PortTransactionState: %v", r)
		}
	}()

	if sourceHeader == nil {
		return nil, errors.New("source header is required")
	}

	portedEntries := make(map[string]string)

	// Port ledger entries
	for keyXDR, entryXDR := range sourceEntries {
		entryBytes, err := base64.StdEncoding.DecodeString(entryXDR)
		if err != nil {
			return nil, errors.WrapUnmarshalFailed(err, "source entry")
		}

		var entry xdr.LedgerEntry
		if err := entry.UnmarshalBinary(entryBytes); err != nil {
			return nil, errors.WrapUnmarshalFailed(err, "source entry binary")
		}

		// Adapt the LastModifiedLedgerSeq to the new target network's sequence
		entry.LastModifiedLedgerSeq = xdr.Uint32(config.TargetSequence)

		// Adapt Account Sequence Numbers if provided in config
		if entry.Data.Type == xdr.LedgerEntryTypeAccount && entry.Data.Account != nil {
			accountID := entry.Data.Account.AccountId.Address()
			if offset, exists := config.SequenceOffsets[accountID]; exists {
				newSeq := int64(entry.Data.Account.SeqNum) + offset
				if newSeq < 0 {
					newSeq = 0
				}
				entry.Data.Account.SeqNum = xdr.SequenceNumber(newSeq)
			}
		}

		// Re-encode ported entry
		portedEntryXDR, err := rpc.EncodeLedgerEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("failed to encode ported entry: %w", err)
		}

		portedEntries[keyXDR] = portedEntryXDR
	}

	return &PortedTransactionState{
		Sequence:        config.TargetSequence,
		ProtocolVersion: config.TargetProtocolVersion,
		Entries:         portedEntries,
	}, nil
}
