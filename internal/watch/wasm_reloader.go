// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

type WasmFingerprint struct {
	Hash string
}

type WasmReloaderConfig struct {
	WasmPath string
	Interval time.Duration
}

type ReloadEvent struct {
	Hash string
}

func ComputeWasmFingerprint(wasmPath string, attempts int, retryDelay time.Duration) (WasmFingerprint, error) {
	if attempts <= 0 {
		attempts = 1
	}
	if retryDelay < 0 {
		retryDelay = 0
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		data, err := os.ReadFile(wasmPath)
		if err == nil {
			sum := sha256.Sum256(data)
			return WasmFingerprint{Hash: hex.EncodeToString(sum[:])}, nil
		}

		lastErr = err
		if i < attempts-1 && retryDelay > 0 {
			time.Sleep(retryDelay)
		}
	}

	return WasmFingerprint{}, fmt.Errorf("failed to fingerprint wasm %q: %w", wasmPath, lastErr)
}

func DefaultWasmReloaderConfig(wasmPath string, interval time.Duration) WasmReloaderConfig {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return WasmReloaderConfig{
		WasmPath: wasmPath,
		Interval: interval,
	}
}

func StartWasmReloader(ctx context.Context, cfg WasmReloaderConfig) (<-chan ReloadEvent, <-chan error, error) {
	if cfg.WasmPath == "" {
		return nil, nil, fmt.Errorf("wasm path is required")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 500 * time.Millisecond
	}

	initial, err := ComputeWasmFingerprint(cfg.WasmPath, 1, 0)
	if err != nil {
		return nil, nil, err
	}
	lastHash := initial.Hash

	events := make(chan ReloadEvent, 1)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fp, fpErr := ComputeWasmFingerprint(cfg.WasmPath, 1, 0)
				if fpErr != nil {
					select {
					case errs <- fpErr:
					default:
					}
					continue
				}

				if fp.Hash != lastHash {
					lastHash = fp.Hash
					select {
					case events <- ReloadEvent{Hash: fp.Hash}:
					default:
					}
				}
			}
		}
	}()

	return events, errs, nil
}
