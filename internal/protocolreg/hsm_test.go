// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package protocolreg

import (
	"os"
	"testing"
)

// TestMockHsmIntegration verifies that the PKCS#11 mock library can be loaded
// as a native shared library and responds with the expected slot information.
//
// To run this test, build the mock library first:
//   cd simulator && cargo build --release
// Then set the environment variable:
//   export HSM_LIB_PATH=$(pwd)/simulator/target/release/libmock.so
//   go test -v ./internal/protocolreg/hsm_test.go
func TestMockHsmIntegration(t *testing.T) {
	libPath := os.Getenv("HSM_LIB_PATH")
	if libPath == "" {
		t.Skip("HSM_LIB_PATH not set, skipping PKCS#11 mock test")
	}

	// Use the cgo wrapper to load the native shared library
	lib, err := LoadHsmLib(libPath)
	if err != nil {
		t.Fatalf("failed to load library %s: %v", libPath, err)
	}
	defer lib.Close()

	if lib.initFn == nil {
		t.Fatal("symbol C_Initialize not found")
	}
	if lib.getSlotsFn == nil {
		t.Fatal("symbol C_GetSlotList not found")
	}

	// 1. Initialize PKCS#11
	rv := lib.Initialize()
	if rv != 0 {
		t.Fatalf("C_Initialize failed: 0x%x", rv)
	}

	// 2. Call C_GetSlotList to get the count
	var count uint64
	rv = lib.GetSlotList(false, nil, &count)
	if rv != 0 {
		t.Fatalf("C_GetSlotList (count) failed: 0x%x", rv)
	}

	// Assert: count == 1
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	// 3. Call C_GetSlotList to get the slot ID
	var slotID uint64
	rv = lib.GetSlotList(false, &slotID, &count)
	if rv != 0 {
		t.Fatalf("C_GetSlotList (slots) failed: 0x%x", rv)
	}

	// Assert: slot ID == 1
	if slotID != 1 {
		t.Errorf("expected slot ID 1, got %d", slotID)
	}
}
