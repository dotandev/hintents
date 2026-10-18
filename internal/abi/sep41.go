// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
)

// The canonical SEP-41 token interface names. Keeping them as constants means
// the strings are compiled into the binary once and referenced from the
// precomputed hash table rather than re-typed at each call site.
const (
	Sep41Transfer     = "transfer"
	Sep41TransferFrom = "transfer_from"
	Sep41Approve      = "approve"
	Sep41Allowance    = "allowance"
	Sep41Balance      = "balance"
	Sep41Decimals     = "decimals"
	Sep41Name         = "name"
	Sep41Symbol       = "symbol"
	Sep41Mint         = "mint"
	Sep41Burn         = "burn"
	Sep41BurnFrom     = "burn_from"
	Sep41TotalSupply  = "total_supply"
)

// standardSep41Symbols is the source-of-truth list of SEP-41 names whose
// SHA-256 digests are computed ahead of time. It is kept sorted so the
// generated table and StandardSep41Symbols are deterministic.
var standardSep41Symbols = []string{
	Sep41Allowance,
	Sep41Approve,
	Sep41Balance,
	Sep41Burn,
	Sep41BurnFrom,
	Sep41Decimals,
	Sep41Mint,
	Sep41Name,
	Sep41Symbol,
	Sep41TotalSupply,
	Sep41Transfer,
	Sep41TransferFrom,
}

// SymbolHash is the SHA-256 digest of a contract symbol name.
type SymbolHash [32]byte

// Hex returns the lowercase hex encoding of the digest.
func (h SymbolHash) Hex() string {
	return hex.EncodeToString(h[:])
}

// String implements fmt.Stringer, returning the hex encoding.
func (h SymbolHash) String() string {
	return h.Hex()
}

// Bytes returns a copy of the digest as a byte slice, safe for the caller to
// mutate without affecting the hash value.
func (h SymbolHash) Bytes() []byte {
	out := make([]byte, len(h))
	copy(out, h[:])
	return out
}

// sep41Hashes holds the precomputed digests. It is built once, lazily and
// thread-safely on first use, then treated as read-only: readers therefore take
// no lock and the standard names never cost a hash at runtime.
var (
	sep41Once   sync.Once
	sep41Hashes map[string]SymbolHash
	sep41Names  []string
)

func buildSep41Hashes() {
	hashes := make(map[string]SymbolHash, len(standardSep41Symbols))
	for _, name := range standardSep41Symbols {
		hashes[name] = sha256.Sum256([]byte(name))
	}
	names := append([]string(nil), standardSep41Symbols...)
	sort.Strings(names)

	// Publish only after both structures are fully built so a concurrent
	// reader can never observe a partially populated table.
	sep41Names = names
	sep41Hashes = hashes
}

func ensureSep41Hashes() {
	sep41Once.Do(buildSep41Hashes)
}

// WarmSep41Hashes builds the precomputed table eagerly instead of on first
// lookup. Useful at startup to keep the first interface match off the hot path.
func WarmSep41Hashes() {
	ensureSep41Hashes()
}

// StandardSep41Symbols returns the standard SEP-41 symbol names in ascending
// order. The returned slice is a copy and safe for the caller to modify.
func StandardSep41Symbols() []string {
	ensureSep41Hashes()
	return append([]string(nil), sep41Names...)
}

// StandardSep41SymbolCount reports how many symbols have a precomputed digest.
func StandardSep41SymbolCount() int {
	ensureSep41Hashes()
	return len(sep41Hashes)
}

// LookupSep41Hash returns the precomputed digest for name. The second result is
// false when name is not a standard SEP-41 symbol.
func LookupSep41Hash(name string) (SymbolHash, bool) {
	ensureSep41Hashes()
	hash, ok := sep41Hashes[name]
	return hash, ok
}

// IsStandardSep41Symbol reports whether name has a precomputed digest. Matching
// is exact and case-sensitive, mirroring Soroban symbol comparison.
func IsStandardSep41Symbol(name string) bool {
	ensureSep41Hashes()
	_, ok := sep41Hashes[name]
	return ok
}

// Sep41Hash returns the digest for name. Standard symbols are served from the
// precomputed table; anything else is hashed on demand.
func Sep41Hash(name string) SymbolHash {
	if hash, ok := LookupSep41Hash(name); ok {
		return hash
	}
	return sha256.Sum256([]byte(name))
}

// Sep41HashHex returns the lowercase hex digest for name.
func Sep41HashHex(name string) string {
	return Sep41Hash(name).Hex()
}

// PrecomputedSep41Hashes returns a copy of the precomputed symbol → digest
// table. Mutating the returned map does not affect the shared table.
func PrecomputedSep41Hashes() map[string]SymbolHash {
	ensureSep41Hashes()
	out := make(map[string]SymbolHash, len(sep41Hashes))
	for name, hash := range sep41Hashes {
		out[name] = hash
	}
	return out
}

// SymbolHasher memoizes SHA-256 digests for symbol names that are not in the
// precomputed SEP-41 table, so a non-standard interface is hashed once per
// process instead of once per occurrence in a trace.
//
// A SymbolHasher is safe for concurrent use and its zero value is not usable;
// construct one with NewSymbolHasher.
type SymbolHasher struct {
	mu    sync.RWMutex
	cache map[string]SymbolHash
}

// NewSymbolHasher returns an empty memoizing hasher.
func NewSymbolHasher() *SymbolHasher {
	return &SymbolHasher{cache: make(map[string]SymbolHash)}
}

// Hash returns the digest for name, consulting the precomputed table first and
// the memoization cache second. Only non-standard names populate the cache.
func (h *SymbolHasher) Hash(name string) SymbolHash {
	if hash, ok := LookupSep41Hash(name); ok {
		return hash
	}
	if h == nil {
		return sha256.Sum256([]byte(name))
	}

	h.mu.RLock()
	cached, ok := h.cache[name]
	h.mu.RUnlock()
	if ok {
		return cached
	}

	sum := sha256.Sum256([]byte(name))

	h.mu.Lock()
	defer h.mu.Unlock()
	// Another goroutine may have computed the same digest while this one was
	// hashing; keep the first value so repeated calls are stable.
	if existing, ok := h.cache[name]; ok {
		return existing
	}
	h.cache[name] = sum
	return sum
}

// Len reports how many distinct non-standard symbols have been memoized.
func (h *SymbolHasher) Len() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.cache)
}

// Reset clears the memoization cache. The precomputed SEP-41 table is global
// and unaffected.
func (h *SymbolHasher) Reset() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache = make(map[string]SymbolHash)
}

// Hashes returns a copy of the memoized cache.
func (h *SymbolHasher) Hashes() map[string]SymbolHash {
	if h == nil {
		return map[string]SymbolHash{}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]SymbolHash, len(h.cache))
	for name, hash := range h.cache {
		out[name] = hash
	}
	return out
}
