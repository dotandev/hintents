use proptest::prelude::*;
use soroban_env_host::xdr::{LedgerEntry, Limits, ReadXdr, WriteXdr};
use crate::snapshot::{decode_ledger_entry, decode_ledger_key};
use base64::{engine::general_purpose::STANDARD, Engine};

proptest! {
    #[test]
    fn test_ledger_entry_to_xdr_does_not_panic(entry_bytes in any::<Vec<u8>>()) {
        // Test decoding raw bytes directly
        if let Ok(entry) = LedgerEntry::from_xdr(&entry_bytes, Limits::none()) {
            // If it's a valid entry, encoding it back shouldn't panic
            let _ = entry.to_xdr(Limits::none());
        }

        // Test base64 decoding
        let b64 = STANDARD.encode(&entry_bytes);
        if let Ok(entry) = decode_ledger_entry(&b64) {
            let _ = entry.to_xdr(Limits::none());
        }
    }
}
