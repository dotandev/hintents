// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package snapshot

// MemoryRegion represents a contiguous region of changed bytes.
type MemoryRegion struct {
	Offset int
	Length int
	BytesA []byte
	BytesB []byte
}

// MemoryDiffResult holds the comparison result between two memory dumps.
type MemoryDiffResult struct {
	SizeA          int
	SizeB          int
	TotalChanged   int
	ChangedRegions []MemoryRegion
}

// DiffMemory compares two byte slices and returns regions that differ.
// It uses a chunked approach for efficiency with large memory segments.
// Adjacent changed bytes within mergeThreshold are merged into single regions.
func DiffMemory(a, b []byte, mergeThreshold int) *MemoryDiffResult {
	if mergeThreshold <= 0 {
		mergeThreshold = 16
	}

	result := &MemoryDiffResult{
		SizeA: len(a),
		SizeB: len(b),
	}

	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	if maxLen == 0 {
		return result
	}

	var regions []MemoryRegion
	var currentRegion *MemoryRegion
	gapCount := 0

	for i := 0; i < maxLen; i++ {
		var byteA, byteB byte
		var aValid, bValid bool

		if i < len(a) {
			byteA = a[i]
			aValid = true
		}
		if i < len(b) {
			byteB = b[i]
			bValid = true
		}

		differs := (aValid != bValid) || (byteA != byteB)

		if differs {
			result.TotalChanged++

			if currentRegion == nil {
				currentRegion = &MemoryRegion{
					Offset: i,
					BytesA: []byte{},
					BytesB: []byte{},
				}
			} else if gapCount > 0 {
				// Fill gap with unchanged bytes to maintain continuous region
				for j := i - gapCount; j < i; j++ {
					if j < len(a) {
						currentRegion.BytesA = append(currentRegion.BytesA, a[j])
					} else {
						currentRegion.BytesA = append(currentRegion.BytesA, 0)
					}
					if j < len(b) {
						currentRegion.BytesB = append(currentRegion.BytesB, b[j])
					} else {
						currentRegion.BytesB = append(currentRegion.BytesB, 0)
					}
				}
			}

			if aValid {
				currentRegion.BytesA = append(currentRegion.BytesA, byteA)
			} else {
				currentRegion.BytesA = append(currentRegion.BytesA, 0)
			}
			if bValid {
				currentRegion.BytesB = append(currentRegion.BytesB, byteB)
			} else {
				currentRegion.BytesB = append(currentRegion.BytesB, 0)
			}

			currentRegion.Length = len(currentRegion.BytesA)
			gapCount = 0
		} else if currentRegion != nil {
			gapCount++
			if gapCount > mergeThreshold {
				regions = append(regions, *currentRegion)
				currentRegion = nil
				gapCount = 0
			}
		}
	}

	if currentRegion != nil {
		regions = append(regions, *currentRegion)
	}

	result.ChangedRegions = regions
	return result
}

// DiffMemoryFull compares two byte slices and returns byte-by-byte differences
// for a specified range. Used for detailed inspection of specific offsets.
func DiffMemoryFull(a, b []byte, offset, length int) *MemoryDiffResult {
	if offset < 0 {
		offset = 0
	}

	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	if offset >= maxLen {
		return &MemoryDiffResult{SizeA: len(a), SizeB: len(b)}
	}

	end := offset + length
	if end > maxLen {
		end = maxLen
	}

	var sliceA, sliceB []byte
	if offset < len(a) {
		endA := end
		if endA > len(a) {
			endA = len(a)
		}
		sliceA = a[offset:endA]
	}
	if offset < len(b) {
		endB := end
		if endB > len(b) {
			endB = len(b)
		}
		sliceB = b[offset:endB]
	}

	// Pad to equal length
	regionLen := end - offset
	paddedA := make([]byte, regionLen)
	paddedB := make([]byte, regionLen)
	copy(paddedA, sliceA)
	copy(paddedB, sliceB)

	totalChanged := 0
	for i := 0; i < regionLen; i++ {
		if paddedA[i] != paddedB[i] {
			totalChanged++
		}
	}

	return &MemoryDiffResult{
		SizeA:        len(a),
		SizeB:        len(b),
		TotalChanged: totalChanged,
		ChangedRegions: []MemoryRegion{
			{
				Offset: offset,
				Length: regionLen,
				BytesA: paddedA,
				BytesB: paddedB,
			},
		},
	}
}
