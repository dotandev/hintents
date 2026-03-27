// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlignToPage(t *testing.T) {
	tests := []struct {
		input uint64
		want  uint64
	}{
		{0, WasmPageSize},
		{1, WasmPageSize},
		{WasmPageSize - 1, WasmPageSize},
		{WasmPageSize, WasmPageSize},
		{WasmPageSize + 1, WasmPageSize * 2},
		{WasmPageSize * 3, WasmPageSize * 3},
	}
	for _, tt := range tests {
		got := alignToPage(tt.input)
		assert.Equal(t, tt.want, got, "alignToPage(%d)", tt.input)
	}
}

func TestDefaultPagedMemoryConfig(t *testing.T) {
	cfg := DefaultPagedMemoryConfig()
	assert.True(t, cfg.LazyGrowth)
	assert.Equal(t, DefaultInitialMemoryBytes, cfg.InitialBytes)
	assert.Equal(t, DefaultMaxMemoryBytes, cfg.MaxBytes)
	assert.Less(t, cfg.InitialBytes, cfg.MaxBytes)
}

func TestEffectiveMemoryLimit_LazyGrowth(t *testing.T) {
	cfg := PagedMemoryConfig{
		InitialBytes: DefaultInitialMemoryBytes,
		MaxBytes:     DefaultMaxMemoryBytes,
		LazyGrowth:   true,
	}
	// No explicit limit on request — should use InitialBytes
	req := &SimulationRequest{}
	limit := effectiveMemoryLimit(cfg, req)
	assert.Equal(t, DefaultInitialMemoryBytes, limit)
}

func TestEffectiveMemoryLimit_EagerAllocation(t *testing.T) {
	cfg := PagedMemoryConfig{
		InitialBytes: DefaultInitialMemoryBytes,
		MaxBytes:     DefaultMaxMemoryBytes,
		LazyGrowth:   false,
	}
	req := &SimulationRequest{}
	limit := effectiveMemoryLimit(cfg, req)
	assert.Equal(t, DefaultMaxMemoryBytes, limit)
}

func TestEffectiveMemoryLimit_ExplicitOverride(t *testing.T) {
	cfg := DefaultPagedMemoryConfig()
	custom := uint64(2 * 1024 * 1024)
	req := &SimulationRequest{MemoryLimit: &custom}
	limit := effectiveMemoryLimit(cfg, req)
	assert.Equal(t, custom, limit)
}

func TestApplyPagedMemoryLimit_SetsLimit(t *testing.T) {
	cfg := DefaultPagedMemoryConfig()
	req := &SimulationRequest{}
	applyPagedMemoryLimit(cfg, req)
	assert.NotNil(t, req.MemoryLimit)
	assert.Equal(t, DefaultInitialMemoryBytes, *req.MemoryLimit)
}

func TestApplyPagedMemoryLimit_DoesNotOverrideExisting(t *testing.T) {
	cfg := DefaultPagedMemoryConfig()
	custom := uint64(9_999_999)
	req := &SimulationRequest{MemoryLimit: &custom}
	applyPagedMemoryLimit(cfg, req)
	assert.Equal(t, custom, *req.MemoryLimit)
}

func TestApplyPagedMemoryLimit_NilRequest(t *testing.T) {
	cfg := DefaultPagedMemoryConfig()
	// Should not panic
	applyPagedMemoryLimit(cfg, nil)
}

func TestDefaultInitialMemory_IsSmallFractionOfMax(t *testing.T) {
	// Initial allocation should be at most 1/4 of the max to be meaningful
	assert.LessOrEqual(t, DefaultInitialMemoryBytes, DefaultMaxMemoryBytes/4)
}

func TestEnvOverride_InitialBytes(t *testing.T) {
	t.Setenv("ERST_SIM_MEMORY_INITIAL_BYTES", "131072") // 2 pages
	cfg := DefaultPagedMemoryConfig()
	assert.Equal(t, uint64(131072), cfg.InitialBytes)
}

func TestEnvOverride_MaxBytes(t *testing.T) {
	t.Setenv("ERST_SIM_MEMORY_MAX_BYTES", "33554432") // 32 MiB
	cfg := DefaultPagedMemoryConfig()
	assert.Equal(t, uint64(33554432), cfg.MaxBytes)
}
