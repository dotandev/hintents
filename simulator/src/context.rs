// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

use std::collections::HashMap;

use soroban_env_host::events::HostEvent;
use soroban_env_host::xdr::{LedgerEntry, LedgerKey};

use crate::runner::{SimHost, SimHostError};
use crate::snapshot::LedgerSnapshot;

#[derive(Debug, Clone)]
struct SnapshotState {
    ledger: LedgerSnapshot,
    event_count: usize,
}

#[derive(Debug, thiserror::Error)]
pub enum SimulationContextError {
    #[error("snapshot '{0}' not found")]
    SnapshotNotFound(String),
    #[error(transparent)]
    Host(#[from] SimHostError),
}

/// Owns the active simulator host and a rewindable history of snapshots.
pub struct SimulationContext {
    host: SimHost,
    snapshots: HashMap<String, SnapshotState>,
    committed_events: Vec<HostEvent>,
    synced_host_event_count: usize,
}

impl SimulationContext {
    pub fn new(host: SimHost) -> Self {
        Self {
            host,
            snapshots: HashMap::new(),
            committed_events: Vec::new(),
            synced_host_event_count: 0,
        }
    }

    pub fn host(&self) -> &SimHost {
        &self.host
    }

    pub fn host_mut(&mut self) -> &mut SimHost {
        &mut self.host
    }

    pub fn set_ledger_entry(
        &mut self,
        key: LedgerKey,
        entry: LedgerEntry,
    ) -> Result<(), SimulationContextError> {
        self.sync_events()?;
        self.host.set_ledger_entry(key, entry)?;
        self.synced_host_event_count = 0;
        Ok(())
    }

    pub fn capture_snapshot(&mut self, snapshot_id: impl Into<String>) -> Result<(), SimulationContextError> {
        self.sync_events()?;
        let snapshot = self.host.capture_snapshot()?;
        self.snapshots.insert(
            snapshot_id.into(),
            SnapshotState {
                ledger: snapshot,
                event_count: self.committed_events.len(),
            },
        );
        Ok(())
    }

    pub fn rollback_to(&mut self, snapshot_id: &str) -> Result<(), SimulationContextError> {
        self.sync_events()?;
        let snapshot = self
            .snapshots
            .get(snapshot_id)
            .cloned()
            .ok_or_else(|| SimulationContextError::SnapshotNotFound(snapshot_id.to_string()))?;

        self.host.restore_from_snapshot(&snapshot.ledger)?;
        self.committed_events.truncate(snapshot.event_count);
        self.synced_host_event_count = 0;
        Ok(())
    }

    pub fn events(&self) -> Result<Vec<HostEvent>, SimulationContextError> {
        let mut events = self.committed_events.clone();
        let host_events = self.host.event_log()?;
        events.extend(host_events.into_iter().skip(self.synced_host_event_count));
        Ok(events)
    }

    fn sync_events(&mut self) -> Result<(), SimulationContextError> {
        let host_events = self.host.event_log()?;
        if self.synced_host_event_count < host_events.len() {
            let host_event_count = host_events.len();
            self.committed_events.extend(
                host_events
                    .into_iter()
                    .skip(self.synced_host_event_count),
            );
            self.synced_host_event_count = host_event_count;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use soroban_env_host::EnvBase;
    use soroban_env_host::xdr::{
        ContractDataDurability, ContractDataEntry, ContractId, Hash, LedgerEntry, LedgerEntryData,
        LedgerEntryExt, LedgerKey, LedgerKeyContractData, Limits, ScAddress, ScVal, WriteXdr,
    };
    use std::rc::Rc;

    fn contract_data_key(id: u8, key: u32) -> Rc<LedgerKey> {
        Rc::new(LedgerKey::ContractData(LedgerKeyContractData {
            contract: ScAddress::Contract(ContractId(Hash([id; 32]))),
            key: ScVal::U32(key),
            durability: ContractDataDurability::Persistent,
        }))
    }

    fn contract_data_entry(id: u8, key: u32, value: u32, ledger_seq: u32) -> Rc<LedgerEntry> {
        Rc::new(LedgerEntry {
            last_modified_ledger_seq: ledger_seq,
            data: LedgerEntryData::ContractData(ContractDataEntry {
                ext: soroban_env_host::xdr::ExtensionPoint::V0,
                contract: ScAddress::Contract(ContractId(Hash([id; 32]))),
                key: ScVal::U32(key),
                durability: ContractDataDurability::Persistent,
                val: ScVal::U32(value),
            }),
            ext: LedgerEntryExt::V0,
        })
    }

    #[test]
    fn rollback_to_restores_exact_snapshot_and_truncates_future_events() {
        let host = SimHost::new(None, None, None);
        let mut context = SimulationContext::new(host);

        let first_key = contract_data_key(7, 1);
        let first_entry = contract_data_entry(7, 1, 11, 1);
        context
            .set_ledger_entry(first_key.as_ref().clone(), first_entry.as_ref().clone())
            .expect("initial state should load");
        context
            .host()
            .inner
            .log_from_slice("snapshot boundary", &[])
            .expect("boundary event should be recorded");
        context
            .capture_snapshot("snap-a")
            .expect("snapshot should save");

        let second_key = contract_data_key(8, 2);
        let second_entry = contract_data_entry(8, 2, 22, 2);
        context
            .set_ledger_entry(second_key.as_ref().clone(), second_entry.as_ref().clone())
            .expect("later state should load");
        context
            .host()
            .inner
            .log_from_slice("future event", &[])
            .expect("future event should be recorded");

        let before_rollback_events = context.events().expect("events should load");
        assert_eq!(before_rollback_events.len(), 2);

        context
            .rollback_to("snap-a")
            .expect("rollback should succeed");

        let restored_snapshot = context
            .host()
            .capture_snapshot()
            .expect("restored snapshot should be readable");
        assert_eq!(restored_snapshot.len(), 1);
        assert!(
            restored_snapshot
                .get(&first_key.to_xdr(Limits::none()).unwrap())
                .is_some()
        );
        assert!(
            restored_snapshot
                .get(&second_key.to_xdr(Limits::none()).unwrap())
                .is_none()
        );

        let after_rollback_events = context.events().expect("events should load");
        assert_eq!(after_rollback_events.len(), 1);

        context
            .set_ledger_entry(second_key.as_ref().clone(), second_entry.as_ref().clone())
            .expect("re-executing from rollback point should work");
        let replayed_snapshot = context
            .host()
            .capture_snapshot()
            .expect("replayed snapshot should be readable");
        assert_eq!(replayed_snapshot.len(), 2);
        assert!(
            replayed_snapshot
                .get(&second_key.to_xdr(Limits::none()).unwrap())
                .is_some()
        );
    }
}
