// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package analytics

import "fmt"

type StorageFeeModel struct {
	FeePerByte uint64 // protocol-defined
}

type StorageRentModel struct {
	LedgerWriteFeePerByte   uint64
	RentFeePerBytePerLedger uint64
	MinPersistentEntryTTL   uint32
	MaxEntryTTL             uint32
}

type StorageRentEstimate struct {
	LiveTTL        uint32
	LedgerWriteFee uint64
	RentFee        uint64
}

func CalculateStorageFee(deltaBytes int64, model StorageFeeModel) int64 {
	if deltaBytes <= 0 {
		return 0
	}
	return deltaBytes * int64(model.FeePerByte)
}

func CalculateStorageRent(entryBytes uint64, currentTTL, requestedBump uint32, model StorageRentModel) (StorageRentEstimate, error) {
	if model.MaxEntryTTL == 0 || model.MinPersistentEntryTTL > model.MaxEntryTTL {
		return StorageRentEstimate{}, fmt.Errorf("invalid storage rent TTL bounds: min %d, max %d", model.MinPersistentEntryTTL, model.MaxEntryTTL)
	}

	bump := requestedBump
	if bump < model.MinPersistentEntryTTL {
		bump = model.MinPersistentEntryTTL
	}

	liveTTL := currentTTL
	if currentTTL < model.MaxEntryTTL {
		proposedTTL := uint64(currentTTL) + uint64(bump)
		if proposedTTL > uint64(model.MaxEntryTTL) {
			proposedTTL = uint64(model.MaxEntryTTL)
		}
		liveTTL = uint32(proposedTTL)
	}

	extendedTTL := uint64(liveTTL - currentTTL)
	if extendedTTL == 0 {
		return StorageRentEstimate{LiveTTL: liveTTL}, nil
	}

	const maxUint64 = ^uint64(0)
	if model.LedgerWriteFeePerByte != 0 && entryBytes > maxUint64/model.LedgerWriteFeePerByte {
		return StorageRentEstimate{}, fmt.Errorf("ledger write fee overflows uint64")
	}
	ledgerWriteFee := entryBytes * model.LedgerWriteFeePerByte

	if extendedTTL != 0 && model.RentFeePerBytePerLedger != 0 && entryBytes > maxUint64/extendedTTL/model.RentFeePerBytePerLedger {
		return StorageRentEstimate{}, fmt.Errorf("storage rent fee overflows uint64")
	}
	rentFee := entryBytes * extendedTTL * model.RentFeePerBytePerLedger

	return StorageRentEstimate{
		LiveTTL:        liveTTL,
		LedgerWriteFee: ledgerWriteFee,
		RentFee:        rentFee,
	}, nil
}
