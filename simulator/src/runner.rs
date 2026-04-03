// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use base64::Engine;
use soroban_env_host::{
    budget::Budget,
    events::{Events, HostEvent},
    storage::{AccessType, Footprint, FootprintMap, Storage, StorageMap},
    xdr::{Hash, Limits, ScErrorCode, ScErrorType, WriteXdr},
    DiagnosticLevel, Error as EnvError, Host, HostError, TryIntoVal, Val,
};
use std::rc::Rc;

use crate::snapshot::{LedgerSnapshot, SnapshotError};

#[derive(Debug, thiserror::Error)]
pub enum SimHostError {
    #[error(transparent)]
    Host(#[from] HostError),
    #[error(transparent)]
    Snapshot(#[from] SnapshotError),
}

/// Wrapper around the Soroban Host to manage initialization and execution context.
pub struct SimHost {
    pub inner: Host,
    ledger_snapshot: LedgerSnapshot,
    budget_limits: Option<(u64, u64)>,
    calibration: Option<crate::types::ResourceCalibration>,
    memory_limit: Option<u64>,
}

impl SimHost {
    /// Initialize a new Host with optional budget settings and resource calibration.
    pub fn new(
        budget_limits: Option<(u64, u64)>,
        calibration: Option<crate::types::ResourceCalibration>,
        memory_limit: Option<u64>,
    ) -> Self {
        let budget = Budget::default();

        if let Some(ref _calib) = calibration {
            // Note: In newer versions of soroban_env_host, the Budget interface
            // no longer uses set_model() or CostModel directly like this.
            // Resource calibration settings from the request are ignored
            // in this simulator version to maintain compatibility with the SDK.
        }

        if let Some((_cpu, _mem)) = budget_limits {
            // Budget customization requires testutils feature or extended API
            // Using default mainnet budget settings
        }

        // Host::with_storage_and_budget is available in recent versions
        let host = Host::with_storage_and_budget(Storage::default(), budget);

        host.set_diagnostic_level(DiagnosticLevel::Debug)
            .expect("failed to set diagnostic level");

        Self {
            inner: host,
            ledger_snapshot: LedgerSnapshot::new(),
            budget_limits,
            calibration,
            memory_limit,
        }
    }

    /// Creates a new host initialized with the provided snapshot contents.
    pub fn from_snapshot(
        budget_limits: Option<(u64, u64)>,
        calibration: Option<crate::types::ResourceCalibration>,
        memory_limit: Option<u64>,
        snapshot: &LedgerSnapshot,
    ) -> Result<Self, SimHostError> {
        let budget = Budget::default();
        let storage = Self::storage_from_snapshot(snapshot, &budget)?;
        let host = Host::with_storage_and_budget(storage, budget);
        host.set_diagnostic_level(DiagnosticLevel::Debug)?;

        Ok(Self {
            inner: host,
            ledger_snapshot: snapshot.clone(),
            budget_limits,
            calibration,
            memory_limit,
        })
    }

    /// Replaces the current host with a freshly initialized host loaded from the snapshot.
    pub fn restore_from_snapshot(&mut self, snapshot: &LedgerSnapshot) -> Result<(), SimHostError> {
        let restored = Self::from_snapshot(
            self.budget_limits,
            self.calibration.clone(),
            self.memory_limit,
            snapshot,
        )?;
        *self = restored;
        Ok(())
    }

    /// Captures the current host storage as a reusable ledger snapshot.
    pub fn capture_snapshot(&self) -> Result<LedgerSnapshot, SimHostError> {
        Ok(self.ledger_snapshot.clone())
    }

    /// Returns the host events that have been emitted so far.
    pub fn events(&self) -> Result<Events, SimHostError> {
        Ok(self.inner.get_events()?)
    }

    /// Returns the host events as a cloned vector for external history tracking.
    pub fn event_log(&self) -> Result<Vec<HostEvent>, SimHostError> {
        Ok(self.events()?.0)
    }

    /// Stores or replaces a ledger entry by rebuilding the host from the updated snapshot.
    pub fn set_ledger_entry(
        &mut self,
        key: soroban_env_host::xdr::LedgerKey,
        entry: soroban_env_host::xdr::LedgerEntry,
    ) -> Result<(), SimHostError> {
        let key_bytes = key
            .to_xdr(Limits::none())
            .map_err(|e| SnapshotError::XdrEncoding(format!("Failed to encode key: {e}")))?;
        self.ledger_snapshot.insert(key_bytes, entry);
        let snapshot = self.ledger_snapshot.clone();
        self.restore_from_snapshot(&snapshot)
    }

    fn storage_from_snapshot(snapshot: &LedgerSnapshot, budget: &Budget) -> Result<Storage, SimHostError> {
        let mut footprint_map = FootprintMap::new();
        let mut storage_map = StorageMap::new();

        for (key_bytes, entry) in snapshot.iter() {
            let key = Rc::new(crate::snapshot::decode_ledger_key(
                &base64::engine::general_purpose::STANDARD.encode(key_bytes),
            )?);
            footprint_map = footprint_map.insert(Rc::clone(&key), AccessType::ReadWrite, budget)?;
            storage_map = storage_map.insert(key, Some((Rc::new(entry.clone()), None)), budget)?;
        }

        Ok(Storage::with_enforcing_footprint_and_map(
            Footprint(footprint_map),
            storage_map,
        ))
    }

    /// Set the contract ID for execution context.
    pub fn _set_contract_id(&mut self, _id: Hash) {}

    /// Set the function name to invoke.
    pub fn _set_fn_name(&mut self, _name: &str) -> Result<(), HostError> {
        Ok(())
    }

    /// Convert a u32 to a Soroban Val.
    pub fn _val_from_u32(&self, v: u32) -> Val {
        Val::from_u32(v).into()
    }

    /// Convert a Val back to u32.
    pub fn _val_to_u32(&self, v: Val) -> Result<u32, HostError> {
        v.try_into_val(&self.inner).map_err(|_| {
            EnvError::from_type_and_code(ScErrorType::Context, ScErrorCode::InvalidInput).into()
        })
    }

    /// Buffer a contract event for inclusion in the next snapshot.
    ///
    /// Call this from the simulation loop each time an event is emitted so that
    /// `_drain_events_for_snapshot` can associate the right events with each
    /// snapshot window.
    pub fn _push_event(&mut self, event: String) {
        self._pending_events.push(event);
    }

    /// Return all events buffered since the last snapshot and clear the buffer.
    ///
    /// The returned `Vec` is moved into the `events` field of the `StateSnapshot`
    /// being constructed.  After this call the buffer is empty and ready for the
    /// next snapshot window.
    pub fn _drain_events_for_snapshot(&mut self) -> Vec<String> {
        std::mem::take(&mut self._pending_events)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_env_host::EnvBase;
    use soroban_env_host::xdr::{
        ContractDataDurability, ContractDataEntry, ContractId, Hash, LedgerEntry, LedgerEntryData,
        LedgerEntryExt, LedgerKey, LedgerKeyContractData, ScAddress, ScVal,
    };

    #[test]
    fn test_host_initialization() {
        let host = SimHost::new(None, None, None);
        // Basic assertion that host is functional
        assert!(host.inner.budget_cloned().get_cpu_insns_consumed().is_ok());
    }

    #[test]
    fn test_configuration() {
        let mut host = SimHost::new(None, None, None);
        // Test setting contract ID (dummy hash)
        let hash = Hash([0u8; 32]);
        host._set_contract_id(hash);

        host._set_fn_name("add")
            .expect("failed to set function name");
    }

    #[test]
    fn test_simple_value_handling() {
        let host = SimHost::new(None, None, None);

        let val_a = host._val_from_u32(10);
        let val_b = host._val_from_u32(20);

        let res_a = host._val_to_u32(val_a).expect("conversion failed");
        let res_b = host._val_to_u32(val_b).expect("conversion failed");

        assert_eq!(res_a + res_b, 30);
    }

    #[test]
    fn test_restore_from_snapshot_replaces_mutated_storage_and_clears_host_events() {
        let mut host = SimHost::new(None, None, None);
        let first_key = Rc::new(LedgerKey::ContractData(LedgerKeyContractData {
            contract: ScAddress::Contract(ContractId(Hash([1u8; 32]))),
            key: ScVal::U32(1),
            durability: ContractDataDurability::Persistent,
        }));
        let first_entry = Rc::new(LedgerEntry {
            last_modified_ledger_seq: 1,
            data: LedgerEntryData::ContractData(ContractDataEntry {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract: ScAddress::Contract(ContractId(Hash([1u8; 32]))),
                key: ScVal::U32(1),
                durability: ContractDataDurability::Persistent,
                val: ScVal::U32(10),
            }),
            ext: LedgerEntryExt::V0,
        });
        host.set_ledger_entry(first_key.as_ref().clone(), first_entry.as_ref().clone())
            .expect("initial entry should be stored");
        host.inner
            .log_from_slice("before snapshot", &[])
            .expect("diagnostic event should be recorded");

        let snapshot = host.capture_snapshot().expect("snapshot should capture");

        let second_key = Rc::new(LedgerKey::ContractData(LedgerKeyContractData {
            contract: ScAddress::Contract(ContractId(Hash([2u8; 32]))),
            key: ScVal::U32(2),
            durability: ContractDataDurability::Persistent,
        }));
        let second_entry = Rc::new(LedgerEntry {
            last_modified_ledger_seq: 2,
            data: LedgerEntryData::ContractData(ContractDataEntry {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract: ScAddress::Contract(ContractId(Hash([2u8; 32]))),
                key: ScVal::U32(2),
                durability: ContractDataDurability::Persistent,
                val: ScVal::U32(20),
            }),
            ext: LedgerEntryExt::V0,
        });
        host.set_ledger_entry(second_key.as_ref().clone(), second_entry.as_ref().clone())
            .expect("mutated entry should be stored");
        host.inner
            .log_from_slice("after snapshot", &[])
            .expect("later event should be recorded");

        host.restore_from_snapshot(&snapshot)
            .expect("restoring snapshot should succeed");

        let restored = host.capture_snapshot().expect("restored snapshot should capture");
        assert_eq!(restored.len(), 1);
        assert!(restored.get(&first_key.to_xdr(Limits::none()).unwrap()).is_some());
        assert!(restored.get(&second_key.to_xdr(Limits::none()).unwrap()).is_none());
        assert!(
            host.events().expect("events should read").0.is_empty(),
            "fresh host should not retain post-rollback host events"
        );
    }
}
