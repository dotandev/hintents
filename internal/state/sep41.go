// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/binary"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

// The training corpus is synthesised deterministically rather than shipped as
// a binary blob so that the dictionary is reproducible and reviewable. Every
// sample is a canonical XDR LedgerEntryData (CONTRACT_DATA) laid out exactly as
// SEP-41 token contracts store their state:
//
//   - Stellar Asset Contract balances:  Vec[Balance, Address] -> Map{amount, authorized, clawback}
//   - Custom token balances:            Vec[Balance, Address] -> I128
//   - Allowances (temporary storage):   Vec[Allowance, Map{from, spender}] -> Map{amount, expiration_ledger}
//   - Contract instance metadata:       LedgerKeyContractInstance -> Instance{METADATA, Admin}
//
// Changing anything here changes the dictionary; bump DictionaryID when you do.

const (
	sep41SampleSeed      = 0x5345503431 // "SEP41"
	sep41SamplesPerKind  = 64
	sep41HistoryPerKind  = 16
	sep41ContractIDCount = 4
)

// XDR discriminants (see Stellar-contract.x / Stellar-ledger-entries.x).
const (
	xdrLedgerEntryContractData = 6

	scvBool                      = 0
	scvU32                       = 3
	scvI128                      = 10
	scvString                    = 14
	scvSymbol                    = 15
	scvVec                       = 16
	scvMap                       = 17
	scvAddress                   = 18
	scvContractInstance          = 19
	scvLedgerKeyContractInstance = 20

	scAddressAccount  = 0
	scAddressContract = 1

	publicKeyEd25519 = 0

	contractExecutableWasm = 0

	durabilityTemporary  = 0
	durabilityPersistent = 1
)

// TrainSEP41Dictionary trains a zstd dictionary (entropy tables plus history)
// on the synthetic SEP-41 state corpus.
func TrainSEP41Dictionary() (dict []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			dict, err = nil, fmt.Errorf("state: dictionary training panicked: %v", r)
		}
	}()

	return zstd.BuildDict(zstd.BuildDictOptions{
		ID:       DictionaryID,
		Contents: SEP41Samples(),
		History:  sep41History(),
		Offsets:  [3]int{1, 4, 8},
	})
}

// SEP41Samples returns the deterministic training corpus of XDR-encoded
// SEP-41 ledger entries.
func SEP41Samples() [][]byte {
	return sep41Samples(sep41SampleSeed, sep41SamplesPerKind)
}

// sep41History concatenates a subset of the corpus to serve as dictionary
// history (the content that compressed values back-reference).
func sep41History() []byte {
	var hist []byte
	for _, s := range sep41Samples(sep41SampleSeed, sep41HistoryPerKind) {
		hist = append(hist, s...)
	}
	return hist
}

// sep41Samples generates perKind samples of each SEP-41 entry shape.
func sep41Samples(seed uint64, perKind int) [][]byte {
	rng := splitmix64(seed)

	// Real stores hold many entries for a handful of token contracts, so
	// contract IDs are drawn from a small fixed set shared across seeds.
	contractRNG := splitmix64(sep41SampleSeed)
	contracts := make([][32]byte, sep41ContractIDCount)
	for i := range contracts {
		contracts[i] = contractRNG.bytes32()
	}

	samples := make([][]byte, 0, perKind*4)
	for i := 0; i < perKind; i++ {
		contract := contracts[i%len(contracts)]
		// Amounts are in stroops (7 decimals); realistic balances leave the
		// high bytes zero, which is part of what the dictionary should learn.
		samples = append(samples,
			sacBalanceEntry(contract, rng.bytes32(), rng.next()>>16, rng.next()%8 != 0),
			tokenBalanceEntry(contract, rng.bytes32(), i%3 == 0, rng.next()>>16),
			allowanceEntry(contract, rng.bytes32(), rng.bytes32(), rng.next()>>16, uint32(rng.next()>>40)),
			instanceEntry(contract, rng.bytes32(), rng.bytes32(), i),
		)
	}
	return samples
}

func sacBalanceEntry(contract, holder [32]byte, amount uint64, authorized bool) []byte {
	var w xdrWriter
	w.contractDataHeader(contract)
	w.vec(2)
	w.symbol("Balance")
	w.accountAddress(holder)
	w.u32(durabilityPersistent)
	w.mapHeader(3)
	w.symbol("amount")
	w.i128(0, amount)
	w.symbol("authorized")
	w.boolean(authorized)
	w.symbol("clawback")
	w.boolean(false)
	return w.buf
}

func tokenBalanceEntry(contract, holder [32]byte, holderIsContract bool, amount uint64) []byte {
	var w xdrWriter
	w.contractDataHeader(contract)
	w.vec(2)
	w.symbol("Balance")
	if holderIsContract {
		w.contractAddress(holder)
	} else {
		w.accountAddress(holder)
	}
	w.u32(durabilityPersistent)
	w.i128(0, amount)
	return w.buf
}

func allowanceEntry(contract, from, spender [32]byte, amount uint64, expiration uint32) []byte {
	var w xdrWriter
	w.contractDataHeader(contract)
	w.vec(2)
	w.symbol("Allowance")
	w.mapHeader(2)
	w.symbol("from")
	w.accountAddress(from)
	w.symbol("spender")
	w.accountAddress(spender)
	w.u32(durabilityTemporary)
	w.mapHeader(2)
	w.symbol("amount")
	w.i128(0, amount)
	w.symbol("expiration_ledger")
	w.scU32(expiration)
	return w.buf
}

var sep41TokenNames = [...][2]string{
	{"USD Coin", "USDC"},
	{"Euro Coin", "EURC"},
	{"Stellar Lumens", "XLM"},
	{"Wrapped Bitcoin", "WBTC"},
}

func instanceEntry(contract, wasmHash, admin [32]byte, i int) []byte {
	name := sep41TokenNames[i%len(sep41TokenNames)]

	var w xdrWriter
	w.contractDataHeader(contract)
	w.u32(scvLedgerKeyContractInstance)
	w.u32(durabilityPersistent)

	w.u32(scvContractInstance)
	w.u32(contractExecutableWasm)
	w.fixed(wasmHash[:])
	w.u32(1) // storage present
	w.u32(2) // storage map entries (sorted: Symbol < Vec)
	w.symbol("METADATA")
	w.mapHeader(3)
	w.symbol("decimal")
	w.scU32(7)
	w.symbol("name")
	w.scString(name[0])
	w.symbol("symbol")
	w.scString(name[1])
	w.vec(1)
	w.symbol("Admin")
	w.accountAddress(admin)
	return w.buf
}

// ---------------------------------------------------------------------------
// Minimal XDR writer for the SCVal shapes above.
// ---------------------------------------------------------------------------

type xdrWriter struct {
	buf []byte
}

func (w *xdrWriter) u32(v uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *xdrWriter) u64(v uint64) {
	w.buf = binary.BigEndian.AppendUint64(w.buf, v)
}

func (w *xdrWriter) fixed(b []byte) {
	w.buf = append(w.buf, b...)
}

// str writes a variable-length XDR string padded to a 4-byte boundary.
func (w *xdrWriter) str(s string) {
	w.u32(uint32(len(s)))
	w.buf = append(w.buf, s...)
	for pad := (4 - len(s)%4) % 4; pad > 0; pad-- {
		w.buf = append(w.buf, 0)
	}
}

// contractDataHeader writes LedgerEntryData{CONTRACT_DATA} up to the key:
// type, ExtensionPoint(v0) and the owning contract address.
func (w *xdrWriter) contractDataHeader(contract [32]byte) {
	w.u32(xdrLedgerEntryContractData)
	w.u32(0) // ext
	w.u32(scAddressContract)
	w.fixed(contract[:])
}

func (w *xdrWriter) symbol(s string) {
	w.u32(scvSymbol)
	w.str(s)
}

func (w *xdrWriter) scString(s string) {
	w.u32(scvString)
	w.str(s)
}

func (w *xdrWriter) scU32(v uint32) {
	w.u32(scvU32)
	w.u32(v)
}

func (w *xdrWriter) boolean(v bool) {
	w.u32(scvBool)
	if v {
		w.u32(1)
	} else {
		w.u32(0)
	}
}

func (w *xdrWriter) i128(hi int64, lo uint64) {
	w.u32(scvI128)
	w.u64(uint64(hi))
	w.u64(lo)
}

// vec writes an SCV_VEC header; the n elements follow.
func (w *xdrWriter) vec(n int) {
	w.u32(scvVec)
	w.u32(1) // optional SCVec present
	w.u32(uint32(n))
}

// mapHeader writes an SCV_MAP header; n key/value pairs follow.
func (w *xdrWriter) mapHeader(n int) {
	w.u32(scvMap)
	w.u32(1) // optional SCMap present
	w.u32(uint32(n))
}

func (w *xdrWriter) accountAddress(key [32]byte) {
	w.u32(scvAddress)
	w.u32(scAddressAccount)
	w.u32(publicKeyEd25519)
	w.fixed(key[:])
}

func (w *xdrWriter) contractAddress(id [32]byte) {
	w.u32(scvAddress)
	w.u32(scAddressContract)
	w.fixed(id[:])
}

// splitmix64 is a tiny deterministic PRNG so the corpus is reproducible.
type splitmix64 uint64

func (s *splitmix64) next() uint64 {
	*s += 0x9E3779B97F4A7C15
	z := uint64(*s)
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func (s *splitmix64) bytes32() [32]byte {
	var b [32]byte
	for i := 0; i < 32; i += 8 {
		binary.BigEndian.PutUint64(b[i:], s.next())
	}
	return b
}
