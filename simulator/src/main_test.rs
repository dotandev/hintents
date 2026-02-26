// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

#[cfg(test)]
mod restore_preamble_tests {
    use super::*;
    use crate::types::SimulationRequest;
    use base64::Engine as _;
    use soroban_env_host::xdr::{LedgerEntry, LedgerKey, WriteXdr, ContractDataDurability, ContractDataEntry, LedgerEntryData, LedgerEntryExt, Hash, ScAddress, ScVal, LedgerKeyContractData};
    use serde_json::json;

    fn encode_xdr<T: WriteXdr>(value: &T) -> String {
        let bytes = value.to_xdr(soroban_env_host::xdr::Limits::none()).unwrap();
        base64::engine::general_purpose::STANDARD.encode(&bytes)
    }

    #[test]
    fn test_restore_preamble_injection() {
        // Create a ContractData key and entry
        let contract_id = Hash([9u8; 32]);
        let key_val = ScVal::U32(123);
        let val = ScVal::U64(456);
        let key = LedgerKey::ContractData(LedgerKeyContractData {
            contract: ScAddress::Contract(contract_id.clone()),
            key: key_val.clone(),
            durability: ContractDataDurability::Persistent,
        });
        let entry = LedgerEntry {
            last_modified_ledger_seq: 999,
            data: LedgerEntryData::ContractData(ContractDataEntry {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract: ScAddress::Contract(contract_id),
                key: key_val,
                durability: ContractDataDurability::Persistent,
                val,
            }),
            ext: LedgerEntryExt::V0,
        };
        let key_b64 = encode_xdr(&key);
        let entry_b64 = encode_xdr(&entry);
        let restore_preamble = json!({
            "ledger_entries": {
                key_b64.clone(): entry_b64.clone()
            }
        });
        let req = SimulationRequest {
            envelope_xdr: "".to_string(),
            result_meta_xdr: "".to_string(),
            ledger_entries: None,
            contract_wasm: None,
            wasm_path: None,
            enable_optimization_advisor: false,
            profile: None,
            timestamp: "".to_string(),
            mock_base_fee: None,
            mock_gas_price: None,
            restore_preamble: Some(restore_preamble),
        };
        // Simulate main logic: inject restore_preamble into host storage
        let sim_host = crate::runner::SimHost::new(None, None);
        let host = sim_host.inner;
        if let Some(ref preamble) = req.restore_preamble {
            if let Some(obj) = preamble.as_object() {
                if let Some(entries) = obj.get("ledger_entries") {
                    if let Some(map) = entries.as_object() {
                        for (key_xdr, entry_xdr_val) in map {
                            if let Some(entry_xdr) = entry_xdr_val.as_str() {
                                let key = crate::snapshot::decode_ledger_key(key_xdr).unwrap();
                                let entry = crate::snapshot::decode_ledger_entry(entry_xdr).unwrap();
                                let result = host.put_ledger_entry(key.clone(), entry.clone());
                                assert!(result.is_ok(), "Injection should succeed");
                            }
                        }
                    }
                }
            }
        }
    }
}
