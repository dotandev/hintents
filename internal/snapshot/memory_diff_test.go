// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package snapshot

import (
	"bytes"
	"testing"
)

func TestDiffMemoryIdentical(t *testing.T) {
	a := []byte{0x00, 0x01, 0x02, 0x03}
	b := []byte{0x00, 0x01, 0x02, 0x03}

	result := DiffMemory(a, b, 16)

	if result.TotalChanged != 0 {
		t.Errorf("expected 0 changed bytes, got %d", result.TotalChanged)
	}
	if len(result.ChangedRegions) != 0 {
		t.Errorf("expected 0 regions, got %d", len(result.ChangedRegions))
	}
}

func TestDiffMemorySingleChange(t *testing.T) {
	a := []byte{0x00, 0x01, 0x02, 0x03}
	b := []byte{0x00, 0xFF, 0x02, 0x03}

	result := DiffMemory(a, b, 16)

	if result.TotalChanged != 1 {
		t.Errorf("expected 1 changed byte, got %d", result.TotalChanged)
	}
	if len(result.ChangedRegions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(result.ChangedRegions))
	}
	if result.ChangedRegions[0].Offset != 1 {
		t.Errorf("expected offset 1, got %d", result.ChangedRegions[0].Offset)
	}
}

func TestDiffMemoryMultipleRegions(t *testing.T) {
	a := make([]byte, 100)
	b := make([]byte, 100)
	copy(b, a)

	// Change bytes at positions 5 and 80 (far apart)
	b[5] = 0xFF
	b[80] = 0xAA

	result := DiffMemory(a, b, 16)

	if result.TotalChanged != 2 {
		t.Errorf("expected 2 changed bytes, got %d", result.TotalChanged)
	}
	if len(result.ChangedRegions) != 2 {
		t.Errorf("expected 2 regions due to large gap, got %d", len(result.ChangedRegions))
	}
}

func TestDiffMemoryMergeAdjacentChanges(t *testing.T) {
	a := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
	b := []byte{0xFF, 0xFF, 0x02, 0x03, 0xFF, 0xFF}

	result := DiffMemory(a, b, 16)

	if result.TotalChanged != 4 {
		t.Errorf("expected 4 changed bytes, got %d", result.TotalChanged)
	}
	// With merge threshold of 16, these should be merged into one region
	if len(result.ChangedRegions) != 1 {
		t.Errorf("expected 1 merged region, got %d", len(result.ChangedRegions))
	}
}

func TestDiffMemoryDifferentSizes(t *testing.T) {
	a := []byte{0x00, 0x01, 0x02}
	b := []byte{0x00, 0x01, 0x02, 0x03, 0x04}

	result := DiffMemory(a, b, 16)

	if result.SizeA != 3 {
		t.Errorf("expected SizeA 3, got %d", result.SizeA)
	}
	if result.SizeB != 5 {
		t.Errorf("expected SizeB 5, got %d", result.SizeB)
	}
	if result.TotalChanged != 2 {
		t.Errorf("expected 2 changed bytes (extra in B), got %d", result.TotalChanged)
	}
}

func TestDiffMemoryEmpty(t *testing.T) {
	result := DiffMemory(nil, nil, 16)

	if result.TotalChanged != 0 {
		t.Errorf("expected 0 changed bytes, got %d", result.TotalChanged)
	}
}

func TestDiffMemoryFullRange(t *testing.T) {
	a := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	b := []byte{0x00, 0xFF, 0x02, 0xFF, 0x04}

	result := DiffMemoryFull(a, b, 0, 5)

	if result.TotalChanged != 2 {
		t.Errorf("expected 2 changed bytes, got %d", result.TotalChanged)
	}
	if len(result.ChangedRegions) != 1 {
		t.Fatalf("expected 1 full region, got %d", len(result.ChangedRegions))
	}
	if result.ChangedRegions[0].Offset != 0 {
		t.Errorf("expected offset 0, got %d", result.ChangedRegions[0].Offset)
	}
	if result.ChangedRegions[0].Length != 5 {
		t.Errorf("expected length 5, got %d", result.ChangedRegions[0].Length)
	}
}

func TestDiffMemoryFullPartialRange(t *testing.T) {
	a := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	b := []byte{0x00, 0xFF, 0x02, 0xFF, 0x04}

	result := DiffMemoryFull(a, b, 1, 2)

	if len(result.ChangedRegions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(result.ChangedRegions))
	}
	if result.ChangedRegions[0].Offset != 1 {
		t.Errorf("expected offset 1, got %d", result.ChangedRegions[0].Offset)
	}
	if !bytes.Equal(result.ChangedRegions[0].BytesA, []byte{0x01, 0x02}) {
		t.Errorf("unexpected BytesA: %v", result.ChangedRegions[0].BytesA)
	}
	if !bytes.Equal(result.ChangedRegions[0].BytesB, []byte{0xFF, 0x02}) {
		t.Errorf("unexpected BytesB: %v", result.ChangedRegions[0].BytesB)
	}
}

func TestDiffMemoryFullOutOfBounds(t *testing.T) {
	a := []byte{0x00, 0x01}
	b := []byte{0x00, 0x01}

	result := DiffMemoryFull(a, b, 100, 10)

	if len(result.ChangedRegions) != 0 {
		t.Errorf("expected no regions for out of bounds, got %d", len(result.ChangedRegions))
	}
}
