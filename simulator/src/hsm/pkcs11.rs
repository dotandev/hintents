use super::{
    HsmError, PublicKey, Signature, SignatureAlgorithm, Signer,
};
use libloading::{Library, Symbol};
use pkcs11::types::*;
use std::ffi::CString;
use std::sync::Arc;
use tokio::sync::Mutex;

/// PKCS#11-based HSM signer implementation
pub struct Pkcs11Signer {
    library: Arc<Mutex<Pkcs11Library>>,
    session_handle: CK_SESSION_HANDLE,
    key_handle: CK_OBJECT_HANDLE,
    algorithm: SignatureAlgorithm,
    slot_id: CK_SLOT_ID,
    pin: String,
}

/// Wrapper for dynamically loaded PKCS#11 library
struct Pkcs11Library {
    lib: Library,
    C_Initialize: Symbol<'static, unsafe extern "C" fn(CK_VOID_PTR) -> CK_RV>,
    C_Finalize: Symbol<'static, unsafe extern "C" fn(CK_VOID_PTR) -> CK_RV>,
    C_GetSlotList: Symbol<'static, unsafe extern "C" fn(CK_BBOOL, *mut CK_SLOT_ID, *mut CK_ULONG) -> CK_RV>,
    C_OpenSession: Symbol<'static, unsafe extern "C" fn(CK_SLOT_ID, CK_FLAGS, CK_VOID_PTR, CK_NOTIFY, *mut CK_SESSION_HANDLE) -> CK_RV>,
    C_CloseSession: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE) -> CK_RV>,
    C_Login: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE, CK_USER_TYPE, CK_CHAR_PTR, CK_ULONG) -> CK_RV>,
    C_Logout: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE) -> CK_RV>,
    C_FindObjectsInit: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE, *mut CK_ATTRIBUTE, CK_ULONG) -> CK_RV>,
    C_FindObjects: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE, *mut CK_OBJECT_HANDLE, CK_ULONG, *mut CK_ULONG) -> CK_RV>,
    C_FindObjectsFinal: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE) -> CK_RV>,
    C_SignInit: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE, *mut CK_MECHANISM, CK_OBJECT_HANDLE) -> CK_RV>,
    C_Sign: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE, CK_BYTE_PTR, CK_ULONG, CK_BYTE_PTR, *mut CK_ULONG) -> CK_RV>,
    C_GetAttributeValue: Symbol<'static, unsafe extern "C" fn(CK_SESSION_HANDLE, CK_OBJECT_HANDLE, *mut CK_ATTRIBUTE, CK_ULONG) -> CK_RV>,
}

impl Pkcs11Signer {
    /// Create a new PKCS#11 signer
    pub async fn new(
        library_path: &str,
        slot_id: Option<u32>,
        pin: &str,
        algorithm: SignatureAlgorithm,
        key_label: Option<&str>,
    ) -> Result<Self, HsmError> {
        let library = Pkcs11Library::load(library_path)?;
        
        // Initialize the library
        unsafe {
            let rv = (library.C_Initialize)(std::ptr::null_mut());
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_Initialize failed: {}", rv)));
            }
        }

        // Get slot list
        let slots = library.get_slot_list()?;
        let slot_id = if let Some(slot_id) = slot_id {
            slot_id as CK_SLOT_ID
        } else {
            *slots.first()
                .ok_or_else(|| HsmError::Configuration("No PKCS#11 slots available".to_string()))?
        };

        // Open session
        let session_handle = library.open_session(slot_id)?;
        
        // Login if PIN is provided
        if !pin.is_empty() {
            library.login(session_handle, pin)?;
        }

        // Find the key
        let key_handle = if let Some(key_label) = key_label {
            library.find_key_by_label(session_handle, key_label)?
        } else {
            library.find_first_key(session_handle, algorithm)?
        };

        // Verify the key supports the required algorithm
        library.verify_key_algorithm(session_handle, key_handle, algorithm)?;

        Ok(Self {
            library: Arc::new(Mutex::new(library)),
            session_handle,
            key_handle,
            algorithm,
            slot_id,
            pin: pin.to_string(),
        })
    }

    /// Create a signer with the first available key
    pub async fn with_first_key(
        library_path: &str,
        slot_id: Option<u32>,
        pin: &str,
        algorithm: SignatureAlgorithm,
    ) -> Result<Self, HsmError> {
        Self::new(library_path, slot_id, pin, algorithm, None).await
    }
}

impl Signer for Pkcs11Signer {
    async fn sign(&self, message: &[u8]) -> Result<Signature, HsmError> {
        let library = self.library.lock().await;
        
        // Initialize signing operation
        let mechanism = match self.algorithm {
            SignatureAlgorithm::Ed25519 => CK_MECHANISM {
                mechanism: CKM_EDDSA,
                pParameter: std::ptr::null_mut(),
                ulParameterLen: 0,
            },
            SignatureAlgorithm::Secp256k1 => CK_MECHANISM {
                mechanism: CKM_ECDSA,
                pParameter: std::ptr::null_mut(),
                ulParameterLen: 0,
            },
        };

        unsafe {
            let rv = (library.C_SignInit)(self.session_handle, &mechanism as *const _ as *mut _, self.key_handle);
            if rv != CKR_OK {
                return Err(HsmError::SigningError(format!("C_SignInit failed: {}", rv)));
            }

            // Get signature length
            let mut signature_len = 0u64;
            let rv = (library.C_Sign)(
                self.session_handle,
                message.as_ptr(),
                message.len() as CK_ULONG,
                std::ptr::null_mut(),
                &mut signature_len,
            );
            if rv != CKR_OK && rv != CKR_BUFFER_TOO_SMALL {
                return Err(HsmError::SigningError(format!("C_Sign (get length) failed: {}", rv)));
            }

            // Allocate buffer and get signature
            let mut signature = vec![0u8; signature_len as usize];
            let rv = (library.C_Sign)(
                self.session_handle,
                message.as_ptr(),
                message.len() as CK_ULONG,
                signature.as_mut_ptr(),
                &mut signature_len,
            );
            if rv != CKR_OK {
                return Err(HsmError::SigningError(format!("C_Sign failed: {}", rv)));
            }

            signature.truncate(signature_len as usize);
            
            Ok(Signature {
                algorithm: self.algorithm,
                bytes: signature,
            })
        }
    }

    async fn public_key(&self) -> Result<PublicKey, HsmError> {
        let library = self.library.lock().await;
        library.get_public_key(self.session_handle, self.key_handle, self.algorithm).await
    }

    async fn verify(&self, message: &[u8], signature: &Signature) -> Result<bool, HsmError> {
        let public_key = self.public_key().await?;
        public_key.verify(message, signature).await
    }

    async fn attestation(&self) -> Result<Vec<u8>, HsmError> {
        Err(HsmError::NotImplemented("PKCS#11 attestation not yet implemented".to_string()))
    }
}

impl Drop for Pkcs11Signer {
    fn drop(&mut self) {
        // Note: We can't use async in Drop, so we'll try to lock without blocking
        if let Ok(library) = self.library.try_lock() {
            unsafe {
                // Logout if logged in
                if !self.pin.is_empty() {
                    let _ = (library.C_Logout)(self.session_handle);
                }
                
                // Close session
                let _ = (library.C_CloseSession)(self.session_handle);
                
                // Finalize library
                let _ = (library.C_Finalize)(std::ptr::null_mut());
            }
        }
    }
}

impl Pkcs11Library {
    fn load(library_path: &str) -> Result<Self, HsmError> {
        let lib = Library::new(library_path)
            .map_err(|e| HsmError::Configuration(format!("Failed to load PKCS#11 library: {}", e)))?;

        unsafe {
            let C_Initialize = lib.get(b"C_Initialize")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_Initialize: {}", e)))?;
            let C_Finalize = lib.get(b"C_Finalize")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_Finalize: {}", e)))?;
            let C_GetSlotList = lib.get(b"C_GetSlotList")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_GetSlotList: {}", e)))?;
            let C_OpenSession = lib.get(b"C_OpenSession")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_OpenSession: {}", e)))?;
            let C_CloseSession = lib.get(b"C_CloseSession")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_CloseSession: {}", e)))?;
            let C_Login = lib.get(b"C_Login")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_Login: {}", e)))?;
            let C_Logout = lib.get(b"C_Logout")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_Logout: {}", e)))?;
            let C_FindObjectsInit = lib.get(b"C_FindObjectsInit")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_FindObjectsInit: {}", e)))?;
            let C_FindObjects = lib.get(b"C_FindObjects")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_FindObjects: {}", e)))?;
            let C_FindObjectsFinal = lib.get(b"C_FindObjectsFinal")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_FindObjectsFinal: {}", e)))?;
            let C_SignInit = lib.get(b"C_SignInit")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_SignInit: {}", e)))?;
            let C_Sign = lib.get(b"C_Sign")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_Sign: {}", e)))?;
            let C_GetAttributeValue = lib.get(b"C_GetAttributeValue")
                .map_err(|e| HsmError::Configuration(format!("Failed to get C_GetAttributeValue: {}", e)))?;

            Ok(Self {
                lib,
                C_Initialize,
                C_Finalize,
                C_GetSlotList,
                C_OpenSession,
                C_CloseSession,
                C_Login,
                C_Logout,
                C_FindObjectsInit,
                C_FindObjects,
                C_FindObjectsFinal,
                C_SignInit,
                C_Sign,
                C_GetAttributeValue,
            })
        }
    }

    fn get_slot_list(&self) -> Result<Vec<CK_SLOT_ID>, HsmError> {
        unsafe {
            // Get slot count
            let mut slot_count = 0u64;
            let rv = (self.C_GetSlotList)(CK_FALSE, std::ptr::null_mut(), &mut slot_count);
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_GetSlotList (count) failed: {}", rv)));
            }

            // Get actual slots
            let mut slots = vec![0u64; slot_count as usize];
            let rv = (self.C_GetSlotList)(CK_FALSE, slots.as_mut_ptr(), &mut slot_count);
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_GetSlotList failed: {}", rv)));
            }

            slots.truncate(slot_count as usize);
            Ok(slots)
        }
    }

    fn open_session(&self, slot_id: CK_SLOT_ID) -> Result<CK_SESSION_HANDLE, HsmError> {
        unsafe {
            let mut session_handle = 0u64;
            let rv = (self.C_OpenSession)(
                slot_id,
                CKF_SERIAL_SESSION | CKF_RW_SESSION,
                std::ptr::null_mut(),
                None,
                &mut session_handle,
            );
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_OpenSession failed: {}", rv)));
            }
            Ok(session_handle)
        }
    }

    fn login(&self, session_handle: CK_SESSION_HANDLE, pin: &str) -> Result<(), HsmError> {
        unsafe {
            let pin_cstring = CString::new(pin)
                .map_err(|e| HsmError::Configuration(format!("Invalid PIN: {}", e)))?;
            
            let rv = (self.C_Login)(
                session_handle,
                CKU_USER,
                pin_cstring.as_ptr(),
                pin.len() as CK_ULONG,
            );
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_Login failed: {}", rv)));
            }
            Ok(())
        }
    }

    fn find_key_by_label(&self, session_handle: CK_SESSION_HANDLE, label: &str) -> Result<CK_OBJECT_HANDLE, HsmError> {
        unsafe {
            let label_cstring = CString::new(label)
                .map_err(|e| HsmError::Configuration(format!("Invalid label: {}", e)))?;
            
            let mut attrs = [
                CK_ATTRIBUTE {
                    type_: CKA_LABEL,
                    pValue: label_cstring.as_ptr() as *mut _,
                    ulValueLen: label.len() as CK_ULONG,
                },
                CK_ATTRIBUTE {
                    type_: CKA_CLASS,
                    pValue: &CKO_PRIVATE_KEY as *const _ as *mut _,
                    ulValueLen: std::mem::size_of::<CK_ULONG>() as CK_ULONG,
                },
            ];

            let rv = (self.C_FindObjectsInit)(session_handle, attrs.as_mut_ptr(), attrs.len() as CK_ULONG);
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_FindObjectsInit failed: {}", rv)));
            }

            let mut objects = [0u64; 1];
            let mut object_count = 0u64;
            let rv = (self.C_FindObjects)(session_handle, objects.as_mut_ptr(), 1, &mut object_count);
            let finalize_rv = (self.C_FindObjectsFinal)(session_handle);

            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_FindObjects failed: {}", rv)));
            }
            if finalize_rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_FindObjectsFinal failed: {}", finalize_rv)));
            }

            if object_count == 0 {
                return Err(HsmError::Configuration(format!("No key found with label: {}", label)));
            }

            Ok(objects[0])
        }
    }

    fn find_first_key(&self, session_handle: CK_SESSION_HANDLE, algorithm: SignatureAlgorithm) -> Result<CK_OBJECT_HANDLE, HsmError> {
        unsafe {
            let key_type = match algorithm {
                SignatureAlgorithm::Ed25519 => CKK_ED25519,
                SignatureAlgorithm::Secp256k1 => CKK_EC,
            };

            let mut attrs = [
                CK_ATTRIBUTE {
                    type_: CKA_CLASS,
                    pValue: &CKO_PRIVATE_KEY as *const _ as *mut _,
                    ulValueLen: std::mem::size_of::<CK_ULONG>() as CK_ULONG,
                },
                CK_ATTRIBUTE {
                    type_: CKA_KEY_TYPE,
                    pValue: &key_type as *const _ as *mut _,
                    ulValueLen: std::mem::size_of::<CK_ULONG>() as CK_ULONG,
                },
            ];

            let rv = (self.C_FindObjectsInit)(session_handle, attrs.as_mut_ptr(), attrs.len() as CK_ULONG);
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_FindObjectsInit failed: {}", rv)));
            }

            let mut objects = [0u64; 1];
            let mut object_count = 0u64;
            let rv = (self.C_FindObjects)(session_handle, objects.as_mut_ptr(), 1, &mut object_count);
            let finalize_rv = (self.C_FindObjectsFinal)(session_handle);

            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_FindObjects failed: {}", rv)));
            }
            if finalize_rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_FindObjectsFinal failed: {}", finalize_rv)));
            }

            if object_count == 0 {
                return Err(HsmError::Configuration("No suitable key found".to_string()));
            }

            Ok(objects[0])
        }
    }

    fn verify_key_algorithm(&self, session_handle: CK_SESSION_HANDLE, key_handle: CK_OBJECT_HANDLE, algorithm: SignatureAlgorithm) -> Result<(), HsmError> {
        unsafe {
            let expected_key_type = match algorithm {
                SignatureAlgorithm::Ed25519 => CKK_ED25519,
                SignatureAlgorithm::Secp256k1 => CKK_EC,
            };

            let mut key_type = CK_ULONG::default();
            let mut attr = CK_ATTRIBUTE {
                type_: CKA_KEY_TYPE,
                pValue: &mut key_type as *mut _ as *mut _,
                ulValueLen: std::mem::size_of::<CK_ULONG>() as CK_ULONG,
            };

            let rv = (self.C_GetAttributeValue)(session_handle, key_handle, &mut attr, 1);
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_GetAttributeValue failed: {}", rv)));
            }

            if key_type != expected_key_type {
                return Err(HsmError::Configuration(
                    format!("Key type mismatch: expected {:?}, got {:?}", expected_key_type, key_type)
                ));
            }

            Ok(())
        }
    }

    async fn get_public_key(&self, session_handle: CK_SESSION_HANDLE, key_handle: CK_OBJECT_HANDLE, algorithm: SignatureAlgorithm) -> Result<PublicKey, HsmError> {
        unsafe {
            // For simplicity, we'll get the public key value directly
            // In a real implementation, you might need to find the corresponding public key object
            let mut public_key_len = 0u64;
            let mut attr = CK_ATTRIBUTE {
                type_: CKA_PUBLIC_KEY_INFO,
                pValue: std::ptr::null_mut(),
                ulValueLen: 0,
            };

            // First get the length
            let rv = (self.C_GetAttributeValue)(session_handle, key_handle, &mut attr, 1);
            if rv != CKR_OK && rv != CKR_ATTRIBUTE_TYPE_INVALID {
                return Err(HsmError::Pkcs11Error(format!("C_GetAttributeValue (length) failed: {}", rv)));
            }

            // Try different attributes based on algorithm
            let attr_type = if rv == CKR_ATTRIBUTE_TYPE_INVALID {
                match algorithm {
                    SignatureAlgorithm::Ed25519 => CKA_EDDSA_PUBLIC_KEY,
                    SignatureAlgorithm::Secp256k1 => CKA_EC_POINT,
                }
            } else {
                CKA_PUBLIC_KEY_INFO
            };

            let mut public_key_len = 0u64;
            let mut attr = CK_ATTRIBUTE {
                type_: attr_type,
                pValue: std::ptr::null_mut(),
                ulValueLen: 0,
            };

            let rv = (self.C_GetAttributeValue)(session_handle, key_handle, &mut attr, 1);
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_GetAttributeValue (public key length) failed: {}", rv)));
            }

            let mut public_key = vec![0u8; attr.ulValueLen as usize];
            let mut attr = CK_ATTRIBUTE {
                type_: attr_type,
                pValue: public_key.as_mut_ptr(),
                ulValueLen: public_key.len() as CK_ULONG,
            };

            let rv = (self.C_GetAttributeValue)(session_handle, key_handle, &mut attr, 1);
            if rv != CKR_OK {
                return Err(HsmError::Pkcs11Error(format!("C_GetAttributeValue (public key) failed: {}", rv)));
            }

            Ok(PublicKey {
                algorithm,
                bytes: public_key,
            })
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::hsm::SignatureAlgorithm;

    #[tokio::test]
    async fn test_pkcs11_signer_creation() {
        // This test requires a real PKCS#11 library to be available
        // For now, we'll just test the error path
        let result = Pkcs11Signer::new(
            "/nonexistent/library.so",
            Some(0),
            "1234",
            SignatureAlgorithm::Ed25519,
            Some("test-key"),
        ).await;

        assert!(matches!(result, Err(HsmError::Configuration(_))));
    }

    #[tokio::test]
    async fn test_pkcs11_signer_with_first_key() {
        // Test with first key selection
        let result = Pkcs11Signer::with_first_key(
            "/nonexistent/library.so",
            None,
            "1234",
            SignatureAlgorithm::Secp256k1,
        ).await;

        assert!(matches!(result, Err(HsmError::Configuration(_))));
    }
}
