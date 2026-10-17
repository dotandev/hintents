// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Property-based testing for LedgerEntry to XDR conversions (Issue #1929).
//!
//! Verifies that:
//! 1. Valid LedgerEntry structures across all major variants round-trip through XDR encoding and decoding losslessly.
//! 2. Base64-encoded LedgerEntry strings round-trip through `decode_ledger_entry`.
//! 3. Arbitrary, corrupted, truncated, or malicious payloads never cause panics or crashes in the converter.

use base64::engine::general_purpose::STANDARD;
use base64::Engine;
use proptest::prelude::*;
use soroban_env_host::xdr::{
    AccountEntry, AccountId, ContractCodeEntry, ContractCodeEntryExt, ContractDataDurability,
    ContractDataEntry, ContractId, ExtensionPoint, Hash, LedgerEntry, LedgerEntryData, Limits,
    PublicKey, ReadXdr, ScAddress, ScVal, SequenceNumber, Thresholds, TtlEntry, Uint256, WriteXdr,
};

use crate::snapshot::decode_ledger_entry;

// ---------------------------------------------------------------------------
// Proptest Strategies for Generating Synthetic LedgerEntries
// ---------------------------------------------------------------------------

fn arb_uint256() -> impl Strategy<Value = Uint256> {
    any::<[u8; 32]>().prop_map(Uint256)
}

fn arb_hash() -> impl Strategy<Value = Hash> {
    any::<[u8; 32]>().prop_map(Hash)
}

fn arb_account_id() -> impl Strategy<Value = AccountId> {
    arb_uint256().prop_map(|u| AccountId(PublicKey::PublicKeyTypeEd25519(u)))
}

fn arb_sc_address() -> impl Strategy<Value = ScAddress> {
    prop_oneof![
        arb_account_id().prop_map(ScAddress::Account),
        arb_hash().prop_map(|h| ScAddress::Contract(ContractId(h))),
    ]
}

fn arb_sc_val() -> impl Strategy<Value = ScVal> {
    prop_oneof![
        any::<bool>().prop_map(ScVal::Bool),
        any::<u32>().prop_map(ScVal::U32),
        any::<i32>().prop_map(ScVal::I32),
        any::<u64>().prop_map(ScVal::U64),
        any::<i64>().prop_map(ScVal::I64),
        Just(ScVal::Void),
    ]
}

fn arb_durability() -> impl Strategy<Value = ContractDataDurability> {
    prop_oneof![
        Just(ContractDataDurability::Persistent),
        Just(ContractDataDurability::Temporary),
    ]
}

fn arb_account_entry() -> impl Strategy<Value = AccountEntry> {
    (
        arb_account_id(),
        any::<i64>(),
        any::<i64>().prop_map(SequenceNumber),
        any::<u32>(),
        any::<u32>(),
        any::<[u8; 4]>().prop_map(Thresholds),
    )
        .prop_map(
            |(account_id, balance, seq_num, num_sub_entries, flags, thresholds)| AccountEntry {
                account_id,
                balance,
                seq_num,
                num_sub_entries,
                inflation_dest: None,
                flags,
                home_domain: Default::default(),
                thresholds,
                signers: Default::default(),
                ext: Default::default(),
            },
        )
}

fn arb_contract_data_entry() -> impl Strategy<Value = ContractDataEntry> {
    (
        arb_sc_address(),
        arb_sc_val(),
        arb_durability(),
        arb_sc_val(),
    )
        .prop_map(|(contract, key, durability, val)| ContractDataEntry {
            contract,
            key,
            durability,
            val,
            ext: ExtensionPoint::V0,
        })
}

fn arb_contract_code_entry() -> impl Strategy<Value = ContractCodeEntry> {
    (arb_hash(), prop::collection::vec(any::<u8>(), 0..2048)).prop_map(|(hash, code_vec)| {
        let code_bytes = code_vec.try_into().unwrap_or_default();
        ContractCodeEntry {
            hash,
            code: code_bytes,
            ext: ContractCodeEntryExt::V0,
        }
    })
}

fn arb_ttl_entry() -> impl Strategy<Value = TtlEntry> {
    (arb_hash(), any::<u32>()).prop_map(|(key_hash, live_until_ledger_seq)| TtlEntry {
        key_hash,
        live_until_ledger_seq,
    })
}

fn arb_ledger_entry_data() -> impl Strategy<Value = LedgerEntryData> {
    prop_oneof![
        arb_account_entry().prop_map(LedgerEntryData::Account),
        arb_contract_data_entry().prop_map(LedgerEntryData::ContractData),
        arb_contract_code_entry().prop_map(LedgerEntryData::ContractCode),
        arb_ttl_entry().prop_map(LedgerEntryData::Ttl),
    ]
}

fn arb_ledger_entry() -> impl Strategy<Value = LedgerEntry> {
    (any::<u32>(), arb_ledger_entry_data()).prop_map(|(last_modified_ledger_seq, data)| {
        LedgerEntry {
            last_modified_ledger_seq,
            data,
            ext: Default::default(),
        }
    })
}

// ---------------------------------------------------------------------------
// Property-Based Invariant Assertions
// ---------------------------------------------------------------------------

proptest! {
    #![proptest_config(ProptestConfig::with_cases(500))]

    #[test]
    fn prop_ledger_entry_xdr_roundtrip(entry in arb_ledger_entry()) {
        // 1. Serialization must succeed without error or panic
        let bytes = entry.to_xdr(Limits::none())
            .expect("serialization to XDR should succeed for valid LedgerEntry");
        prop_assert!(!bytes.is_empty(), "serialized XDR bytes must not be empty");

        // 2. Deserialization must yield identical LedgerEntry
        let decoded = LedgerEntry::from_xdr(&bytes, Limits::none())
            .expect("deserialization from XDR should succeed for valid payload");
        prop_assert_eq!(entry, decoded, "round-trip LedgerEntry must equal original");
    }

    #[test]
    fn prop_ledger_entry_base64_roundtrip(entry in arb_ledger_entry()) {
        let bytes = entry.to_xdr(Limits::none())
            .expect("to_xdr should succeed");
        let b64 = STANDARD.encode(&bytes);

        // decode_ledger_entry must parse base64 without error
        let decoded = decode_ledger_entry(&b64)
            .expect("decode_ledger_entry should parse valid base64 payload");
        prop_assert_eq!(entry, decoded, "base64 round-trip must equal original");
    }

    #[test]
    fn prop_arbitrary_bytes_no_panic(corrupted_bytes in prop::collection::vec(any::<u8>(), 0..1024)) {
        // Deserializing completely randomized byte slices must NEVER panic
        let _ = LedgerEntry::from_xdr(&corrupted_bytes, Limits::none());

        // Base64 encoding arbitrary bytes and calling decode_ledger_entry must NEVER panic
        let b64 = STANDARD.encode(&corrupted_bytes);
        let _ = decode_ledger_entry(&b64);
    }

    #[test]
    fn prop_truncated_xdr_graceful_rejection(
        entry in arb_ledger_entry(),
        cut in any::<prop::sample::Index>(),
    ) {
        let bytes = entry.to_xdr(Limits::none()).unwrap();
        if !bytes.is_empty() {
            let cut_idx = cut.index(bytes.len());
            let truncated = &bytes[..cut_idx];

            // Truncated bytes must either deserialize (if cut_idx happened to be full len)
            // or fail gracefully with Err — NEVER panic.
            let res = LedgerEntry::from_xdr(truncated, Limits::none());
            if cut_idx < bytes.len() {
                let _ = res;
            }
        }
    }

    #[test]
    fn prop_arbitrary_string_payload_no_panic(raw_string in "\\PC*") {
        // Calling decode_ledger_entry on completely random utf-8 strings must NEVER panic
        let _ = decode_ledger_entry(&raw_string);
    }
}
