// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//go:build unix && cgo

package protocolreg

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <stdint.h>

typedef uint64_t CK_RV;
typedef uint8_t  CK_BBOOL;
typedef uint64_t CK_SLOT_ID;
typedef uint64_t CK_ULONG;

typedef CK_RV (*C_Initialize_t)(void*);
typedef CK_RV (*C_GetSlotList_t)(CK_BBOOL, CK_SLOT_ID*, CK_ULONG*);

CK_RV call_initialize(void* f, void* args) {
	if (!f) return 0xFFFFFFFF;
	return ((C_Initialize_t)f)(args);
}

CK_RV call_get_slot_list(void* f, CK_BBOOL tokenPresent, CK_SLOT_ID* pSlotList, CK_ULONG* pulCount) {
	if (!f) return 0xFFFFFFFF;
	return ((C_GetSlotList_t)f)(tokenPresent, pSlotList, pulCount);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type HsmLib struct {
	handle     unsafe.Pointer
	initFn     unsafe.Pointer
	getSlotsFn unsafe.Pointer
}

func LoadHsmLib(path string) (*HsmLib, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.dlopen(cPath, C.RTLD_NOW)
	if handle == nil {
		return nil, fmt.Errorf("dlopen failed: %s", C.GoString(C.dlerror()))
	}

	cInitSym := C.CString("C_Initialize")
	defer C.free(unsafe.Pointer(cInitSym))
	initFn := C.dlsym(handle, cInitSym)

	cGetSlotsSym := C.CString("C_GetSlotList")
	defer C.free(unsafe.Pointer(cGetSlotsSym))
	getSlotsFn := C.dlsym(handle, cGetSlotsSym)

	return &HsmLib{
		handle:     handle,
		initFn:     initFn,
		getSlotsFn: getSlotsFn,
	}, nil
}

func (h *HsmLib) Close() {
	if h.handle != nil {
		C.dlclose(h.handle)
	}
}

func (h *HsmLib) Initialize() uint64 {
	return uint64(C.call_initialize(h.initFn, nil))
}

func (h *HsmLib) GetSlotList(tokenPresent bool, slots *uint64, count *uint64) uint64 {
	var tp C.CK_BBOOL
	if tokenPresent {
		tp = 1
	}
	return uint64(C.call_get_slot_list(h.getSlotsFn, tp, (*C.CK_SLOT_ID)(slots), (*C.CK_ULONG)(count)))
}
