// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/base64"
	"errors"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSEP41Samples_AreValidLedgerEntryData(t *testing.T) {
	samples := SEP41Samples()
	require.Len(t, samples, sep41SamplesPerKind*4)

	for i, s := range samples {
		var data xdr.LedgerEntryData
		require.NoError(t, xdr.SafeUnmarshal(s, &data), "sample %d is not valid XDR", i)
		assert.Equal(t, xdr.LedgerEntryTypeContractData, data.Type, "sample %d", i)

		// Canonical encoding: re-marshalling must reproduce the exact bytes.
		out, err := data.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, s, out, "sample %d is not canonical XDR", i)
	}
}

func TestSEP41Samples_Deterministic(t *testing.T) {
	assert.Equal(t, SEP41Samples(), SEP41Samples())
	assert.Equal(t, sep41History(), sep41History())
}

func TestTrainSEP41Dictionary(t *testing.T) {
	dict, err := TrainSEP41Dictionary()
	require.NoError(t, err)

	info, err := zstd.InspectDictionary(dict)
	require.NoError(t, err)
	assert.Equal(t, DictionaryID, info.ID())
	assert.Equal(t, len(sep41History()), info.ContentSize())

	// Training is deterministic, so every process derives the same dictionary.
	again, err := TrainSEP41Dictionary()
	require.NoError(t, err)
	assert.Equal(t, dict, again)
}

func mustDefaultCodec(t *testing.T) *Codec {
	t.Helper()
	c, err := DefaultCodec()
	require.NoError(t, err)
	return c
}

// freshBalance returns a base64 SAC balance entry not present in the training set.
func freshBalance() string {
	return base64.StdEncoding.EncodeToString(sep41Samples(0xC0FFEE, 1)[0])
}

func TestCodec_RoundTrip(t *testing.T) {
	c := mustDefaultCodec(t)

	var fresh []string
	for _, s := range sep41Samples(0xBEEF, 2) {
		fresh = append(fresh, base64.StdEncoding.EncodeToString(s))
	}

	cases := map[string]string{
		"empty":                "",
		"short text":           "hi",
		"json":                 `{"amount":"1000000","authorized":true,"clawback":false}`,
		"non-canonical base64": "YQ",
		"base64 with newline":  "AAAA\nAAAA",
		"leading NUL text":     "\x00legacy",
		"leading NUL long":     "\x00" + string(make([]byte, 512)),
		"binary garbage":       string([]byte{0xff, 0x00, 0x13, 0x37}),
	}
	for i, f := range fresh {
		cases["sep41 entry "+string(rune('a'+i))] = f
	}

	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := c.Decode(c.Encode(value))
			require.NoError(t, err)
			assert.Equal(t, value, got)
		})
	}
}

func TestCodec_CompressesSEP41EntriesWithDictionary(t *testing.T) {
	c := mustDefaultCodec(t)
	value := freshBalance()

	encoded := c.Encode(value)
	require.True(t, IsEncoded(encoded), "SEP-41 entry should be stored compressed")
	assert.Equal(t, formatXDR, encoded[1])

	plain, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	require.NoError(t, err)
	defer plain.Close()
	raw, err := base64.StdEncoding.DecodeString(value)
	require.NoError(t, err)
	noDict := plain.EncodeAll(raw, nil)

	t.Logf("base64=%dB raw=%dB zstd=%dB zstd+dict=%dB", len(value), len(raw), len(noDict), len(encoded))
	assert.Less(t, len(encoded), len(noDict), "dictionary should beat dictionary-less zstd")
	assert.Less(t, len(encoded)*2, len(value), "expected at least 2x saving over base64 storage")
}

func TestCodec_LegacyValuesPassThrough(t *testing.T) {
	c := mustDefaultCodec(t)
	legacy := freshBalance()

	got, err := c.Decode([]byte(legacy))
	require.NoError(t, err)
	assert.Equal(t, legacy, got)
	assert.False(t, IsEncoded([]byte(legacy)))
}

func TestCodec_IncompressibleValueStoredVerbatim(t *testing.T) {
	c := mustDefaultCodec(t)
	assert.Equal(t, []byte("hi"), c.Encode("hi"))
	assert.Empty(t, c.Encode(""))
}

func TestCodec_DecodeErrors(t *testing.T) {
	c := mustDefaultCodec(t)

	_, err := c.Decode([]byte{formatMagic})
	assert.ErrorIs(t, err, ErrCorrupt)

	_, err = c.Decode([]byte{formatMagic, 0x7f, 1, 2, 3})
	assert.ErrorIs(t, err, ErrUnknownFormat)

	encoded := c.Encode(freshBalance())
	corrupt := append([]byte(nil), encoded...)
	corrupt[len(corrupt)/2] ^= 0xff
	_, err = c.Decode(corrupt)
	assert.ErrorIs(t, err, ErrCorrupt)

	_, err = c.Decode(encoded[:len(encoded)-3])
	assert.ErrorIs(t, err, ErrCorrupt)
}

func TestCodec_RejectsValuesFromOtherDictionary(t *testing.T) {
	other, err := NewRawDictCodec(DictionaryID+1, []byte("some unrelated dictionary content"))
	require.NoError(t, err)

	_, err = mustDefaultCodec(t).Decode(other.Encode(freshBalance()))
	assert.ErrorIs(t, err, ErrCorrupt)
}

func TestCodec_DecodesRawFallbackDictionary(t *testing.T) {
	fallback, err := NewRawDictCodec(rawDictionaryID, sep41History())
	require.NoError(t, err)

	value := freshBalance()
	got, err := mustDefaultCodec(t).Decode(fallback.Encode(value))
	require.NoError(t, err)
	assert.Equal(t, value, got)
}

func TestCodec_ConcurrentUse(t *testing.T) {
	c := mustDefaultCodec(t)
	samples := sep41Samples(0xABCD, 8)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				v := base64.StdEncoding.EncodeToString(samples[(i+j)%len(samples)])
				got, err := c.Decode(c.Encode(v))
				if err != nil || got != v {
					t.Errorf("round trip mismatch: err=%v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

type mapKV struct {
	mu   sync.Mutex
	data map[string][]byte
	err  error
}

func newMapKV() *mapKV { return &mapKV{data: map[string][]byte{}} }

func (m *mapKV) Get(key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, false, m.err
	}
	v, ok := m.data[key]
	return v, ok, nil
}

func (m *mapKV) Put(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.data[key] = append([]byte(nil), value...)
	return nil
}

func TestStore_TransparentCompression(t *testing.T) {
	kv := newMapKV()
	s, err := NewStore(kv, nil)
	require.NoError(t, err)

	value := freshBalance()
	require.NoError(t, s.Put("balance", value))

	assert.True(t, IsEncoded(kv.data["balance"]), "value must be compressed on disk")
	assert.Less(t, len(kv.data["balance"]), len(value))

	got, ok, err := s.Get("balance")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, value, got)

	_, ok, err = s.Get("missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestStore_ReadsLegacyValues(t *testing.T) {
	kv := newMapKV()
	kv.data["old"] = []byte("legacy-plain-value")
	s, err := NewStore(kv, nil)
	require.NoError(t, err)

	got, ok, err := s.Get("old")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "legacy-plain-value", got)
}

func TestStore_Errors(t *testing.T) {
	_, err := NewStore(nil, nil)
	assert.Error(t, err)

	kv := newMapKV()
	s, err := NewStore(kv, nil)
	require.NoError(t, err)

	kv.data["bad"] = []byte{formatMagic, formatXDR, 0xde, 0xad}
	_, ok, err := s.Get("bad")
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrCorrupt)

	boom := errors.New("disk full")
	kv.err = boom
	assert.ErrorIs(t, s.Put("k", "v"), boom)
	_, _, err = s.Get("k")
	assert.ErrorIs(t, err, boom)
}

func BenchmarkCodec_EncodeSEP41(b *testing.B) {
	c, err := DefaultCodec()
	require.NoError(b, err)
	value := freshBalance()
	b.SetBytes(int64(len(value)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Encode(value)
	}
}

func BenchmarkCodec_DecodeSEP41(b *testing.B) {
	c, err := DefaultCodec()
	require.NoError(b, err)
	encoded := c.Encode(freshBalance())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Decode(encoded); err != nil {
			b.Fatal(err)
		}
	}
}
