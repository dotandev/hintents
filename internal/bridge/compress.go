// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package bridge provides IPC compression helpers for ledger snapshot payloads.
// Ledger entries contain highly repetitive XDR bytes that compress extremely well
// with Zstd, reducing IPC payload size by 60-80% in practice.
package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

var (
	encoder     *zstd.Encoder
	decoder     *zstd.Decoder
	encoderOnce sync.Once
	decoderOnce sync.Once
)

func getEncoder() *zstd.Encoder {
	encoderOnce.Do(func() {
		var err error
		encoder, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
		if err != nil {
			panic(fmt.Sprintf("bridge: failed to create zstd encoder: %v", err))
		}
	})
	return encoder
}

func getDecoder() *zstd.Decoder {
	decoderOnce.Do(func() {
		var err error
		decoder, err = zstd.NewReader(nil)
		if err != nil {
			panic(fmt.Sprintf("bridge: failed to create zstd decoder: %v", err))
		}
	})
	return decoder
}

// CompressLedgerEntries serialises entries to JSON and compresses with Zstd.
// Returns the raw compressed bytes.
func CompressLedgerEntries(entries map[string]string) ([]byte, error) {
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("bridge: marshal ledger entries: %w", err)
	}
	return getEncoder().EncodeAll(raw, make([]byte, 0, len(raw)/4)), nil
}

// DecompressLedgerEntries decompresses a Zstd blob produced by CompressLedgerEntries.
func DecompressLedgerEntries(compressed []byte) (map[string]string, error) {
	raw, err := getDecoder().DecodeAll(compressed, make([]byte, 0, len(compressed)*4))
	if err != nil {
		return nil, fmt.Errorf("bridge: decompress ledger entries: %w", err)
	}
	var entries map[string]string
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&entries); err != nil {
		return nil, fmt.Errorf("bridge: unmarshal ledger entries: %w", err)
	}
	return entries, nil
}
