// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package bridge

import "testing"

func TestParseFrameSnapshot_Normal(t *testing.T) {
	data := make([]byte, 1024)
	result := ParseFrameSnapshot(0, data)

	if result.Oversized {
		t.Fatal("expected non-oversized frame")
	}
	if result.Data == nil {
		t.Fatal("expected data to be set")
	}
	if result.Message != "Snapshot captured" {
		t.Fatalf("unexpected message: %s", result.Message)
	}
}

func TestParseFrameSnapshot_Oversized(t *testing.T) {
	data := make([]byte, MaxSnapshotSize+1)
	result := ParseFrameSnapshot(42, data)

	if !result.Oversized {
		t.Fatal("expected oversized frame")
	}
	if result.Data != nil {
		t.Fatal("expected data to be nil for oversized frame")
	}
	if !result.IsOversized() {
		t.Fatal("IsOversized() should return true")
	}
}

func TestParseFrameSnapshot_ExactLimit(t *testing.T) {
	data := make([]byte, MaxSnapshotSize)
	result := ParseFrameSnapshot(1, data)

	if result.Oversized {
		t.Fatal("frame at exact limit should not be oversized")
	}
}
