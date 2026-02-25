// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWASMSourceMapper(t *testing.T) {
	tests := []struct {
		name        string
		wasmBytes   []byte
		expectError bool
		hasSymbols  bool
	}{
		{
			name:        "empty bytes",
			wasmBytes:   []byte{},
			expectError: true,
		},
		{
			name:        "valid WASM without symbols",
			wasmBytes:   []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00},
			expectError: false,
			hasSymbols:  false,
		},
		{
			name:        "minimal WASM header",
			wasmBytes:   []byte{0x00, 0x61, 0x73, 0x6d},
			expectError: false,
			hasSymbols:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper, err := NewWASMSourceMapper(tt.wasmBytes)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, mapper)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, mapper)
				assert.Equal(t, tt.hasSymbols, mapper.HasDebugSymbols())
				assert.NotEmpty(t, mapper.GetWASMHash())
				assert.Equal(t, 64, len(mapper.GetWASMHash())) // SHA256 hex = 64 chars
			}
		})
	}
}

func TestWASMSourceMapper_HasDebugSymbols(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Without actual debug symbols, should return false
	assert.False(t, mapper.HasDebugSymbols())
}

func TestWASMSourceMapper_GetWASMHash(t *testing.T) {
	wasmBytes1 := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	wasmBytes2 := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x01}

	mapper1, err := NewWASMSourceMapper(wasmBytes1)
	require.NoError(t, err)

	mapper2, err := NewWASMSourceMapper(wasmBytes2)
	require.NoError(t, err)

	// Different WASM bytes should have different hashes
	assert.NotEqual(t, mapper1.GetWASMHash(), mapper2.GetWASMHash())

	// Same WASM bytes should have same hash
	mapper3, err := NewWASMSourceMapper(wasmBytes1)
	require.NoError(t, err)
	assert.Equal(t, mapper1.GetWASMHash(), mapper3.GetWASMHash())
}

func TestWASMSourceMapper_MapOffsetToSource_NoSymbols(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Should fail when no debug symbols
	loc, err := mapper.MapOffsetToSource(0x1234)
	assert.Error(t, err)
	assert.Nil(t, loc)
	assert.Contains(t, err.Error(), "no debug symbols")
}

func TestWASMSourceMapper_GetSubprogramAt_NoSymbols(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Should fail when no debug symbols
	subprog, err := mapper.GetSubprogramAt(0x1234)
	assert.Error(t, err)
	assert.Nil(t, subprog)
	assert.Contains(t, err.Error(), "no debug symbols")
}

func TestWASMSourceMapper_GetLocalVarsAt_NoSymbols(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Should fail when no debug symbols
	vars, err := mapper.GetLocalVarsAt(0x1234)
	assert.Error(t, err)
	assert.Nil(t, vars)
	assert.Contains(t, err.Error(), "no debug symbols")
}

func TestWASMSourceMapper_GetAllSubprograms_NoSymbols(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Should fail when no debug symbols
	subprogs, err := mapper.GetAllSubprograms()
	assert.Error(t, err)
	assert.Nil(t, subprogs)
	assert.Contains(t, err.Error(), "no debug symbols")
}

func TestWASMSourceMapper_PreloadMappings_NoSymbols(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Should fail when no debug symbols
	err = mapper.PreloadMappings()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no debug symbols")
}

func TestWASMSourceMapper_GetMappingCount(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Initially should have 0 mappings
	assert.Equal(t, 0, mapper.GetMappingCount())
}

func TestWASMSourceMapper_ClearCache(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Clear cache should not error
	mapper.ClearCache()
	assert.Equal(t, 0, mapper.GetMappingCount())
}

func TestWASMSourceMapper_ThreadSafety(t *testing.T) {
	wasmBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	mapper, err := NewWASMSourceMapper(wasmBytes)
	require.NoError(t, err)

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = mapper.GetMappingCount()
			_ = mapper.HasDebugSymbols()
			_ = mapper.GetWASMHash()
			mapper.ClearCache()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or race
	assert.Equal(t, 0, mapper.GetMappingCount())
}
