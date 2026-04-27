// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Minimal PKCS#11 mock library for CI testing.
//!
//! This library exports the bare minimum functions required to simulate
//! a PKCS#11 token being present in a slot.

pub type CK_RV = u64;
pub type CK_SLOT_ID = u64;
pub type CK_ULONG = u64;

/// PKCS#11 return code for success
pub const CKR_OK: CK_RV = 0;
/// PKCS#11 return code for bad arguments
pub const CKR_ARGUMENTS_BAD: CK_RV = 7;

/// C_Initialize initializes the Cryptoki library.
///
/// Requirement: Always return CKR_OK (0).
#[no_mangle]
pub extern "C" fn C_Initialize(_p_reserved: *mut std::ffi::c_void) -> CK_RV {
    CKR_OK
}

/// C_GetSlotList obtains a list of slots in the system.
///
/// Requirement:
/// - If p_slot_list == NULL: set *pul_count = 1 and return CKR_OK.
/// - Else: set first slot to 1, set *pul_count = 1, return CKR_OK.
#[no_mangle]
pub extern "C" fn C_GetSlotList(
    _token_present: CK_ULONG,
    p_slot_list: *mut CK_SLOT_ID,
    pul_count: *mut CK_ULONG,
) -> CK_RV {
    if pul_count.is_null() {
        return CKR_ARGUMENTS_BAD;
    }

    unsafe {
        if p_slot_list.is_null() {
            // Caller wants the count of slots
            *pul_count = 1;
        } else {
            // Caller wants the slot IDs
            *p_slot_list = 1;
            *pul_count = 1;
        }
    }

    CKR_OK
}
