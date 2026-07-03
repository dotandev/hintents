// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Note: This file is built as a shared library (cdylib) as configured in Cargo.toml.
// It provides a minimal mock of the PKCS#11 API for CI testing.

#![allow(non_camel_case_types, non_snake_case)]

//! Minimal PKCS#11 mock shared library.

// PKCS#11 minimal types as requested (u64 for simplicity)
pub type CK_RV = u64;
pub type CK_SLOT_ID = u64;
pub type CK_ULONG = u64;
pub type CK_BBOOL = u8;
pub type CK_SLOT_ID_PTR = *mut CK_SLOT_ID;
pub type CK_ULONG_PTR = *mut CK_ULONG;

// PKCS#11 constants
pub const CKR_OK: CK_RV = 0;

/// C_Initialize always returns CKR_OK (0).
/// Signature: CK_RV C_Initialize(void* pInitArgs)
///
/// # Safety
/// This function is unsafe because it dereferences raw pointers.
#[no_mangle]
pub unsafe extern "C" fn C_Initialize(_p_init_args: *mut std::ffi::c_void) -> CK_RV {
    CKR_OK
}

/// C_GetSlotList returns a single mock slot with ID 1.
/// Signature: CK_RV C_GetSlotList(CK_BBOOL tokenPresent, CK_SLOT_ID_PTR pSlotList, CK_ULONG_PTR pulCount)
///
/// # Safety
/// This function is unsafe because it dereferences raw pointers.
#[no_mangle]
pub unsafe extern "C" fn C_GetSlotList(
    _token_present: CK_BBOOL,
    p_slot_list: CK_SLOT_ID_PTR,
    pul_count: CK_ULONG_PTR,
) -> CK_RV {
    if pul_count.is_null() {
        return CKR_OK;
    }

    if p_slot_list.is_null() {
        // Caller is asking for the count of slots
        *pul_count = 1;
    } else {
        // Caller provided a buffer, fill the first entry with slot ID 1
        *p_slot_list = 1;
        *pul_count = 1;
    }

    CKR_OK
}
