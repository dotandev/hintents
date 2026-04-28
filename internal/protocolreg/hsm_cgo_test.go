// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && pkcs11mock
// +build cgo,pkcs11mock

package protocolreg

/*
#include <stdlib.h>

#ifndef _WIN32
#include <dlfcn.h>
#else
#include <windows.h>
#endif

// Dynamic loading wrappers
void* load_lib(const char* path) {
#ifndef _WIN32
    return dlopen(path, RTLD_LAZY);
#else
    return (void*)LoadLibraryA(path);
#endif
}

void* get_fn(void* handle, const char* name) {
#ifndef _WIN32
    return dlsym(handle, name);
#else
    return (void*)GetProcAddress((HMODULE)handle, name);
#endif
}

void free_lib(void* handle) {
#ifndef _WIN32
    if (handle) dlclose(handle);
#else
    if (handle) FreeLibrary((HMODULE)handle);
#endif
}

// Function pointer types and wrappers for the mock
typedef unsigned long long (*C_Initialize_t)(void*);
typedef unsigned long long (*C_GetSlotList_t)(unsigned long long, unsigned long long*, unsigned long long*);

unsigned long long call_C_Initialize(void* fn, void* reserved) {
    return ((C_Initialize_t)fn)(reserved);
}

unsigned long long call_C_GetSlotList(void* fn, unsigned long long tokenPresent, unsigned long long* pSlotList, unsigned long long* pulCount) {
    return ((C_GetSlotList_t)fn)(tokenPresent, pSlotList, pulCount);
}
*/
import "C"
import (
	"runtime"
	"testing"
	"unsafe"
)

// Minimal PKCS#11 constants
const CKR_OK = 0

// TestHSMMock verifies that the minimal PKCS#11 mock library can be loaded
// and responds correctly to initialization and slot listing calls.
func TestHSMMock(t *testing.T) {
	// The mock library must be compiled as a shared object/DLL.
	libName := "./mock.so"
	if runtime.GOOS == "windows" {
		libName = "./mock.dll"
	}

	cPath := C.CString(libName)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.load_lib(cPath)
	if handle == nil {
		t.Skipf("Skipping HSM mock test: failed to load %s (ensure it is compiled and present)", libName)
	}
	defer C.free_lib(handle)

	cInitName := C.CString("C_Initialize")
	defer C.free(unsafe.Pointer(cInitName))
	fnInitialize := C.get_fn(handle, cInitName)
	if fnInitialize == nil {
		t.Fatalf("Failed to find C_Initialize")
	}

	cGetSlotName := C.CString("C_GetSlotList")
	defer C.free(unsafe.Pointer(cGetSlotName))
	fnGetSlotList := C.get_fn(handle, cGetSlotName)
	if fnGetSlotList == nil {
		t.Fatalf("Failed to find C_GetSlotList")
	}

	// 1. Test C_Initialize(NULL)
	ret := C.call_C_Initialize(fnInitialize, nil)
	if uint64(ret) != CKR_OK {
		t.Errorf("C_Initialize returned %d, want %d", ret, CKR_OK)
	}

	// 2. Test C_GetSlotList(0, NULL, &count)
	var count C.unsigned_long_long
	ret = C.call_C_GetSlotList(fnGetSlotList, 0, nil, &count)
	if uint64(ret) != CKR_OK {
		t.Errorf("C_GetSlotList (count query) returned %d, want %d", ret, CKR_OK)
	}
	if uint64(count) != 1 {
		t.Errorf("Expected 1 slot, got %d", uint64(count))
	}

	// 3. Test C_GetSlotList(0, &slot, &count)
	var slot C.unsigned_long_long
	ret = C.call_C_GetSlotList(fnGetSlotList, 0, &slot, &count)
	if uint64(ret) != CKR_OK {
		t.Errorf("C_GetSlotList (slot query) returned %d, want %d", ret, CKR_OK)
	}
	if uint64(slot) != 1 {
		t.Errorf("Expected slot ID 1, got %d", uint64(slot))
	}
}