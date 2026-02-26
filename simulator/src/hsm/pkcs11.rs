// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! PKCS#11 HSM signer implementation for hardware-backed cryptographic operations.

use super::{PublicKey, Signature, Signer, SignerError, SignerInfo, Pkcs11SignerConfig};
use async_trait::async_trait;
use libloading::{Library, Symbol};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_ulong, c_void};
use std::ptr;
use std::sync::Arc;

// PKCS#11 constants and types
const CKF_OS_LOCKING_OK: c_ulong = 0x1;
const CKF_SERIAL_SESSION: c_ulong = 0x4;
const CKF_RW_SESSION: c_ulong = 0x2;
const CKU_USER: c_ulong = 1;
const CKO_PRIVATE_KEY: c_ulong = 0x3;
const CKO_PUBLIC_KEY: c_ulong = 0x2;
const CKK_EC: c_ulong = 0x3;
const CKK_ECDSA: c_ulong = 0x3;
const CKK_EDDSA: c_ulong = 0x42;
const CKA_CLASS: c_ulong = 0x0;
const CKA_KEY_TYPE: c_ulong = 0x100;
const CKA_LABEL: c_ulong = 0x3;
const CKA_ID: c_ulong = 0x102;
const CKA_EC_PARAMS: c_ulong = 0x180;
const CKA_EC_POINT: c_ulong = 0x181;
const CKM_ECDSA: c_ulong = 0x1041;
const CKM_EDDSA: c_ulong = 0x1050;
const CKR_OK: c_ulong = 0x0;
const CKR_BUFFER_TOO_SMALL: c_ulong = 0x150;
const CKR_FUNCTION_FAILED: c_ulong = 0x6;

// PKCS#11 types
#[repr(C)]
#[derive(Debug, Clone)]
pub struct CK_VERSION {
    pub major: u8,
    pub minor: u8,
}

#[repr(C)]
#[derive(Debug, Clone)]
pub struct CK_INFO {
    pub manufacturer_id: [c_char; 32],
    pub flags: c_ulong,
    pub library_description: [c_char; 32],
    pub library_version: CK_VERSION,
}

#[repr(C)]
#[derive(Debug, Clone)]
pub struct CK_SLOT_INFO {
    pub slot_description: [c_char; 64],
    pub manufacturer_id: [c_char; 32],
    pub flags: c_ulong,
    pub hardware_version: CK_VERSION,
    pub firmware_version: CK_VERSION,
}

#[repr(C)]
#[derive(Debug, Clone)]
pub struct CK_TOKEN_INFO {
    pub label: [c_char; 32],
    pub manufacturer_id: [c_char; 32],
    pub model: [c_char; 16],
    pub serial_number: [c_char; 16],
    pub flags: c_ulong,
    pub ul_max_session_count: c_ulong,
    pub ul_session_count: c_ulong,
    pub ul_max_rw_session_count: c_ulong,
    pub ul_rw_session_count: c_ulong,
    pub ul_max_pin_len: c_ulong,
    pub ul_min_pin_len: c_ulong,
    pub ul_total_public_memory: c_ulong,
    pub ul_free_public_memory: c_ulong,
    pub ul_total_private_memory: c_ulong,
    pub ul_free_private_memory: c_ulong,
    pub hardware_version: CK_VERSION,
    pub firmware_version: CK_VERSION,
    pub utc_time: [c_char; 16],
}

#[repr(C)]
#[derive(Debug, Clone)]
pub struct CK_ATTRIBUTE {
    pub type_: c_ulong,
    pub p_value: *mut c_void,
    pub ul_value_len: c_ulong,
}

#[repr(C)]
#[derive(Debug, Clone)]
pub struct CK_MECHANISM {
    pub mechanism: c_ulong,
    pub p_parameter: *mut c_void,
    pub ul_parameter_len: c_ulong,
}

// PKCS#11 function types
type C_InitializeFn = unsafe extern "C" fn(pInitArgs: *mut c_void) -> c_ulong;
type C_FinalizeFn = unsafe extern "C" fn(pReserved: *mut c_void) -> c_ulong;
type C_GetInfoFn = unsafe extern "C" fn(pInfo: *mut CK_INFO) -> c_ulong;
type C_GetSlotListFn = unsafe extern "C" fn(bTokenPresent: bool, pSlotList: *mut c_ulong, pulCount: *mut c_ulong) -> c_ulong;
type C_GetSlotInfoFn = unsafe extern "C" fn(slotID: c_ulong, pInfo: *mut CK_SLOT_INFO) -> c_ulong;
type C_GetTokenInfoFn = unsafe extern "C" fn(slotID: c_ulong, pInfo: *mut CK_TOKEN_INFO) -> c_ulong;
type C_OpenSessionFn = unsafe extern "C" fn(slotID: c_ulong, flags: c_ulong, phSession: *mut c_ulong) -> c_ulong;
type C_CloseSessionFn = unsafe extern "C" fn(hSession: c_ulong) -> c_ulong;
type C_LoginFn = unsafe extern "C" fn(hSession: c_ulong, userType: c_ulong, pPin: *mut c_char) -> c_ulong;
type C_LogoutFn = unsafe extern "C" fn(hSession: c_ulong) -> c_ulong;
type C_FindObjectsInitFn = unsafe extern "C" fn(hSession: c_ulong, pTemplate: *mut CK_ATTRIBUTE, ulCount: c_ulong) -> c_ulong;
type C_FindObjectsFn = unsafe extern "C" fn(hSession: c_ulong, phObject: *mut c_ulong, ulMaxObjectCount: c_ulong, pulObjectCount: *mut c_ulong) -> c_ulong;
type C_FindObjectsFinalFn = unsafe extern "C" fn(hSession: c_ulong) -> c_ulong;
type C_SignInitFn = unsafe extern "C" fn(hSession: c_ulong, pMechanism: *mut CK_MECHANISM, hKey: c_ulong) -> c_ulong;
type C_SignFn = unsafe extern "C" fn(hSession: c_ulong, pData: *mut u8, ulDataLen: c_ulong, pSignature: *mut u8, pulSignatureLen: *mut c_ulong) -> c_ulong;
type C_GetAttributeValueFn = unsafe extern "C" fn(hSession: c_ulong, hObject: c_ulong, pTemplate: *mut CK_ATTRIBUTE, ulCount: c_ulong) -> c_ulong;

/// PKCS#11 HSM signer implementation
pub struct Pkcs11Signer {
    config: Pkcs11SignerConfig,
    library: Arc<Library>,
    algorithm: String,
}

impl Pkcs11Signer {
    /// Create a new PKCS#11 signer from configuration
    pub async fn from_config(config: Pkcs11SignerConfig) -> Result<Self, SignerError> {
        let library = unsafe { Library::new(&config.module_path) }
            .map_err(|e| SignerError::Pkcs11(format!("Failed to load PKCS#11 module: {}", e)))?;

        let algorithm = if config.module_path.to_lowercase().contains("yubikey") {
            "ed25519".to_string()
        } else {
            "ed25519".to_string() // Default to Ed25519
        };

        Ok(Self {
            config,
            library: Arc::new(library),
            algorithm,
        })
    }

    /// Load PKCS#11 functions from the library
    unsafe fn load_functions(&self) -> Result<Pkcs11Functions, SignerError> {
        let lib = &self.library;
        
        Ok(Pkcs11Functions {
            C_Initialize: *lib.get(b"C_Initialize\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_Initialize: {}", e)))?,
            C_Finalize: *lib.get(b"C_Finalize\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_Finalize: {}", e)))?,
            C_GetInfo: *lib.get(b"C_GetInfo\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_GetInfo: {}", e)))?,
            C_GetSlotList: *lib.get(b"C_GetSlotList\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_GetSlotList: {}", e)))?,
            C_GetSlotInfo: *lib.get(b"C_GetSlotInfo\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_GetSlotInfo: {}", e)))?,
            C_GetTokenInfo: *lib.get(b"C_GetTokenInfo\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_GetTokenInfo: {}", e)))?,
            C_OpenSession: *lib.get(b"C_OpenSession\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_OpenSession: {}", e)))?,
            C_CloseSession: *lib.get(b"C_CloseSession\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_CloseSession: {}", e)))?,
            C_Login: *lib.get(b"C_Login\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_Login: {}", e)))?,
            C_Logout: *lib.get(b"C_Logout\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_Logout: {}", e)))?,
            C_FindObjectsInit: *lib.get(b"C_FindObjectsInit\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_FindObjectsInit: {}", e)))?,
            C_FindObjects: *lib.get(b"C_FindObjects\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_FindObjects: {}", e)))?,
            C_FindObjectsFinal: *lib.get(b"C_FindObjectsFinal\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_FindObjectsFinal: {}", e)))?,
            C_SignInit: *lib.get(b"C_SignInit\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_SignInit: {}", e)))?,
            C_Sign: *lib.get(b"C_Sign\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_Sign: {}", e)))?,
            C_GetAttributeValue: *lib.get(b"C_GetAttributeValue\0").map_err(|e| SignerError::Pkcs11(format!("Failed to load C_GetAttributeValue: {}", e)))?,
        })
    }

    /// Find the appropriate slot based on configuration
    fn find_slot(&self, functions: &Pkcs11Functions) -> Result<c_ulong, SignerError> {
        unsafe {
            // Get slot list
            let mut slot_count: c_ulong = 0;
            let result = (functions.C_GetSlotList)(true, ptr::null_mut(), &mut slot_count);
            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to get slot count: 0x{:x}", result)));
            }

            let mut slots = vec![0u64; slot_count as usize];
            let result = (functions.C_GetSlotList)(true, slots.as_mut_ptr(), &mut slot_count);
            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to get slot list: 0x{:x}", result)));
            }

            // Find the appropriate slot
            if let Some(slot_index) = self.config.slot_index {
                if slot_index as usize >= slots.len() {
                    return Err(SignerError::Config(format!("Slot index {} out of range", slot_index)));
                }
                return Ok(slots[slot_index as usize]);
            }

            if let Some(ref token_label) = self.config.token_label {
                let label_cstr = CString::new(token_label.as_str()).unwrap();
                
                for &slot in &slots {
                    let mut token_info = CK_TOKEN_INFO {
                        label: [0; 32],
                        manufacturer_id: [0; 32],
                        model: [0; 16],
                        serial_number: [0; 16],
                        flags: 0,
                        ul_max_session_count: 0,
                        ul_session_count: 0,
                        ul_max_rw_session_count: 0,
                        ul_rw_session_count: 0,
                        ul_max_pin_len: 0,
                        ul_min_pin_len: 0,
                        ul_total_public_memory: 0,
                        ul_free_public_memory: 0,
                        ul_total_private_memory: 0,
                        ul_free_private_memory: 0,
                        hardware_version: CK_VERSION { major: 0, minor: 0 },
                        firmware_version: CK_VERSION { major: 0, minor: 0 },
                        utc_time: [0; 16],
                    };

                    let result = (functions.C_GetTokenInfo)(slot, &mut token_info);
                    if result == CKR_OK {
                        let token_label_str = CStr::from_ptr(token_info.label.as_ptr()).to_string_lossy();
                        if token_label_str.trim_matches('\0') == token_label {
                            return Ok(slot);
                        }
                    }
                }
                
                return Err(SignerError::Config(format!("Token with label '{}' not found", token_label)));
            }

            // Use first slot if no specific configuration
            if slots.is_empty() {
                return Err(SignerError::Pkcs11("No slots available".to_string()));
            }
            
            Ok(slots[0])
        }
    }

    /// Find the private key in the HSM
    fn find_private_key(&self, functions: &Pkcs11Functions, session: c_ulong) -> Result<c_ulong, SignerError> {
        unsafe {
            let mut template = vec![
                CK_ATTRIBUTE {
                    type_: CKA_CLASS,
                    p_value: &mut CKO_PRIVATE_KEY as *mut _ as *mut c_void,
                    ul_value_len: std::mem::size_of::<c_ulong>() as c_ulong,
                },
            ];

            // Add key identifier if specified
            if let Some(ref key_label) = self.config.key_label {
                let label_cstr = CString::new(key_label.as_str()).unwrap();
                template.push(CK_ATTRIBUTE {
                    type_: CKA_LABEL,
                    p_value: label_cstr.as_ptr() as *mut c_void,
                    ul_value_len: key_label.len() as c_ulong,
                });
            }

            if let Some(ref key_id_hex) = self.config.key_id_hex {
                let key_id_bytes = hex::decode(key_id_hex)
                    .map_err(|e| SignerError::Config(format!("Invalid key ID hex: {}", e)))?;
                template.push(CK_ATTRIBUTE {
                    type_: CKA_ID,
                    p_value: key_id_bytes.as_ptr() as *mut c_void,
                    ul_value_len: key_id_bytes.len() as c_ulong,
                });
            }

            let result = (functions.C_FindObjectsInit)(session, template.as_mut_ptr(), template.len() as c_ulong);
            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to initialize key search: 0x{:x}", result)));
            }

            let mut key_handle: c_ulong = 0;
            let mut object_count: c_ulong = 0;
            let result = (functions.C_FindObjects)(session, &mut key_handle, 1, &mut object_count);
            
            // Always finalize the search
            (functions.C_FindObjectsFinal)(session);

            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to find key: 0x{:x}", result)));
            }

            if object_count == 0 {
                return Err(SignerError::KeyNotFound("Private key not found in HSM".to_string()));
            }

            Ok(key_handle)
        }
    }

    /// Get public key from HSM
    fn get_public_key(&self, functions: &Pkcs11Functions, session: c_ulong) -> Result<PublicKey, SignerError> {
        unsafe {
            // If public key is provided in config, use it
            if let Some(ref pem_data) = self.config.public_key_pem {
                let spki_bytes = pem_data.as_bytes().to_vec();
                return Ok(PublicKey {
                    algorithm: self.algorithm.clone(),
                    spki_bytes,
                });
            }

            // Otherwise, extract public key from HSM
            let mut template = vec![
                CK_ATTRIBUTE {
                    type_: CKA_CLASS,
                    p_value: &mut CKO_PUBLIC_KEY as *mut _ as *mut c_void,
                    ul_value_len: std::mem::size_of::<c_ulong>() as c_ulong,
                },
            ];

            // Add key identifier if specified
            if let Some(ref key_label) = self.config.key_label {
                let label_cstr = CString::new(key_label.as_str()).unwrap();
                template.push(CK_ATTRIBUTE {
                    type_: CKA_LABEL,
                    p_value: label_cstr.as_ptr() as *mut c_void,
                    ul_value_len: key_label.len() as c_ulong,
                });
            }

            if let Some(ref key_id_hex) = self.config.key_id_hex {
                let key_id_bytes = hex::decode(key_id_hex)
                    .map_err(|e| SignerError::Config(format!("Invalid key ID hex: {}", e)))?;
                template.push(CK_ATTRIBUTE {
                    type_: CKA_ID,
                    p_value: key_id_bytes.as_ptr() as *mut c_void,
                    ul_value_len: key_id_bytes.len() as c_ulong,
                });
            }

            let result = (functions.C_FindObjectsInit)(session, template.as_mut_ptr(), template.len() as c_ulong);
            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to initialize public key search: 0x{:x}", result)));
            }

            let mut key_handle: c_ulong = 0;
            let mut object_count: c_ulong = 0;
            let result = (functions.C_FindObjects)(session, &mut key_handle, 1, &mut object_count);
            
            (functions.C_FindObjectsFinal)(session);

            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to find public key: 0x{:x}", result)));
            }

            if object_count == 0 {
                return Err(SignerError::KeyNotFound("Public key not found in HSM".to_string()));
            }

            // Get EC point (public key)
            let mut point_len: c_ulong = 0;
            let mut point_attr = CK_ATTRIBUTE {
                type_: CKA_EC_POINT,
                p_value: ptr::null_mut(),
                ul_value_len: 0,
            };

            let result = (functions.C_GetAttributeValue)(session, key_handle, &mut point_attr, 1);
            if result == CKR_OK {
                point_len = point_attr.ul_value_len;
            }

            let mut point_bytes = vec![0u8; point_len as usize];
            point_attr.p_value = point_bytes.as_mut_ptr() as *mut c_void;
            point_attr.ul_value_len = point_len;

            let result = (functions.C_GetAttributeValue)(session, key_handle, &mut point_attr, 1);
            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to get public key point: 0x{:x}", result)));
            }

            // Convert to SPKI format (simplified - in practice you'd need proper DER encoding)
            Ok(PublicKey {
                algorithm: self.algorithm.clone(),
                spki_bytes: point_bytes,
            })
        }
    }
}

#[async_trait]
impl Signer for Pkcs11Signer {
    async fn sign(&self, data: &[u8]) -> Result<Signature, SignerError> {
        let functions = unsafe { self.load_functions()? };
        
        unsafe {
            // Initialize PKCS#11
            let result = (functions.C_Initialize)(ptr::null_mut());
            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to initialize PKCS#11: 0x{:x}", result)));
            }

            // Find slot
            let slot = self.find_slot(&functions)?;
            
            // Open session
            let mut session: c_ulong = 0;
            let result = (functions.C_OpenSession)(slot, CKF_SERIAL_SESSION | CKF_RW_SESSION, &mut session);
            if result != CKR_OK {
                (functions.C_Finalize)(ptr::null_mut());
                return Err(SignerError::Pkcs11(format!("Failed to open session: 0x{:x}", result)));
            }

            // Login
            let pin_cstr = CString::new(self.config.pin.as_str()).unwrap();
            let result = (functions.C_Login)(session, CKU_USER, pin_cstr.as_ptr() as *mut c_char);
            if result != CKR_OK {
                (functions.C_CloseSession)(session);
                (functions.C_Finalize)(ptr::null_mut());
                return Err(SignerError::Pkcs11(format!("Failed to login: 0x{:x}", result)));
            }

            // Find private key
            let key_handle = self.find_private_key(&functions, session)?;

            // Initialize signing
            let mechanism = if self.algorithm == "secp256k1" {
                CK_MECHANISM {
                    mechanism: CKM_ECDSA,
                    p_parameter: ptr::null_mut(),
                    ul_parameter_len: 0,
                }
            } else {
                CK_MECHANISM {
                    mechanism: CKM_EDDSA,
                    p_parameter: ptr::null_mut(),
                    ul_parameter_len: 0,
                }
            };

            let result = (functions.C_SignInit)(session, &mut mechanism as *mut _, key_handle);
            if result != CKR_OK {
                (functions.C_Logout)(session);
                (functions.C_CloseSession)(session);
                (functions.C_Finalize)(ptr::null_mut());
                return Err(SignerError::Pkcs11(format!("Failed to initialize signing: 0x{:x}", result)));
            }

            // Sign data
            let mut signature_len: c_ulong = 0;
            let data_ptr = data.as_ptr() as *mut u8;
            let result = (functions.C_Sign)(session, data_ptr, data.len() as c_ulong, ptr::null_mut(), &mut signature_len);
            if result != CKR_OK && result != CKR_BUFFER_TOO_SMALL {
                (functions.C_Logout)(session);
                (functions.C_CloseSession)(session);
                (functions.C_Finalize)(ptr::null_mut());
                return Err(SignerError::Pkcs11(format!("Failed to get signature length: 0x{:x}", result)));
            }

            let mut signature_bytes = vec![0u8; signature_len as usize];
            let result = (functions.C_Sign)(session, data_ptr, data.len() as c_ulong, signature_bytes.as_mut_ptr(), &mut signature_len);
            
            // Cleanup
            (functions.C_Logout)(session);
            (functions.C_CloseSession)(session);
            (functions.C_Finalize)(ptr::null_mut());

            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to sign data: 0x{:x}", result)));
            }

            Ok(Signature {
                algorithm: self.algorithm.clone(),
                bytes: signature_bytes,
            })
        }
    }

    async fn public_key(&self) -> Result<PublicKey, SignerError> {
        let functions = unsafe { self.load_functions()? };
        
        unsafe {
            // Initialize PKCS#11
            let result = (functions.C_Initialize)(ptr::null_mut());
            if result != CKR_OK {
                return Err(SignerError::Pkcs11(format!("Failed to initialize PKCS#11: 0x{:x}", result)));
            }

            // Find slot
            let slot = self.find_slot(&functions)?;
            
            // Open session
            let mut session: c_ulong = 0;
            let result = (functions.C_OpenSession)(slot, CKF_SERIAL_SESSION | CKF_RW_SESSION, &mut session);
            if result != CKR_OK {
                (functions.C_Finalize)(ptr::null_mut());
                return Err(SignerError::Pkcs11(format!("Failed to open session: 0x{:x}", result)));
            }

            // Login
            let pin_cstr = CString::new(self.config.pin.as_str()).unwrap();
            let result = (functions.C_Login)(session, CKU_USER, pin_cstr.as_ptr() as *mut c_char);
            if result != CKR_OK {
                (functions.C_CloseSession)(session);
                (functions.C_Finalize)(ptr::null_mut());
                return Err(SignerError::Pkcs11(format!("Failed to login: 0x{:x}", result)));
            }

            // Get public key
            let public_key = self.get_public_key(&functions, session);

            // Cleanup
            (functions.C_Logout)(session);
            (functions.C_CloseSession)(session);
            (functions.C_Finalize)(ptr::null_mut());

            public_key
        }
    }

    fn signer_info(&self) -> SignerInfo {
        let mut metadata = HashMap::new();
        metadata.insert("implementation".to_string(), "pkcs11".to_string());
        metadata.insert("module_path".to_string(), self.config.module_path.clone());
        
        if let Some(ref token_label) = self.config.token_label {
            metadata.insert("token_label".to_string(), token_label.clone());
        }

        SignerInfo {
            signer_type: "pkcs11".to_string(),
            algorithm: self.algorithm.clone(),
            metadata,
        }
    }
}

/// PKCS#11 function pointers
struct Pkcs11Functions {
    C_Initialize: C_InitializeFn,
    C_Finalize: C_FinalizeFn,
    C_GetInfo: C_GetInfoFn,
    C_GetSlotList: C_GetSlotListFn,
    C_GetSlotInfo: C_GetSlotInfoFn,
    C_GetTokenInfo: C_GetTokenInfoFn,
    C_OpenSession: C_OpenSessionFn,
    C_CloseSession: C_CloseSessionFn,
    C_Login: C_LoginFn,
    C_Logout: C_LogoutFn,
    C_FindObjectsInit: C_FindObjectsInitFn,
    C_FindObjects: C_FindObjectsFn,
    C_FindObjectsFinal: C_FindObjectsFinalFn,
    C_SignInit: C_SignInitFn,
    C_Sign: C_SignFn,
    C_GetAttributeValue: C_GetAttributeValueFn,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pkcs11_config_from_env() {
        // This test will fail unless environment variables are set
        // but it demonstrates the expected behavior
        
        // Temporarily set environment variables
        std::env::set_var("ERST_PKCS11_MODULE", "/usr/lib/libykcs11.so");
        std::env::set_var("ERST_PKCS11_PIN", "123456");
        
        let config = Pkcs11SignerConfig::from_env();
        assert!(config.is_ok());
        
        let config = config.unwrap();
        assert_eq!(config.module_path, "/usr/lib/libykcs11.so");
        assert_eq!(config.pin, "123456");
        
        // Clean up
        std::env::remove_var("ERST_PKCS11_MODULE");
        std::env::remove_var("ERST_PKCS11_PIN");
    }

    #[test]
    fn test_pkcs11_config_missing_required() {
        // Should fail when required environment variables are missing
        std::env::remove_var("ERST_PKCS11_MODULE");
        std::env::remove_var("ERST_PKCS11_PIN");
        
        let config = Pkcs11SignerConfig::from_env();
        assert!(config.is_err());
    }
}
