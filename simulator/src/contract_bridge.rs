// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

//! Dynamic contract bridging: override external contract WASM during multi-contract replay.

use crate::types::ContractWasmOverrides;
use base64::Engine;
use soroban_env_host::xdr::{
    ContractCodeEntry, ContractCodeEntryExt, ContractDataDurability, ContractDataEntry,
    ContractExecutable, ContractId, LedgerEntry, LedgerEntryData, LedgerEntryExt, LedgerKey,
    LedgerKeyContractCode, LedgerKeyContractData, ReadXdr, ScAddress, ScContractInstance, ScVal,
};
use sha2::{Digest, Sha256};
use std::collections::HashMap;

/// Default ledger seq for injected/overridden entries (high value so they are valid during replay).
const DEFAULT_LIVE_UNTIL: u32 = 16_777_215;

/// Decode contract ID from 64-char hex string to 32-byte hash.
fn contract_id_hex_to_hash(hex_str: &str) -> Result<[u8; 32], String> {
    let hex_str = hex_str.trim_start_matches("0x");
    let bytes = hex::decode(hex_str).map_err(|e| format!("invalid contract_id hex: {}", e))?;
    if bytes.len() != 32 {
        return Err(format!(
            "contract_id must be 32 bytes (64 hex chars), got {}",
            bytes.len()
        ));
    }
    let mut arr = [0u8; 32];
    arr.copy_from_slice(&bytes);
    Ok(arr)
}

/// Build ledger key for contract instance (ContractData with LedgerKeyContractInstance key).
fn instance_ledger_key(contract: &soroban_env_host::xdr::Hash) -> LedgerKey {
    LedgerKey::ContractData(LedgerKeyContractData {
        contract: ScAddress::Contract(ContractId(contract.clone())),
        key: ScVal::LedgerKeyContractInstance,
        durability: ContractDataDurability::Persistent,
    })
}

/// Apply contract WASM overrides to a decoded ledger entry list.
/// For each (contract_id_hex, wasm_b64): adds ContractCode entry and updates the contract instance entry to point to the new code.
pub fn apply_contract_wasm_overrides(
    entries: &mut Vec<(LedgerKey, LedgerEntry)>,
    overrides: &ContractWasmOverrides,
) -> Result<(), String> {
    use soroban_env_host::xdr::Hash;

    for (contract_id_hex, wasm_b64) in overrides {
        let contract_hash = Hash(contract_id_hex_to_hash(contract_id_hex)?);
        let wasm_bytes = base64::engine::general_purpose::STANDARD
            .decode(wasm_b64)
            .map_err(|e| format!("invalid wasm base64 for contract {}: {}", contract_id_hex, e))?;
        let code_hash = Hash(Sha256::digest(&wasm_bytes).into());

        let code_key = LedgerKey::ContractCode(LedgerKeyContractCode {
            hash: code_hash.clone(),
        });
        let code_entry = LedgerEntry {
            last_modified_ledger_seq: DEFAULT_LIVE_UNTIL,
            data: LedgerEntryData::ContractCode(ContractCodeEntry {
                ext: ContractCodeEntryExt::V0,
                hash: code_hash.clone(),
                code: wasm_bytes.try_into().map_err(|_| "wasm too large")?,
            }),
            ext: LedgerEntryExt::V0,
        };

        // Replace or add ContractCode for this hash
        if let Some((_, entry)) = entries.iter_mut().find(|(k, _)| matches!(k, LedgerKey::ContractCode(c) if c.hash == code_hash.clone())) {
            *entry = code_entry.clone();
        } else {
            entries.push((code_key, code_entry));
        }

        // Update contract instance entry to point to new code hash
        let instance_key = instance_ledger_key(&contract_hash);
        let new_instance_val = ScVal::ContractInstance(ScContractInstance {
            executable: ContractExecutable::Wasm(code_hash),
            storage: None,
        });

        if let Some((_, entry)) = entries.iter_mut().find(|(k, _)| {
            if let LedgerKey::ContractData(ref d) = k {
                if d.contract != ScAddress::Contract(ContractId(contract_hash.clone())) {
                    return false;
                }
                matches!(&d.key, ScVal::LedgerKeyContractInstance)
            } else {
                false
            }
        }) {
            if let LedgerEntryData::ContractData(ref mut data) = entry.data {
                data.val = new_instance_val;
            }
        } else {
            // No instance entry yet; add one so the host resolves this contract to our WASM
            let instance_entry = LedgerEntry {
                last_modified_ledger_seq: DEFAULT_LIVE_UNTIL,
                data: LedgerEntryData::ContractData(ContractDataEntry {
                    ext: soroban_env_host::xdr::ExtensionPoint::V0,
                    contract: ScAddress::Contract(ContractId(contract_hash.clone())),
                    key: ScVal::LedgerKeyContractInstance,
                    durability: ContractDataDurability::Persistent,
                    val: new_instance_val,
                }),
                ext: LedgerEntryExt::V0,
            };
            entries.push((instance_key, instance_entry));
        }
    }
    Ok(())
}

/// Decode ledger_entries map (key_xdr -> entry_xdr) into a list of (LedgerKey, LedgerEntry).
/// If contract_wasm_overrides is set, applies overrides to the list.
pub fn decode_ledger_entries_and_apply_overrides(
    ledger_entries: Option<&HashMap<String, String>>,
    overrides: Option<&ContractWasmOverrides>,
) -> Result<Vec<(LedgerKey, LedgerEntry)>, String> {
    let mut entries = Vec::new();
    if let Some(entries_map) = ledger_entries {
        for (key_xdr, entry_xdr) in entries_map {
            let key_bytes = base64::engine::general_purpose::STANDARD
                .decode(key_xdr)
                .map_err(|e| format!("Failed to decode LedgerKey Base64: {}", e))?;
            let key = LedgerKey::from_xdr(key_bytes, soroban_env_host::xdr::Limits::none())
                .map_err(|e| format!("Failed to parse LedgerKey XDR: {}", e))?;
            let entry_bytes = base64::engine::general_purpose::STANDARD
                .decode(entry_xdr)
                .map_err(|e| format!("Failed to decode LedgerEntry Base64: {}", e))?;
            let entry = LedgerEntry::from_xdr(entry_bytes, soroban_env_host::xdr::Limits::none())
                .map_err(|e| format!("Failed to parse LedgerEntry XDR: {}", e))?;
            entries.push((key, entry));
        }
    }
    if let Some(ov) = overrides {
        if !ov.is_empty() {
            apply_contract_wasm_overrides(&mut entries, ov)?;
        }
    }
    Ok(entries)
}

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_env_host::xdr::Hash;

    #[test]
    fn test_contract_id_hex_to_hash() {
        let hex_64 = "00".repeat(32);
        let h = contract_id_hex_to_hash(&hex_64).unwrap();
        assert_eq!(h, [0u8; 32]);
        let bad = contract_id_hex_to_hash("zz");
        assert!(bad.is_err());
        let short = contract_id_hex_to_hash(&"00".repeat(16));
        assert!(short.is_err());
    }

    #[test]
    fn test_apply_overrides_adds_code_and_instance() {
        let mut entries = Vec::new();
        let wasm = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_b64 = base64::engine::general_purpose::STANDARD.encode(&wasm);
        let contract_hash = Hash([1u8; 32]);
        let contract_hex = hex::encode(contract_hash.0);
        let overrides: ContractWasmOverrides =
            [(contract_hex.clone(), wasm_b64)].into_iter().collect();
        apply_contract_wasm_overrides(&mut entries, &overrides).unwrap();
        assert_eq!(entries.len(), 2);
        let (code_key, code_entry) = entries.iter().find(|(k, _)| matches!(k, LedgerKey::ContractCode(_))).unwrap();
        if let LedgerKey::ContractCode(c) = code_key {
            let expected: [u8; 32] = Sha256::digest(&wasm).into();
            assert_eq!(c.hash.0, expected);
        }
        if let LedgerEntryData::ContractCode(ce) = &code_entry.data {
            assert_eq!(ce.code.as_slice(), wasm);
        }
    }
}
