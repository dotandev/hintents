// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package state implements transparent compression for ledger state payloads
// persisted in the local KV store.
//
// Contract data entries (token balances, allowances, instance metadata) share
// highly repetitive XDR structure: the same symbols, SCVal tags, address
// headers and contract IDs appear in almost every value. Compressing each
// value in isolation cannot exploit that redundancy because individual entries
// are only a few hundred bytes. Instead, values are compressed with a zstd
// dictionary trained on representative SEP-41 token state entries, so the
// shared structure is referenced from the dictionary rather than stored per
// value.
//
// Encoded value layout:
//
//	legacy / uncompressed text:  <raw bytes>              (first byte != 0x00)
//	compressed:                  0x00 <format> <payload>
//
// Values written before compression was introduced remain readable because
// base64 XDR and JSON never begin with a NUL byte.
package state

import (
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// DictionaryID identifies the trained SEP-41 dictionary inside zstd frames.
// Bump it whenever the training samples change so that values compressed
// with an older dictionary are rejected (and treated as cache misses) instead
// of being decoded against the wrong dictionary.
const DictionaryID uint32 = 0x45525301

// rawDictionaryID identifies the raw-content fallback dictionary used when
// entropy-table training is unavailable.
const rawDictionaryID = DictionaryID + 0x100

// maxDecodedSize bounds decompression memory to guard against zstd bombs.
const maxDecodedSize = 64 << 20

// Encoded value header bytes.
const (
	formatMagic byte = 0x00 // marks a value produced by Codec.Encode
	formatXDR   byte = 0x01 // zstd(dict) of base64-decoded XDR; re-encoded on read
	formatText  byte = 0x02 // zstd(dict) of the raw value bytes
	formatPlain byte = 0x03 // uncompressed value that itself starts with formatMagic
)

var (
	// ErrCorrupt is returned when a stored value cannot be decoded.
	ErrCorrupt = errors.New("state: corrupt compressed value")

	// ErrUnknownFormat is returned when a stored value uses an unknown format byte.
	ErrUnknownFormat = errors.New("state: unknown value format")
)

// Codec compresses and decompresses state values with a shared zstd dictionary.
// A Codec is safe for concurrent use.
type Codec struct {
	enc *zstd.Encoder
	dec *zstd.Decoder
}

var (
	defaultCodecOnce sync.Once
	defaultCodec     *Codec
	defaultCodecErr  error
)

// DefaultCodec returns the process-wide codec backed by the trained SEP-41
// dictionary. The dictionary is trained lazily on first use.
func DefaultCodec() (*Codec, error) {
	defaultCodecOnce.Do(func() {
		defaultCodec, defaultCodecErr = newSEP41Codec()
	})
	return defaultCodec, defaultCodecErr
}

// newSEP41Codec builds a codec from the trained dictionary, falling back to a
// raw-content dictionary over the same samples if training fails. The decoder
// always registers both so values written by either variant stay readable.
func newSEP41Codec() (*Codec, error) {
	history := sep41History()

	dict, err := TrainSEP41Dictionary()
	if err != nil {
		return NewRawDictCodec(rawDictionaryID, history)
	}

	return newCodec(
		[]zstd.EOption{zstd.WithEncoderDict(dict)},
		[]zstd.DOption{
			zstd.WithDecoderDicts(dict),
			zstd.WithDecoderDictRaw(rawDictionaryID, history),
		},
	)
}

// NewCodec creates a codec from a dictionary in zstd dictionary format
// (as produced by zstd.BuildDict or "zstd --train").
func NewCodec(dict []byte) (*Codec, error) {
	return newCodec(
		[]zstd.EOption{zstd.WithEncoderDict(dict)},
		[]zstd.DOption{zstd.WithDecoderDicts(dict)},
	)
}

// NewRawDictCodec creates a codec that uses arbitrary content as a raw
// (history-only) dictionary identified by id.
func NewRawDictCodec(id uint32, content []byte) (*Codec, error) {
	return newCodec(
		[]zstd.EOption{zstd.WithEncoderDictRaw(id, content)},
		[]zstd.DOption{zstd.WithDecoderDictRaw(id, content)},
	)
}

func newCodec(eopts []zstd.EOption, dopts []zstd.DOption) (*Codec, error) {
	eopts = append([]zstd.EOption{zstd.WithEncoderLevel(zstd.SpeedDefault)}, eopts...)
	enc, err := zstd.NewWriter(nil, eopts...)
	if err != nil {
		return nil, fmt.Errorf("state: init zstd encoder: %w", err)
	}

	dopts = append([]zstd.DOption{zstd.WithDecoderMaxMemory(maxDecodedSize)}, dopts...)
	dec, err := zstd.NewReader(nil, dopts...)
	if err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("state: init zstd decoder: %w", err)
	}

	return &Codec{enc: enc, dec: dec}, nil
}

// Encode compresses value for storage.
//
// Canonical base64 input (the representation used for XDR ledger entries) is
// decoded first so the dictionary matches against raw XDR structure; it is
// re-encoded on Decode. If compression does not shrink the value, the
// original bytes are returned unchanged.
func (c *Codec) Encode(value string) []byte {
	format := formatText
	payload := []byte(value)
	if raw, ok := canonicalBase64(value); ok {
		format = formatXDR
		payload = raw
	}

	compressed := c.enc.EncodeAll(payload, make([]byte, 2, 2+len(payload)/2))
	compressed[0] = formatMagic
	compressed[1] = format

	if len(compressed) < len(value) {
		return compressed
	}
	if !IsEncoded([]byte(value)) {
		return []byte(value)
	}

	out := make([]byte, 0, 2+len(value))
	out = append(out, formatMagic, formatPlain)
	return append(out, value...)
}

// Decode reverses Encode. Values that were stored without a compression
// header (including legacy values written before compression existed) are
// returned as-is.
func (c *Codec) Decode(stored []byte) (string, error) {
	if !IsEncoded(stored) {
		return string(stored), nil
	}
	if len(stored) < 2 {
		return "", ErrCorrupt
	}

	switch stored[1] {
	case formatPlain:
		return string(stored[2:]), nil
	case formatXDR, formatText:
		raw, err := c.dec.DecodeAll(stored[2:], nil)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrCorrupt, err)
		}
		if stored[1] == formatXDR {
			return base64.StdEncoding.EncodeToString(raw), nil
		}
		return string(raw), nil
	default:
		return "", fmt.Errorf("%w: 0x%02x", ErrUnknownFormat, stored[1])
	}
}

// IsEncoded reports whether stored carries a compression header written by
// Encode, as opposed to a legacy uncompressed value.
func IsEncoded(stored []byte) bool {
	return len(stored) > 0 && stored[0] == formatMagic
}

// canonicalBase64 decodes s if it is non-empty, standard-alphabet base64 that
// re-encodes to exactly the same string, guaranteeing a lossless round trip.
func canonicalBase64(s string) ([]byte, bool) {
	if s == "" || len(s)%4 != 0 {
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	if base64.StdEncoding.EncodeToString(raw) != s {
		return nil, false
	}
	return raw, true
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

// KV is the minimal byte-oriented key/value backend a Store writes through to.
type KV interface {
	Get(key string) ([]byte, bool, error)
	Put(key string, value []byte) error
}

// Store wraps a KV backend and transparently compresses values on write and
// decompresses them on read.
type Store struct {
	kv    KV
	codec *Codec
}

// NewStore returns a Store over kv. If codec is nil the default SEP-41
// dictionary codec is used.
func NewStore(kv KV, codec *Codec) (*Store, error) {
	if kv == nil {
		return nil, errors.New("state: nil KV backend")
	}
	if codec == nil {
		c, err := DefaultCodec()
		if err != nil {
			return nil, err
		}
		codec = c
	}
	return &Store{kv: kv, codec: codec}, nil
}

// Get reads and decompresses the value stored under key.
func (s *Store) Get(key string) (string, bool, error) {
	stored, ok, err := s.kv.Get(key)
	if err != nil || !ok {
		return "", ok, err
	}
	value, err := s.codec.Decode(stored)
	if err != nil {
		return "", false, fmt.Errorf("state: decode %q: %w", key, err)
	}
	return value, true, nil
}

// Put compresses value and writes it under key.
func (s *Store) Put(key, value string) error {
	return s.kv.Put(key, s.codec.Encode(value))
}
