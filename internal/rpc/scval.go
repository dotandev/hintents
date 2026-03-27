// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package rpc provides conversion utilities between Soroban ScVal XDR types and
// native Go types. These helpers are a common source of bugs—especially for
// large integers (i128/u128) and nested structures—so they are kept in one
// place and covered by a comprehensive test suite.
package rpc

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// ScValToGoValue converts an xdr.ScVal to the most natural Go representation.
//
// Mapping:
//
//	ScvVoid        → nil
//	ScvBool        → bool
//	ScvError       → ScValError (structured error)
//	ScvU32         → uint32
//	ScvI32         → int32
//	ScvU64         → uint64
//	ScvI64         → int64
//	ScvTimepoint   → uint64
//	ScvDuration    → uint64
//	ScvU128        → *big.Int  (always non-negative)
//	ScvI128        → *big.Int  (may be negative)
//	ScvU256        → *big.Int  (always non-negative)
//	ScvI256        → *big.Int  (may be negative)
//	ScvBytes       → []byte
//	ScvString      → string
//	ScvSymbol      → string
//	ScvVec         → []interface{}  (recursive)
//	ScvMap         → []ScValMapEntry (ordered, recursive)
//	ScvAddress     → string  (strkey or hex)
//	ScvLedgerKeyContractInstance → nil (opaque sentinel)
//	ScvLedgerKeyNonce            → int64
//	ScvContractInstance          → *ScValContractInstance
func ScValToGoValue(v xdr.ScVal) (interface{}, error) {
	switch v.Type {
	case xdr.ScValTypeScvVoid:
		return nil, nil

	case xdr.ScValTypeScvBool:
		if v.B == nil {
			return false, nil
		}
		return bool(*v.B), nil

	case xdr.ScValTypeScvError:
		if v.Error == nil {
			return ScValError{}, nil
		}
		return ScValError{
			Type: uint32(v.Error.Type),
			Code: scErrorCode(v.Error),
		}, nil

	case xdr.ScValTypeScvU32:
		if v.U32 == nil {
			return uint32(0), nil
		}
		return uint32(*v.U32), nil

	case xdr.ScValTypeScvI32:
		if v.I32 == nil {
			return int32(0), nil
		}
		return int32(*v.I32), nil

	case xdr.ScValTypeScvU64:
		if v.U64 == nil {
			return uint64(0), nil
		}
		return uint64(*v.U64), nil

	case xdr.ScValTypeScvI64:
		if v.I64 == nil {
			return int64(0), nil
		}
		return int64(*v.I64), nil

	case xdr.ScValTypeScvTimepoint:
		if v.Timepoint == nil {
			return uint64(0), nil
		}
		return uint64(*v.Timepoint), nil

	case xdr.ScValTypeScvDuration:
		if v.Duration == nil {
			return uint64(0), nil
		}
		return uint64(*v.Duration), nil

	case xdr.ScValTypeScvU128:
		return u128ToBigInt(v.U128)

	case xdr.ScValTypeScvI128:
		return i128ToBigInt(v.I128)

	case xdr.ScValTypeScvU256:
		return u256ToBigInt(v.U256)

	case xdr.ScValTypeScvI256:
		return i256ToBigInt(v.I256)

	case xdr.ScValTypeScvBytes:
		if v.Bytes == nil {
			return []byte{}, nil
		}
		cp := make([]byte, len(*v.Bytes))
		copy(cp, *v.Bytes)
		return cp, nil

	case xdr.ScValTypeScvString:
		if v.Str == nil {
			return "", nil
		}
		return string(*v.Str), nil

	case xdr.ScValTypeScvSymbol:
		if v.Sym == nil {
			return "", nil
		}
		return string(*v.Sym), nil

	case xdr.ScValTypeScvVec:
		return scVecToSlice(v.Vec)

	case xdr.ScValTypeScvMap:
		return scMapToEntries(v.Map)

	case xdr.ScValTypeScvAddress:
		if v.Address == nil {
			return "", nil
		}
		return scAddressToString(*v.Address)

	case xdr.ScValTypeScvLedgerKeyContractInstance:
		return nil, nil

	case xdr.ScValTypeScvLedgerKeyNonce:
		if v.NonceKey == nil {
			return int64(0), nil
		}
		return int64(v.NonceKey.Nonce), nil

	case xdr.ScValTypeScvContractInstance:
		if v.Instance == nil {
			return (*ScValContractInstance)(nil), nil
		}
		return scContractInstanceToGo(v.Instance)

	default:
		return nil, fmt.Errorf("unsupported ScVal type: %v", v.Type)
	}
}

// ScValError is the Go representation of an xdr.ScError.
type ScValError struct {
	Type uint32
	Code uint32
}

// ScValMapEntry is a single key-value pair from an ScMap.
type ScValMapEntry struct {
	Key   interface{}
	Value interface{}
}

// ScValContractInstance is the Go representation of an xdr.ScContractInstance.
type ScValContractInstance struct {
	ExecutableType string
	WasmHash       string // hex-encoded, only set when ExecutableType == "wasm"
	Storage        []ScValMapEntry
}

// ---- helpers ----------------------------------------------------------------

func scErrorCode(e *xdr.ScError) uint32 {
	if e == nil {
		return 0
	}
	if e.Code != nil {
		return uint32(*e.Code)
	}
	if e.ContractCode != nil {
		return uint32(*e.ContractCode)
	}
	return 0
}

// u128ToBigInt converts an xdr.UInt128Parts to a *big.Int.
// UInt128Parts.Hi is the high 64 bits, Lo is the low 64 bits.
func u128ToBigInt(p *xdr.UInt128Parts) (*big.Int, error) {
	if p == nil {
		return big.NewInt(0), nil
	}
	hi := new(big.Int).SetUint64(uint64(p.Hi))
	lo := new(big.Int).SetUint64(uint64(p.Lo))
	result := new(big.Int).Lsh(hi, 64)
	result.Or(result, lo)
	return result, nil
}

// i128ToBigInt converts an xdr.Int128Parts to a signed *big.Int.
// Int128Parts.Hi is a signed int64 (high 64 bits), Lo is unsigned uint64 (low 64 bits).
func i128ToBigInt(p *xdr.Int128Parts) (*big.Int, error) {
	if p == nil {
		return big.NewInt(0), nil
	}
	hi := new(big.Int).SetInt64(int64(p.Hi))
	lo := new(big.Int).SetUint64(uint64(p.Lo))
	// value = hi * 2^64 + lo  (big.Int handles sign extension for negative hi)
	result := new(big.Int).Lsh(hi, 64)
	result.Add(result, lo)
	return result, nil
}

// u256ToBigInt converts an xdr.UInt256Parts to a *big.Int.
// Parts are: HiHi (bits 255-192), HiLo (191-128), LoHi (127-64), LoLo (63-0).
func u256ToBigInt(p *xdr.UInt256Parts) (*big.Int, error) {
	if p == nil {
		return big.NewInt(0), nil
	}
	tmp := new(big.Int)
	result := new(big.Int).SetUint64(uint64(p.HiHi))
	result.Lsh(result, 64)
	result.Or(result, tmp.SetUint64(uint64(p.HiLo)))
	result.Lsh(result, 64)
	result.Or(result, tmp.SetUint64(uint64(p.LoHi)))
	result.Lsh(result, 64)
	result.Or(result, tmp.SetUint64(uint64(p.LoLo)))
	return result, nil
}

// i256ToBigInt converts an xdr.Int256Parts to a signed *big.Int.
// Int256Parts.HiHi is a signed int64; the remaining three limbs are unsigned.
func i256ToBigInt(p *xdr.Int256Parts) (*big.Int, error) {
	if p == nil {
		return big.NewInt(0), nil
	}
	tmp := new(big.Int)
	result := new(big.Int).SetInt64(int64(p.HiHi))
	result.Lsh(result, 64)
	result.Or(result, tmp.SetUint64(uint64(p.HiLo)))
	result.Lsh(result, 64)
	result.Or(result, tmp.SetUint64(uint64(p.LoHi)))
	result.Lsh(result, 64)
	result.Add(result, tmp.SetUint64(uint64(p.LoLo)))
	return result, nil
}

// scVecToSlice converts an **xdr.ScVec to []interface{}.
func scVecToSlice(vec **xdr.ScVec) ([]interface{}, error) {
	if vec == nil || *vec == nil {
		return []interface{}{}, nil
	}
	sv := **vec
	out := make([]interface{}, len(sv))
	for i, elem := range sv {
		v, err := ScValToGoValue(elem)
		if err != nil {
			return nil, fmt.Errorf("vec element %d: %w", i, err)
		}
		out[i] = v
	}
	return out, nil
}

// scMapToEntries converts an **xdr.ScMap to []ScValMapEntry.
func scMapToEntries(m **xdr.ScMap) ([]ScValMapEntry, error) {
	if m == nil || *m == nil {
		return []ScValMapEntry{}, nil
	}
	sm := **m
	out := make([]ScValMapEntry, len(sm))
	for i, entry := range sm {
		k, err := ScValToGoValue(entry.Key)
		if err != nil {
			return nil, fmt.Errorf("map entry %d key: %w", i, err)
		}
		val, err := ScValToGoValue(entry.Val)
		if err != nil {
			return nil, fmt.Errorf("map entry %d value: %w", i, err)
		}
		out[i] = ScValMapEntry{Key: k, Value: val}
	}
	return out, nil
}

// scAddressToString converts an xdr.ScAddress to a human-readable string.
// Account addresses are returned as strkey G-addresses; contract IDs as C-addresses.
func scAddressToString(a xdr.ScAddress) (string, error) {
	s, err := a.String()
	if err != nil {
		return "", fmt.Errorf("ScAddress.String: %w", err)
	}
	return s, nil
}

// scContractInstanceToGo converts an xdr.ScContractInstance to *ScValContractInstance.
func scContractInstanceToGo(inst *xdr.ScContractInstance) (*ScValContractInstance, error) {
	if inst == nil {
		return nil, nil
	}
	result := &ScValContractInstance{}
	switch inst.Executable.Type {
	case xdr.ContractExecutableTypeContractExecutableWasm:
		result.ExecutableType = "wasm"
		if inst.Executable.WasmHash != nil {
			result.WasmHash = hex.EncodeToString(inst.Executable.WasmHash[:])
		}
	case xdr.ContractExecutableTypeContractExecutableStellarAsset:
		result.ExecutableType = "stellar-asset"
	default:
		return nil, fmt.Errorf("unsupported ContractExecutable type: %v", inst.Executable.Type)
	}
	if inst.Storage != nil {
		entries, err := scMapToEntries(&inst.Storage)
		if err != nil {
			return nil, fmt.Errorf("contract instance storage: %w", err)
		}
		result.Storage = entries
	}
	return result, nil
}
