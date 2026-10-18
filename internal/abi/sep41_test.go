// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package abi

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A few digests computed independently (sha256sum over the raw UTF-8 name) so a
// mistaken implementation — a different algorithm, a length prefix, upper-casing —
// fails loudly instead of agreeing with itself.
var referenceSep41Digests = map[string]string{
	Sep41Transfer:     "27f576cafbb263ed44be8bd094f66114da26877706f96c4c31d5a97ffebf2e29",
	Sep41Balance:      "5751e048eb6fee3eb9bb4ea70f107f19c35863766bee5100044e52ab60d9edcf",
	Sep41Approve:      "74e21680eac7385ca408cb01878465fd693b37e58eb0c0c32663a2d8f15d8136",
	Sep41TransferFrom: "86c8ccd7f956f63fd06cbdc8b089b9edab0ee86d7761e6317515d96efba5ffe7",
	Sep41Allowance:    "32f09fa1e0d7bc30389024258d4f02ad7d67bf49575be54c00e7ceb333a8079f",
	Sep41Symbol:       "b76a7ca153c24671658335bbd08946350ffc621fa1c516e7123095d4ffd5c581",
}

func TestStandardSep41Symbols_AreCompleteSortedAndUnique(t *testing.T) {
	expected := []string{
		"allowance", "approve", "balance", "burn", "burn_from", "decimals",
		"mint", "name", "symbol", "total_supply", "transfer", "transfer_from",
	}

	got := StandardSep41Symbols()

	assert.Equal(t, expected, got, "the standard set must stay sorted and complete")
	assert.True(t, sort.StringsAreSorted(got), "symbols must be returned in ascending order")
	assert.Len(t, got, len(standardSep41Symbols), "no duplicates in the source list")
	assert.Equal(t, len(got), StandardSep41SymbolCount())
}

func TestSep41SymbolConstants_MatchTheirNames(t *testing.T) {
	// Guards against a rename drifting away from the on-chain symbol string.
	pairs := map[string]string{
		Sep41Transfer:     "transfer",
		Sep41TransferFrom: "transfer_from",
		Sep41Approve:      "approve",
		Sep41Allowance:    "allowance",
		Sep41Balance:      "balance",
		Sep41Decimals:     "decimals",
		Sep41Name:         "name",
		Sep41Symbol:       "symbol",
		Sep41Mint:         "mint",
		Sep41Burn:         "burn",
		Sep41BurnFrom:     "burn_from",
		Sep41TotalSupply:  "total_supply",
	}
	for constant, literal := range pairs {
		assert.Equal(t, literal, constant)
	}
	assert.Len(t, pairs, len(standardSep41Symbols), "every constant is in the precomputed list")
}

func TestLookupSep41Hash_MatchesSHA256ForEveryStandardSymbol(t *testing.T) {
	for _, name := range StandardSep41Symbols() {
		expected := sha256Of(name)

		hash, ok := LookupSep41Hash(name)

		require.True(t, ok, "expected %q to be precomputed", name)
		assert.Equal(t, expected, hash)
		assert.Equal(t, hex.EncodeToString(expected[:]), hash.Hex())
	}
}

func TestLookupSep41Hash_MatchesIndependentlyComputedDigests(t *testing.T) {
	for name, expectedHex := range referenceSep41Digests {
		hash, ok := LookupSep41Hash(name)
		require.True(t, ok)
		assert.Equal(t, expectedHex, hash.Hex(), "digest for %q is wrong", name)
	}
}

func TestLookupSep41Hash_UnknownSymbolIsNotPrecomputed(t *testing.T) {
	hash, ok := LookupSep41Hash("custom_interface_method")

	assert.False(t, ok)
	assert.Equal(t, SymbolHash{}, hash)
}

func TestIsStandardSep41Symbol(t *testing.T) {
	assert.True(t, IsStandardSep41Symbol(Sep41Transfer))
	assert.True(t, IsStandardSep41Symbol("total_supply"))
	assert.False(t, IsStandardSep41Symbol(""))
	assert.False(t, IsStandardSep41Symbol("Transfer"), "matching is case-sensitive")
	assert.False(t, IsStandardSep41Symbol("TRANSFER"))
	assert.False(t, IsStandardSep41Symbol("transfer "))
	assert.False(t, IsStandardSep41Symbol("transfer_"))
}

func TestSep41Hash_FallsBackToSHA256ForCustomSymbols(t *testing.T) {
	custom := "my_protocol_settle"
	expected := sha256Of(custom)

	assert.Equal(t, expected, Sep41Hash(custom))
	assert.Equal(t, Sep41Hash(custom), Sep41Hash(custom), "fallback is deterministic")
	assert.False(t, IsStandardSep41Symbol(custom))
}

func TestSep41Hash_PrefersPrecomputedTableForStandardSymbols(t *testing.T) {
	hash, ok := LookupSep41Hash(Sep41Balance)
	require.True(t, ok)

	assert.Equal(t, hash, Sep41Hash(Sep41Balance))
}

func TestSep41HashHex_IsLowercaseHexOfTheDigest(t *testing.T) {
	for _, name := range []string{Sep41Transfer, "not_standard_symbol"} {
		expected := sha256Of(name)

		got := Sep41HashHex(name)

		assert.Len(t, got, 64)
		assert.Equal(t, hex.EncodeToString(expected[:]), got)
		assert.Equal(t, got, Sep41Hash(name).Hex())
		assert.Equal(t, got, Sep41Hash(name).String())
	}
}

func TestStandardSep41Symbols_ReturnsAnIndependentCopy(t *testing.T) {
	first := StandardSep41Symbols()
	first[0] = "clobbered"

	second := StandardSep41Symbols()

	assert.NotEqual(t, "clobbered", second[0], "the shared table must not be mutable through the accessor")
	assert.Equal(t, "allowance", second[0])
}

func TestPrecomputedSep41Hashes_ReturnsAnIndependentCopy(t *testing.T) {
	table := PrecomputedSep41Hashes()
	require.Contains(t, table, Sep41Transfer)

	table[Sep41Transfer] = SymbolHash{}
	table["injected"] = SymbolHash{}

	fresh := PrecomputedSep41Hashes()

	assert.NotEqual(t, SymbolHash{}, fresh[Sep41Transfer], "the shared table must not be mutable through the accessor")
	assert.NotContains(t, fresh, "injected")
	assert.Len(t, fresh, StandardSep41SymbolCount())
}

func TestSymbolHash_BytesIsACopy(t *testing.T) {
	hash := Sep41Hash(Sep41Transfer)

	bytes := hash.Bytes()
	require.Len(t, bytes, 32)
	bytes[0] ^= 0xff

	assert.Equal(t, Sep41Hash(Sep41Transfer), hash, "mutating Bytes must not change the digest")
}

func TestWarmSep41Hashes_IsIdempotent(t *testing.T) {
	before := StandardSep41SymbolCount()

	WarmSep41Hashes()
	WarmSep41Hashes()

	assert.Equal(t, before, StandardSep41SymbolCount())
}

func TestSymbolHasher_MemoizesOnlyNonStandardSymbols(t *testing.T) {
	hasher := NewSymbolHasher()

	assert.Equal(t, 0, hasher.Len())

	first := hasher.Hash("custom_a")
	assert.Equal(t, 1, hasher.Len())

	second := hasher.Hash("custom_a")
	assert.Equal(t, first, second)
	assert.Equal(t, 1, hasher.Len(), "repeat lookups must not grow the cache")

	hasher.Hash("custom_b")
	assert.Equal(t, 2, hasher.Len())

	// Standard symbols come from the global table and must not be cached.
	standard := hasher.Hash(Sep41Transfer)
	assert.Equal(t, Sep41Hash(Sep41Transfer), standard)
	assert.Equal(t, 2, hasher.Len(), "standard symbols must not populate the memo cache")
}

func TestSymbolHasher_MatchesDirectSHA256(t *testing.T) {
	hasher := NewSymbolHasher()

	for _, name := range []string{"settle", "custom_symbol", Sep41Mint, "another_custom"} {
		assert.Equal(t, sha256Of(name), hasher.Hash(name), "name %q", name)
	}
}

func TestSymbolHasher_ResetClearsOnlyItsOwnCache(t *testing.T) {
	hasher := NewSymbolHasher()
	hasher.Hash("custom_a")
	hasher.Hash("custom_b")
	require.Equal(t, 2, hasher.Len())

	hasher.Reset()

	assert.Equal(t, 0, hasher.Len())
	assert.Empty(t, hasher.Hashes())
	assert.Equal(t, Sep41Hash(Sep41Transfer), hasher.Hash(Sep41Transfer), "global table still works")
}

func TestSymbolHasher_HashesReturnsACopy(t *testing.T) {
	hasher := NewSymbolHasher()
	hasher.Hash("custom_a")

	snapshot := hasher.Hashes()
	snapshot["custom_a"] = SymbolHash{}

	assert.Equal(t, sha256Of("custom_a"), hasher.Hash("custom_a"))
}

func TestSymbolHasher_NilReceiverFallsBackToHashing(t *testing.T) {
	var hasher *SymbolHasher

	assert.Equal(t, sha256Of("custom"), hasher.Hash("custom"))
	assert.Equal(t, Sep41Hash(Sep41Transfer), hasher.Hash(Sep41Transfer))
	assert.Equal(t, 0, hasher.Len())
	assert.Empty(t, hasher.Hashes())
	assert.NotPanics(t, hasher.Reset)
}

func TestSymbolHasher_ConcurrentUseIsConsistent(t *testing.T) {
	hasher := NewSymbolHasher()
	names := []string{"alpha", "beta", Sep41Transfer, "gamma", "beta", Sep41Balance}

	const goroutines = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for _, name := range names {
				assert.Equal(t, sha256Of(name), hasher.Hash(name))
			}
		}()
	}

	wg.Wait()

	// Only the three non-standard names get memoized.
	assert.Equal(t, 3, hasher.Len())
	assert.ElementsMatch(t, []string{"alpha", "beta", "gamma"}, sortedKeys(hasher.Hashes()))
}

func TestSep41Hash_AllStandardSymbolsAreDistinct(t *testing.T) {
	seen := make(map[SymbolHash]string, StandardSep41SymbolCount())

	for _, name := range StandardSep41Symbols() {
		hash := Sep41Hash(name)
		other, clash := seen[hash]
		require.False(t, clash, "%q and %q hash to the same digest", name, other)
		seen[hash] = name
	}
}

// ── test helpers ──────────────────────────────────────────────────────────────

// sha256Of returns the digest as the package's named type so assertions compare
// like with like instead of failing on the named/unnamed type difference.
func sha256Of(name string) SymbolHash {
	return SymbolHash(sha256.Sum256([]byte(name)))
}

func sortedKeys(m map[string]SymbolHash) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ── benchmarks ────────────────────────────────────────────────────────────────

func BenchmarkSep41Hash_PrecomputedStandard(b *testing.B) {
	b.ReportAllocs()
	var sink SymbolHash

	for i := 0; i < b.N; i++ {
		sink = Sep41Hash(Sep41Transfer)
	}
	_ = sink
}

func BenchmarkSep41Hash_UncachedHash(b *testing.B) {
	// Baseline: hashing the same name with no precomputation or memoization.
	b.ReportAllocs()
	var sink SymbolHash

	for i := 0; i < b.N; i++ {
		sink = sha256.Sum256([]byte(Sep41Transfer))
	}
	_ = sink
}

func BenchmarkSymbolHasher_MemoizedCustom(b *testing.B) {
	hasher := NewSymbolHasher()
	hasher.Hash("custom_interface")
	b.ReportAllocs()
	var sink SymbolHash

	for i := 0; i < b.N; i++ {
		sink = hasher.Hash("custom_interface")
	}
	_ = sink
}

func BenchmarkSymbolHasher_UncachedCustom(b *testing.B) {
	// Baseline: hashing a non-standard name with no memoization, as happens on
	// every occurrence of the symbol in a trace today.
	b.ReportAllocs()
	var sink SymbolHash

	for i := 0; i < b.N; i++ {
		sink = sha256.Sum256([]byte("custom_interface"))
	}
	_ = sink
}

func BenchmarkSymbolHasher_ConcurrentReads(b *testing.B) {
	hasher := NewSymbolHasher()
	hasher.Hash("custom_interface")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var sink SymbolHash
		for pb.Next() {
			sink = hasher.Hash("custom_interface")
		}
		_ = sink
	})
}
