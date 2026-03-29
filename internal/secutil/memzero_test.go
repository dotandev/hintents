// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package secutil

import "testing"

func TestMemzero(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	Memzero(data)
	for i, b := range data {
		if b != 0 {
			t.Errorf("index %d: got %d, want 0", i, b)
		}
	}
}

func TestMemzeroEmpty(t *testing.T) {
	Memzero([]byte{})
}

func TestMemzeroNil(t *testing.T) {
	Memzero(nil)
}

func TestMemzeroRetainsLength(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	Memzero(data)
	if len(data) != 4 {
		t.Errorf("slice length changed: got %d, want 4", len(data))
	}
}
